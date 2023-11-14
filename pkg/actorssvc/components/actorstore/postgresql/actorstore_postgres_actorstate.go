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

	"github.com/jackc/pgx/v5"

	pginterfaces "github.com/dapr/components-contrib/common/component/postgresql/interfaces"
	"github.com/dapr/dapr/pkg/actorssvc/components/actorstore"
)

func (p *PostgreSQL) AddActorHost(ctx context.Context, properties actorstore.AddActorHostRequest) (string, error) {
	if properties.AppID == "" || properties.Address == "" || properties.APILevel <= 0 {
		return "", actorstore.ErrInvalidRequestMissingParameters
	}

	// Because we need to update 2 tables, we need a transaction
	return executeInTransaction(ctx, p.logger, p.db, p.metadata.Timeout, func(ctx context.Context, tx pgx.Tx) (hostID string, err error) {
		var (
			hostsTable           = p.metadata.TableName(pgTableHosts)
			hostsActorTypesTable = p.metadata.TableName(pgTableHostsActorTypes)
		)

		// The hosts table has a unique constraint on host_address
		// To start, we need to delete from the hosts table records which have the same host_address if one of:
		// - The last health check is before the configured amount (i.e. the host is down)
		// - The host_app_id is also the same, which indicates that the app has just been restarted
		// We need to actually delete the row if it exists and matches the criteria above, and can't just perform a "REPLACE" query (i.e. "ON CONFLICT DO UPDATE") because that doesn't cause records in other tables that reference this one (most notably, "actors") to be deleted automatically. We also want to make sure the host_id changes.
		queryCtx, queryCancel := context.WithTimeout(ctx, p.metadata.Timeout)
		defer queryCancel()
		_, err = tx.Exec(queryCtx,
			fmt.Sprintf(
				`DELETE FROM %s WHERE
					(host_address = $1 AND host_app_id = $2)
        			OR host_last_healthcheck < CURRENT_TIMESTAMP - $3::interval`,
				hostsTable,
			),
			properties.Address, properties.AppID, p.metadata.Config.FailedInterval(),
		)
		if err != nil {
			return "", fmt.Errorf("failed to remove conflicting hosts: %w", err)
		}

		// We're ready to add the actor host
		// This is also when we detect if we have a conflict on the host_address column's unique index
		queryCtx, queryCancel = context.WithTimeout(ctx, p.metadata.Timeout)
		defer queryCancel()
		err = tx.QueryRow(queryCtx,
			fmt.Sprintf(
				`INSERT INTO %s
					(host_address, host_app_id, host_actors_api_level, host_last_healthcheck)
				VALUES
					($1, $2, $3, CURRENT_TIMESTAMP)
				RETURNING host_id`,
				hostsTable,
			),
			properties.Address, properties.AppID, properties.APILevel,
		).Scan(&hostID)
		if err != nil {
			if isUniqueViolationError(err) {
				return "", actorstore.ErrActorHostConflict
			}
			return "", fmt.Errorf("failed to insert actor host in hosts table: %w", err)
		}

		// Register each supported actor type
		queryCtx, queryCancel = context.WithTimeout(ctx, p.metadata.Timeout)
		defer queryCancel()
		err = insertHostActorTypes(queryCtx, tx, hostID, properties.ActorTypes, hostsActorTypesTable, p.metadata.Timeout)
		if err != nil {
			return "", err
		}

		return hostID, nil
	})
}

