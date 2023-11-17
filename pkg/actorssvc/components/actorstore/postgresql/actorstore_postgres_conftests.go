//go:build conftests

/*
Copyright 2023 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package postgresql

import (
	"context"
	"errors"
	"fmt"
	"time"

	// Blank embed for the import package.
	_ "embed"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	clocktesting "k8s.io/utils/clock/testing"

	"github.com/dapr/dapr/pkg/actorssvc/components/actorstore"
)

/*
This file contains additional methods that are only used for testing.
It is compiled only when the "conftests" tag is enabled
*/

//go:embed queries/setup-conformance-tests.sql
var confTestsSetupQueries string

func init() {
	// Function to execute after getting the pgxpool.Config object
	modifyConfigFn = func(p *PostgreSQL, config *pgxpool.Config) {
		// After each connection, we need to set the search order to allow overriding the now() method
		config.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
			p.logger.Debugf("Override search_path in new connection")
			_, err := c.Exec(context.Background(), `SET SESSION search_path = override, pg_catalog, public;`)
			if err != nil {
				return fmt.Errorf("failed to set search_path: %w", err)
			}
			return nil
		}
	}
}

// GetTime returns the current time.
func (p *PostgreSQL) GetTime() time.Time {
	return p.clock.Now()
}

// AdvanceTime makes the time advance by the specified duration.
func (p *PostgreSQL) AdvanceTime(d time.Duration) error {
	p.clock.Sleep(d)

	_, err := p.db.Exec(context.Background(), `SELECT override.freeze_time($1, false)`, p.clock.Now())
	if err != nil {
		return fmt.Errorf("failed to freeze time: %w", err)
	}
	return nil
}

// SetupConformanceTests performs the setup of test resources.
func (p *PostgreSQL) SetupConformanceTests() error {
	// Switch the clock to a mocked one
	// (This date just feels right :) https://en.wikipedia.org/wiki/2006_FIFA_World_Cup_final )
	p.clock = clocktesting.NewFakeClock(time.Date(2006, 7, 9, 20, 0, 0, 0, time.UTC))

	// Execute queries for conformance tests
	_, err := p.db.Exec(context.Background(), confTestsSetupQueries)
	if err != nil {
		return fmt.Errorf("failed to perform setup queries: %w", err)
	}

	// Freeze the current time in the database
	_, err = p.db.Exec(context.Background(), `SELECT override.freeze_time($1, false)`, p.clock.Now())
	if err != nil {
		return fmt.Errorf("failed to freeze time: %w", err)
	}

	return nil
}

