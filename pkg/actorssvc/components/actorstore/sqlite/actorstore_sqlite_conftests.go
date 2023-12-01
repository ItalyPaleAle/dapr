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

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/exp/slices"
	clocktesting "k8s.io/utils/clock/testing"

	"github.com/dapr/dapr/pkg/actorssvc/components/actorstore"
)

/*
This file contains additional methods that are only used for testing.
It is compiled only when the "conftests" tag is enabled
*/

// GetTime returns the current time.
func (s *SQLite) GetTime() time.Time {
	return s.clock.Now()
}

// AdvanceTime makes the time advance by the specified duration.
func (s *SQLite) AdvanceTime(d time.Duration) error {
	s.clock.Sleep(d)
	return nil
}

// SetupConformanceTests performs the setup of test resources.
func (s *SQLite) SetupConformanceTests() error {
	// Switch the clock to a mocked one
	// (This date just feels right :) https://en.wikipedia.org/wiki/2006_FIFA_World_Cup_final )
	s.clock = clocktesting.NewFakeClock(time.Date(2006, 7, 9, 20, 0, 0, 0, time.UTC))
	return nil
}

// CleanupConformanceTests performs the cleanup of test resources.
func (s *SQLite) CleanupConformanceTests() error {
	// Nothing to do here since the SQLite database is disposed
	return nil
}

// GetAllHosts returns the entire list of hosts in the database.
func (s *SQLite) GetAllHosts() (actorstore.TestDataHosts, error) {
	// Use a transaction for consistency
	return executeInTransaction(context.Background(), s.logger, s.db, time.Minute, func(ctx context.Context, tx *sql.Tx) (map[string]actorstore.TestDataHost, error) {
		res := actorstore.TestDataHosts{}

		// First, load all hosts
		rows, err := tx.QueryContext(ctx, "SELECT host_id, host_address, host_app_id, host_actors_api_level, host_last_healthcheck FROM hosts")
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
		rows, err = tx.QueryContext(ctx, "SELECT host_id, actor_type, actor_idle_timeout FROM hosts_actor_types")
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
		rows, err = tx.QueryContext(ctx, "SELECT actor_type, actor_id, host_id FROM actors")
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
func (s *SQLite) GetAllReminders() (actorstore.TestDataReminders, error) {
	res := actorstore.TestDataReminders{}

	// First, load all hosts
	rows, err := s.db.QueryContext(context.Background(), "SELECT reminder_id, actor_type, actor_id, reminder_name, reminder_execution_time - now(), reminder_lease_id, reminder_lease_time, reminder_lease_pid FROM reminders")
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
func (s *SQLite) LoadActorStateTestData(testData actorstore.TestData) error {
	ctx := context.Background()

	hosts := [][]any{}
	hostsActorTypes := [][]any{}
	actors := [][]any{}

	for hostID, host := range testData.Hosts {
		hosts = append(hosts, []any{hostID, host.Address, host.AppID, host.APILevel, s.clock.Now().Add(host.LastHealthCheckStore).Unix()})
		for actorType, at := range host.ActorTypes {
			hostsActorTypes = append(hostsActorTypes, []any{hostID, actorType, int(at.IdleTimeout.Seconds()), at.ConcurrentRemindersLimit})

			for _, actorID := range at.ActorIDs {
				actors = append(actors, []any{actorType, actorID, hostID, int(at.IdleTimeout.Seconds())})
			}
		}
	}

	// Sort hosts putting those with the lowest API level first
	slices.SortFunc(hosts, func(a, b []any) int {
		return a[3].(int) - b[3].(int)
	})

	// Start transaction
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Clean the tables first
	// Note that the hosts actor types and actors table use foreign keys, so deleting hosts is enough to clean those too
	_, err = tx.ExecContext(ctx, "DELETE FROM hosts")
	if err != nil {
		return fmt.Errorf("failed to clean the hosts table: %w", err)
	}

	// Copy data for each table
	for _, r := range hosts {
		_, err = tx.ExecContext(ctx, `INSERT INTO hosts (host_id, host_address, host_app_id, host_actors_api_level, host_last_healthcheck) VALUES (?, ?, ?, ?, ?)`, r...)
		if err != nil {
			return fmt.Errorf("failed to insert into hosts table: %w", err)
		}
	}
	for _, r := range hostsActorTypes {
		_, err = tx.ExecContext(ctx, `INSERT INTO hosts (host_id, actor_type, actor_idle_timeout, actor_concurrent_reminders) VALUES (?, ?, ?, ?)`, r...)
		if err != nil {
			return fmt.Errorf("failed to insert into hosts actor types table: %w", err)
		}
	}
	for _, r := range actors {
		_, err = tx.ExecContext(ctx, `INSERT INTO hosts (actor_type, actor_id, host_id, actor_idle_timeout) VALUES (?, ?, ?, ?)`, r...)
		if err != nil {
			return fmt.Errorf("failed to insert into actors table: %w", err)
		}
	}

	// Commit
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// LoadReminderTestData loads all reminder test data in the database.
func (s *SQLite) LoadReminderTestData(testData actorstore.TestData) error {
	ctx := context.Background()
	now := s.clock.Now()

	// Start transaction
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Clean the table first
	_, err = tx.ExecContext(ctx, "DELETE FROM reminders")
	if err != nil {
		return fmt.Errorf("failed to clean the reminders table: %w", err)
	}

	reminders := [][]any{}
	for reminderID, reminder := range testData.Reminders {
		reminders = append(reminders, []any{
			reminderID, reminder.ActorType, reminder.ActorID, reminder.Name,
			now.Add(reminder.ExecutionTime).Unix(),
			reminder.LeaseID, reminder.LeaseTime, reminder.LeasePID,
		})
		_, err = tx.ExecContext(ctx,
			`INSERT INTO reminders
				(reminder_id, actor_type, actor_id, reminder_name, reminder_execution_time, reminder_lease_id, reminder_lease_time, reminder_lease_pid)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			reminderID, reminder.ActorType, reminder.ActorID, reminder.Name,
			now.Add(reminder.ExecutionTime).Unix(),
			reminder.LeaseID, reminder.LeaseTime, reminder.LeasePID,
		)
		if err != nil {
			return fmt.Errorf("failed to insert into actors table: %w", err)
		}
	}

	// Commit
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