// Inserts the list of supported actor types for a host.
// Note that the context must have a timeout already applied if needed.
func insertHostActorTypes(ctx context.Context, tx pgx.Tx, actorHostID string, actorTypes []actorstore.ActorHostType, hostsActorTypesTable string, timeout time.Duration) error {
	if len(actorTypes) == 0 {
		// Nothing to do here
		return nil
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, timeout)
	defer queryCancel()

	// Use "CopyFrom" to insert multiple records more efficiently
	rows := make([][]any, len(actorTypes))
	for i, t := range actorTypes {
		rows[i] = []any{
			actorHostID,
			t.ActorType,
			t.IdleTimeout,
		}
	}
	n, err := tx.CopyFrom(
		queryCtx,
		pgx.Identifier{hostsActorTypesTable},
		[]string{"host_id", "actor_type", "actor_idle_timeout"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("failed to insert supported actor types in hosts actor types table: %w", err)
	}
	if n != int64(len(actorTypes)) {
		return fmt.Errorf("failed to insert supported actor types in hosts actor types table: inserted %d rows, but expected %d", n, len(actorTypes))
	}

	return nil
}

func (p *PostgreSQL) UpdateActorHost(ctx context.Context, actorHostID string, properties actorstore.UpdateActorHostRequest) (err error) {
	// We need at least _something_ to update
	// Note that:
	// ActorTypes==nil -> Do not update actor types
	// ActorTypes==slice with 0 elements -> Remove all actor types
	if actorHostID == "" || (!properties.UpdateLastHealthCheck && properties.ActorTypes == nil) {
		return actorstore.ErrInvalidRequestMissingParameters
	}

	var (
		hostsTable           = p.metadata.TableName(pgTableHosts)
		hostsActorTypesTable = p.metadata.TableName(pgTableHostsActorTypes)
	)

	// Let's avoid creating a transaction if we are not updating actor types (which involve updating 2 tables)
	// This saves at least 2 round-trips to the database and improves locking
	if properties.ActorTypes == nil {
		err = updateHostsTable(ctx, p.db, actorHostID, properties, hostsTable, p.metadata.Config.FailedInterval(), p.metadata.Timeout)
	} else {
		// Because we need to update 2 tables, we need a transaction
		_, err = executeInTransaction(ctx, p.logger, p.db, p.metadata.Timeout, func(ctx context.Context, tx pgx.Tx) (z struct{}, zErr error) {
			// Update all hosts properties, besides the list of supported actor types
			zErr = updateHostsTable(ctx, tx, actorHostID, properties, hostsTable, p.metadata.Config.FailedInterval(), p.metadata.Timeout)
			if zErr != nil {
				return z, zErr
			}

			// Next, delete all existing actor
			// This query could affect 0 rows, and that's fine
			queryCtx, queryCancel := context.WithTimeout(ctx, p.metadata.Timeout)
			defer queryCancel()
			_, zErr = p.db.Exec(queryCtx,
				fmt.Sprintf("DELETE FROM %s WHERE host_id = $1", hostsActorTypesTable),
				actorHostID,
			)
			if zErr != nil {
				return z, fmt.Errorf("failed to delete old host actor types: %w", zErr)
			}

			// Register the new supported actor types (if any)
			zErr = insertHostActorTypes(ctx, tx, actorHostID, properties.ActorTypes, hostsActorTypesTable, p.metadata.Timeout)
			if zErr != nil {
				return z, zErr
			}

			return z, nil
		})
	}

	if err != nil {
		return err
	}
	return nil
}

// Updates the hosts table with the given properties.
// Does not update ActorTypes which impacts a separate table.
func updateHostsTable(ctx context.Context, db pginterfaces.DBQuerier, actorHostID string, properties actorstore.UpdateActorHostRequest, hostsTable string, failedInterval time.Duration, timeout time.Duration) error {
	// For now, host_last_healthcheck is the only property that can be updated in the hosts table
	if !properties.UpdateLastHealthCheck {
		return nil
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, timeout)
	defer queryCancel()
	res, err := db.Exec(queryCtx,
		fmt.Sprintf(`UPDATE %s
			SET
				host_last_healthcheck = CURRENT_TIMESTAMP
			WHERE
				host_id = $1 AND
				host_last_healthcheck >= CURRENT_TIMESTAMP - $2::interval
			`, hostsTable),
		actorHostID, failedInterval,
	)
	if err != nil {
		return fmt.Errorf("failed to update actor host: %w", err)
	}
	if res.RowsAffected() == 0 {
		return actorstore.ErrActorHostNotFound
	}
	return nil
}

func (p *PostgreSQL) RemoveActorHost(ctx context.Context, actorHostID string) error {
	if actorHostID == "" {
		return actorstore.ErrInvalidRequestMissingParameters
	}

	// We need to delete from the hosts table only
	// Other table references rows from the hosts table through foreign keys, so records are deleted from there automatically (and atomically)
	queryCtx, queryCancel := context.WithTimeout(ctx, p.metadata.Timeout)
	defer queryCancel()
	q := fmt.Sprintf("DELETE FROM %s WHERE host_id = $1", p.metadata.TableName(pgTableHosts))
	res, err := p.db.Exec(queryCtx, q, actorHostID)
	if err != nil {
		return fmt.Errorf("failed to remove actor host: %w", err)
	}
	if res.RowsAffected() == 0 {
		return actorstore.ErrActorHostNotFound
	}

	return nil
}

func (p *PostgreSQL) LookupActor(ctx context.Context, ref actorstore.ActorRef, opts actorstore.LookupActorOpts) (res actorstore.LookupActorResponse, err error) {
	if ref.ActorType == "" || ref.ActorID == "" {
		return res, actorstore.ErrInvalidRequestMissingParameters
	}

	var (
		hostsTable           = p.metadata.TableName(pgTableHosts)
		hostsActorTypesTable = p.metadata.TableName(pgTableHostsActorTypes)
		actorsTable          = p.metadata.TableName(pgTableActors)
	)

	// If we have host restrictions, limit to those
	var query string
	var args []any
	if len(opts.Hosts) == 0 {
		query = fmt.Sprintf(lookupActorQuery, hostsTable, hostsActorTypesTable, actorsTable)
		args = []any{ref.ActorType, ref.ActorID, p.metadata.Config.FailedInterval()}
	} else {
		query = fmt.Sprintf(lookupActorQueryWithHostRestriction, hostsTable, hostsActorTypesTable, actorsTable)
		args = []any{ref.ActorType, ref.ActorID, p.metadata.Config.FailedInterval(), opts.Hosts}
	}

	// This query could fail with no rows there's a race condition where the same actor is being invoked multiple times and it doesn't exist already (on an active host)
	// So, let's implement a retry in case of actors not found, up to 3 times
	for i := 0; i < 3; i++ {
		queryCtx, queryCancel := context.WithTimeout(ctx, p.metadata.Timeout)
		defer queryCancel()

		err = p.db.
			QueryRow(queryCtx, query, args...).
			Scan(&res.HostID, &res.AppID, &res.Address, &res.IdleTimeout)

		if err == nil {
			break
		}

		// If we got no rows, it's possible that we had a race condition where we were trying to create the same actor that didn't exist twice
		// In this case, we will retry after a very short delay if we didn't find an actor
		// This could make the entire method slower when an actor isn't found, but that's an error case and it shouldn't bother us much anyways
		if errors.Is(err, pgx.ErrNoRows) {
			select {
			case <-time.After(10 * time.Millisecond):
				// nop
			case <-ctx.Done():
				return res, ctx.Err()
			}
			continue
		} else {
			// Return in case of other errors
			return res, fmt.Errorf("failed to lookup actor: %w", err)
		}
	}

	// If we're here and we still have an error, it means that we don't have a host that supports actors of the given type
	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		return res, actorstore.ErrNoActorHost
	}

	return res, nil
}

func (p *PostgreSQL) RemoveActor(ctx context.Context, ref actorstore.ActorRef) error {
	if ref.ActorType == "" || ref.ActorID == "" {
		return actorstore.ErrInvalidRequestMissingParameters
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, p.metadata.Timeout)
	defer queryCancel()
	q := fmt.Sprintf("DELETE FROM %s WHERE actor_type = $1 AND actor_id = $2", p.metadata.TableName(pgTableActors))
	res, err := p.db.Exec(queryCtx, q, ref.ActorType, ref.ActorID)
	if err != nil {
		return fmt.Errorf("failed to remove actor: %w", err)
	}
	if res.RowsAffected() == 0 {
		return actorstore.ErrActorNotFound
	}

	return nil
}