// CleanupConformanceTests performs the cleanup of test resources.
func (p *PostgreSQL) CleanupConformanceTests() error {
	errs := []error{}

	// Tables
	for _, table := range []pgTable{pgTableReminders, pgTableActors, pgTableHostsActorTypes, pgTableHosts, "metadata"} {
		p.logger.Infof("Removing table %s", p.metadata.TableName(table))
		_, err := p.db.Exec(context.Background(), fmt.Sprintf("DROP TABLE IF EXISTS %s", p.metadata.TableName(table)))
		if err != nil {
			p.logger.Errorf("Failed to remove table %s: %v", table, err)
			errs = append(errs, err)
		}
	}

	// Functions
	for _, fn := range []string{
		p.metadata.FunctionName(pgFunctionFetchReminders) + "(interval,interval,uuid[],text[],interval,integer)",
		p.metadata.TablePrefix + "enforce_min_api_level()",
		p.metadata.TablePrefix + "update_metadata_min_api_level()",
	} {
		p.logger.Infof("Removing function %s", fn)
		_, err := p.db.Exec(context.Background(), fmt.Sprintf("DROP FUNCTION IF EXISTS %s;", fn))
		if err != nil {
			p.logger.Errorf("Failed to remove function %s: %v", fn, err)
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// GetAllHosts returns the entire list of hosts in the database.
func (p *PostgreSQL) GetAllHosts() (map[string]actorstore.TestDataHost, error) {
	// Use a transaction for consistency
	return executeInTransaction(context.Background(), p.logger, p.db, time.Minute, func(ctx context.Context, tx pgx.Tx) (map[string]actorstore.TestDataHost, error) {
		res := map[string]actorstore.TestDataHost{}

		// First, load all hosts
		rows, err := tx.Query(ctx, "SELECT host_id, host_address, host_app_id, host_actors_api_level, host_last_healthcheck FROM "+p.metadata.TableName(pgTableHosts))
		if err != nil {
			return nil, fmt.Errorf("failed to load data from the hosts table: %w", err)
		}

		for rows.Next() {
			var hostID string
			r := actorstore.TestDataHost{
				ActorTypes: map[string]actorstore.TestDataActorType{},
			}
			err = rows.Scan(&hostID, &r.Address, &r.AppID, &r.APILevel, &r.LastHealthCheck)
			if err != nil {
				return nil, fmt.Errorf("failed to load data from the hosts table: %w", err)
			}
			res[hostID] = r
		}

		// Load all actor types
		rows, err = tx.Query(ctx, "SELECT host_id, actor_type, actor_idle_timeout FROM "+p.metadata.TableName(pgTableHostsActorTypes))
		if err != nil {
			return nil, fmt.Errorf("failed to load data from the hosts actor types table: %w", err)
		}

		for rows.Next() {
			var (
				hostID      string
				actorType   string
				idleTimeout int
			)
			err = rows.Scan(&hostID, &actorType, &idleTimeout)
			if err != nil {
				return nil, fmt.Errorf("failed to load data from the hosts actor types table: %w", err)
			}

			host, ok := res[hostID]
			if !ok {
				// Should never happen, given that host_id has a foreign key reference to the hosts table…
				return nil, fmt.Errorf("hosts actor types table contains data for non-existing host ID: %s", hostID)
			}
			host.ActorTypes[actorType] = actorstore.TestDataActorType{
				IdleTimeout: time.Duration(idleTimeout) * time.Second,
				ActorIDs:    make([]string, 0),
			}
		}

		// Lastly, load all actor IDs
		rows, err = tx.Query(ctx, "SELECT actor_type, actor_id, host_id FROM "+p.metadata.TableName(pgTableActors))
		if err != nil {
			return nil, fmt.Errorf("failed to load data from the actors table: %w", err)
		}

		for rows.Next() {
			var (
				actorType string
				actorID   string
				hostID    string
			)
			err = rows.Scan(&actorType, &actorID, &hostID)
			if err != nil {
				return nil, fmt.Errorf("failed to load data from the actors table: %w", err)
			}

			host, ok := res[hostID]
			if !ok {
				// Should never happen, given that host_id has a foreign key reference to the hosts table…
				return nil, fmt.Errorf("actors table contains data for non-existing host ID: %s", hostID)
			}
			at, ok := host.ActorTypes[actorType]
			if !ok {
				// Should never happen, given that host_id has a foreign key reference to the hosts table…
				return nil, fmt.Errorf("actors table contains data for non-existing actor type: %s", actorType)
			}
			at.ActorIDs = append(at.ActorIDs, actorID)
			host.ActorTypes[actorType] = at
		}

		return res, nil
	})
}

// GetAllReminders returns the entire list of reminders in the database.
func (p *PostgreSQL) GetAllReminders() (map[string]actorstore.TestDataReminder, error) {
	res := map[string]actorstore.TestDataReminder{}

	// First, load all hosts
	rows, err := p.db.Query(context.Background(), "SELECT reminder_id, actor_type, actor_id, reminder_name, reminder_execution_time, reminder_lease_id, reminder_lease_time, reminder_lease_pid FROM "+p.metadata.TableName(pgTableReminders))
	if err != nil {
		return nil, fmt.Errorf("failed to load data from the reminders table: %w", err)
	}

	for rows.Next() {
		var reminderID string
		r := actorstore.TestDataReminder{}
		err = rows.Scan(&reminderID, &r.ActorType, &r.ActorID, &r.Name, &r.ExecutionTime, &r.LeaseID, &r.LeaseTime, &r.LeasePID)
		if err != nil {
			return nil, fmt.Errorf("failed to load data from the reminders table: %w", err)
		}
		res[reminderID] = r
	}

	return res, nil
}

// LoadActorStateTestData loads all actor state test data in the database.
func (p *PostgreSQL) LoadActorStateTestData(testData actorstore.TestData) error {
	hosts := [][]any{}
	hostsActorTypes := [][]any{}
	actors := [][]any{}

	for hostID, host := range testData.Hosts {
		hosts = append(hosts, []any{hostID, host.Address, host.AppID, host.APILevel, p.clock.Now().Add(host.LastHealthCheckStore)})

		for actorType, at := range host.ActorTypes {
			hostsActorTypes = append(hostsActorTypes, []any{hostID, actorType, int(at.IdleTimeout.Seconds()), at.ConcurrentRemindersLimit})

			for _, actorID := range at.ActorIDs {
				actors = append(actors, []any{actorType, actorID, hostID, int(at.IdleTimeout.Seconds())})
			}
		}
	}

	// Clean the tables first
	// Note that the hosts actor types and actors table use foreign keys, so deleting hosts is enough to clean those too
	_, err := p.db.Exec(
		context.Background(),
		"DELETE FROM "+p.metadata.TableName(pgTableHosts),
	)
	if err != nil {
		return fmt.Errorf("failed to clean the hosts table: %w", err)
	}

	// Copy data for each table
	_, err = p.db.CopyFrom(
		context.Background(),
		pgx.Identifier{p.metadata.TableName(pgTableHosts)},
		[]string{"host_id", "host_address", "host_app_id", "host_actors_api_level", "host_last_healthcheck"},
		pgx.CopyFromRows(hosts),
	)
	if err != nil {
		return fmt.Errorf("failed to load test data for hosts table: %w", err)
	}

	_, err = p.db.CopyFrom(
		context.Background(),
		pgx.Identifier{p.metadata.TableName(pgTableHostsActorTypes)},
		[]string{"host_id", "actor_type", "actor_idle_timeout", "actor_concurrent_reminders"},
		pgx.CopyFromRows(hostsActorTypes),
	)
	if err != nil {
		return fmt.Errorf("failed to load test data for hosts actor types table: %w", err)
	}

	_, err = p.db.CopyFrom(
		context.Background(),
		pgx.Identifier{p.metadata.TableName(pgTableActors)},
		[]string{"actor_type", "actor_id", "host_id", "actor_idle_timeout"},
		pgx.CopyFromRows(actors),
	)
	if err != nil {
		return fmt.Errorf("failed to load test data for actors table: %w", err)
	}

	return nil
}

// LoadReminderTestData loads all reminder test data in the database.
func (p *PostgreSQL) LoadReminderTestData(testData actorstore.TestData) error {
	now := p.clock.Now()

	reminders := [][]any{}
	for reminderID, reminder := range testData.Reminders {
		reminders = append(reminders, []any{
			reminderID, reminder.ActorType, reminder.ActorID, reminder.Name,
			now.Add(reminder.ExecutionTime), reminder.LeaseID, reminder.LeaseTime, reminder.LeasePID,
		})
	}

	// Clean the table first
	_, err := p.db.Exec(
		context.Background(),
		"DELETE FROM "+p.metadata.TableName(pgTableReminders),
	)
	if err != nil {
		return fmt.Errorf("failed to clean the reminders table: %w", err)
	}

	// Copy data
	_, err = p.db.CopyFrom(
		context.Background(),
		pgx.Identifier{p.metadata.TableName(pgTableReminders)},
		[]string{"reminder_id", "actor_type", "actor_id", "reminder_name", "reminder_execution_time", "reminder_lease_id", "reminder_lease_time", "reminder_lease_pid"},
		pgx.CopyFromRows(reminders),
	)
	if err != nil {
		return fmt.Errorf("failed to load test data for reminders table: %w", err)
	}

	return nil
}
