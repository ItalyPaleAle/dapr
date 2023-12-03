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
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dapr/dapr/pkg/actorssvc/components/actorstore"
)

func (s *SQLite) AddActorHost(ctx context.Context, properties actorstore.AddActorHostRequest) (actorstore.AddActorHostResponse, error) {
	if properties.AppID == "" || properties.Address == "" {
		return actorstore.AddActorHostResponse{}, actorstore.ErrInvalidRequestMissingParameters
	}

	// Because we need to update 2 tables, we need a transaction
	return executeInTransaction(ctx, s.logger, s.db, s.metadata.Timeout, func(ctx context.Context, tx *sql.Tx) (res actorstore.AddActorHostResponse, err error) {
		now := s.clock.Now().UnixMilli()

		// First, check if the api level of the host to add is high enough
		apiLevel, err := s.getClusterActorsAPILevel(ctx, tx)
		if err != nil {
			return res, err
		}
		if properties.APILevel < apiLevel {
			return res, actorstore.ErrActorHostAPILevelTooLow
		}

		// Generate a new host_id UUID
		hostID, err := uuid.NewRandom()
		if err != nil {
			return res, fmt.Errorf("failed to generate UUID: %w", err)
		}
		res.HostID = hostID.String()

		// The hosts table has a unique constraint on host_address
		// To start, we need to delete from the hosts table records which have the same host_address if one of:
		// - The last health check is before the configured amount (i.e. the host is down)
		// - The host_app_id is also the same, which indicates that the app has just been restarted
		// We need to actually delete the row if it exists and matches the criteria above, and can't just perform a "REPLACE" query (i.e. "ON CONFLICT DO UPDATE") because that doesn't cause records in other tables that reference this one (most notably, "actors") to be deleted automatically. We also want to make sure the host_id changes.
		queryCtx, queryCancel := context.WithTimeout(ctx, s.metadata.Timeout)
		defer queryCancel()
		_, err = tx.ExecContext(queryCtx, `
DELETE FROM hosts WHERE
	(host_address = ? AND host_app_id = ?)
	OR host_last_healthcheck < ?`,
			properties.Address, properties.AppID, now-int64(s.metadata.Config.FailedInterval().Milliseconds()),
		)
		if err != nil {
			return res, fmt.Errorf("failed to remove conflicting hosts: %w", err)
		}

		// We're ready to add the actor host
		// This is also when we detect if we have a conflict on the host_address column's unique index
		queryCtx, queryCancel = context.WithTimeout(ctx, s.metadata.Timeout)
		defer queryCancel()
		_, err = tx.ExecContext(queryCtx, `
INSERT INTO hosts
  (host_id, host_address, host_app_id, host_actors_api_level, host_last_healthcheck)
VALUES
  (?, ?, ?, ?, ?)`,
			hostID[:], properties.Address, properties.AppID, properties.APILevel, now,
		)
		if isUniqueViolationError(err) {
			return res, actorstore.ErrActorHostConflict
		} else if err != nil {
			return res, fmt.Errorf("failed to insert actor host in hosts table: %w", err)
		}

		// Register each supported actor type
		queryCtx, queryCancel = context.WithTimeout(ctx, s.metadata.Timeout)
		defer queryCancel()
		err = insertHostActorTypes(queryCtx, tx, hostID, properties.ActorTypes, s.metadata.Timeout)
		if err != nil {
			return res, err
		}

		// Update the actors API level
		newLevel, err := s.updateClusterActorsAPILevel(ctx, tx)
		if err != nil {
			return res, err
		}

		// Calling s.updatedAPILevel with the same level as before is a nop
		s.updateAPILevel(newLevel)

		return res, nil
	})
}

// Inserts the list of supported actor types for a host.
// Note that the context must have a timeout already applied if needed.
func insertHostActorTypes(ctx context.Context, tx *sql.Tx, actorHostID uuid.UUID, actorTypes []actorstore.ActorHostType, timeout time.Duration) error {
	if len(actorTypes) == 0 {
		// Nothing to do here
		return nil
	}

	// Build the query
	q := strings.Builder{}
	q.WriteString("INSERT INTO hosts_actor_types (host_id, actor_type, actor_idle_timeout) VALUES ")

	args := make([]any, 0, len(actorTypes)*3)
	for i, t := range actorTypes {
		args = append(args,
			actorHostID[:],
			t.ActorType,
			t.IdleTimeout,
		)

		if i == 0 {
			q.WriteString("(?, ?, ?)")
		} else {
			q.WriteString(", (?, ?, ?)")
		}
	}
	queryCtx, queryCancel := context.WithTimeout(ctx, timeout)
	defer queryCancel()

	res, err := tx.ExecContext(queryCtx, q.String(), args...)
	if err != nil {
		return fmt.Errorf("failed to insert supported actor types in hosts actor types table: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to count affected rows inserting supported actor types in hosts actor types table: %w", err)
	}
	if affected != int64(len(actorTypes)) {
		return fmt.Errorf("failed to insert supported actor types in hosts actor types table: inserted %d rows, but expected %d", affected, len(actorTypes))
	}

	return nil
}

func (s *SQLite) UpdateActorHost(ctx context.Context, actorHostID string, properties actorstore.UpdateActorHostRequest) (err error) {
	// We need at least _something_ to update
	// Note that:
	// ActorTypes==nil -> Do not update actor types
	// ActorTypes==slice with 0 elements -> Remove all actor types
	if actorHostID == "" || (!properties.UpdateLastHealthCheck && properties.ActorTypes == nil) {
		return actorstore.ErrInvalidRequestMissingParameters
	}
	actorHostUUID, err := uuid.Parse(actorHostID)
	if err != nil {
		return actorstore.ErrInvalidRequestMissingParameters
	}

	// Let's avoid creating a transaction if we are not updating actor types (which involve updating 2 tables)
	// This saves at least 2 round-trips to the database and improves locking
	if properties.ActorTypes == nil {
		err = updateHostsTable(ctx, s.db, s.clock.Now(), actorHostUUID, properties, s.metadata.Config.FailedInterval(), s.metadata.Timeout)
	} else {
		// Because we need to update 2 tables, we need a transaction
		_, err = executeInTransaction(ctx, s.logger, s.db, s.metadata.Timeout, func(ctx context.Context, tx *sql.Tx) (z struct{}, zErr error) {
			// Update all hosts properties, besides the list of supported actor types
			zErr = updateHostsTable(ctx, tx, s.clock.Now(), actorHostUUID, properties, s.metadata.Config.FailedInterval(), s.metadata.Timeout)
			if zErr != nil {
				return z, zErr
			}

			// Next, delete all existing host actor types
			// This query could affect 0 rows, and that's fine
			queryCtx, queryCancel := context.WithTimeout(ctx, s.metadata.Timeout)
			defer queryCancel()
			_, zErr = tx.ExecContext(queryCtx,
				"DELETE FROM hosts_actor_types WHERE host_id = ?",
				actorHostUUID[:],
			)
			if zErr != nil {
				return z, fmt.Errorf("failed to delete old host actor types: %w", zErr)
			}

			// Register the new supported actor types (if any)
			zErr = insertHostActorTypes(ctx, tx, actorHostUUID, properties.ActorTypes, s.metadata.Timeout)
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
func updateHostsTable(ctx context.Context, db querier, now time.Time, actorHostID uuid.UUID, properties actorstore.UpdateActorHostRequest, failedInterval time.Duration, timeout time.Duration) error {
	// For now, host_last_healthcheck is the only property that can be updated in the hosts table
	if !properties.UpdateLastHealthCheck {
		return nil
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, timeout)
	defer queryCancel()
	res, err := db.ExecContext(queryCtx, `
	UPDATE hosts
	SET
		host_last_healthcheck = ?
	WHERE
		host_id = ? AND
		host_last_healthcheck >= ?`,
		now.UnixMilli(), actorHostID[:], now.Add(-1*failedInterval).UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("failed to update actor host: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to count affected rows: %w", err)
	}
	if affected == 0 {
		return actorstore.ErrActorHostNotFound
	}
	return nil
}

func (s *SQLite) RemoveActorHost(ctx context.Context, actorHostID string) error {
	if actorHostID == "" {
		return actorstore.ErrInvalidRequestMissingParameters
	}
	actorHostUUID, err := uuid.Parse(actorHostID)
	if err != nil {
		return actorstore.ErrInvalidRequestMissingParameters
	}

	// We need to delete from the hosts table only
	// Other table references rows from the hosts table through foreign keys, so records are deleted from there automatically (and atomically)
	queryCtx, queryCancel := context.WithTimeout(ctx, s.metadata.Timeout)
	defer queryCancel()
	res, err := s.db.ExecContext(queryCtx, "DELETE FROM hosts WHERE host_id = ?", actorHostUUID[:])
	if err != nil {
		return fmt.Errorf("failed to remove actor host: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to count affected rows: %w", err)
	}
	if affected == 0 {
		return actorstore.ErrActorHostNotFound
	}

	return nil
}

func (s *SQLite) LookupActor(ctx context.Context, ref actorstore.ActorRef, opts actorstore.LookupActorOpts) (res actorstore.LookupActorResponse, err error) {
	if ref.ActorType == "" || ref.ActorID == "" {
		return res, actorstore.ErrInvalidRequestMissingParameters
	}

	// Parse all opts.Hosts as []byte
	hostUUIDs := make([][]byte, len(opts.Hosts))
	for i := 0; i < len(opts.Hosts); i++ {
		var u uuid.UUID
		u, err = uuid.Parse(opts.Hosts[i])
		if err != nil {
			return res, actorstore.ErrInvalidRequestMissingParameters
		}
		hostUUIDs[i] = u[:]
	}

	// We need a transaction here to ensure consistency
	return executeInTransaction(ctx, s.logger, s.db, s.metadata.Timeout, func(ctx context.Context, tx *sql.Tx) (res actorstore.LookupActorResponse, err error) {
		now := s.clock.Now().UnixMilli()
		minLastHealthCheck := now - int64(s.metadata.Config.FailedInterval().Milliseconds())

		var hostID []byte

		// First, check if the actor is already active
		queryCtx, queryCancel := context.WithTimeout(ctx, s.metadata.Timeout)
		defer queryCancel()
		err = tx.QueryRowContext(queryCtx, `
SELECT
	hosts.host_id, hosts.host_app_id, hosts.host_address, actors.actor_idle_timeout
FROM actors, hosts
WHERE
  actors.actor_type = ?
  AND actors.actor_id = ?
  AND actors.host_id = hosts.host_id
  AND hosts.host_last_healthcheck >= ?`,
			ref.ActorType, ref.ActorID, minLastHealthCheck,
		).Scan(
			&hostID, &res.AppID, &res.Address, &res.IdleTimeout,
		)

		switch {
		case errors.Is(err, sql.ErrNoRows):
			// If we get ErrNoRows, it means that the actor isn't active, so we don't do anything here
			// Logic continues after the switch statement
		case err != nil:
			// If we have an error and it's not an ErrNoRows, return
			return res, fmt.Errorf("query error: failed to lookup existing actor: %w", err)
		default:
			// No error, the actor already exists…
			// …but if we have host restrictions, we must make sure it's active on one of those hosts

			// Parse the UUID
			var hostUUID uuid.UUID
			hostUUID, err = uuid.FromBytes(hostID)
			if err != nil {
				return res, fmt.Errorf("invalid host ID, not a UUID: %w", err)
			}
			res.HostID = hostUUID.String()

			// No host restriction
			if len(hostUUIDs) == 0 {
				return res, nil
			}

			// We have host restrictions
			for i := range hostUUIDs {
				if bytes.Equal(hostUUIDs[i], hostID) {
					// Found a match! We can return
					return res, nil
				}
			}

			// If we are here, it means the actor is active on a host that is not in the list of allowed ones, so return ErrNoActorHost
			return res, actorstore.ErrNoActorHost
		}

		// If we are here, we need to activate the actor
		// First, select a random host that is capable of hosting actors of this type
		queryParams := make([]any, len(hostUUIDs)+2)
		queryParams[0] = ref.ActorType
		queryParams[1] = minLastHealthCheck
		var hostRestrictionFragment string
		if len(hostUUIDs) > 0 {
			b := strings.Builder{}
			b.WriteString("AND (")
			for i := range hostUUIDs {
				if i != 0 {
					b.WriteString(" OR ")
				}
				b.WriteString("hosts.host_id = ?")
				queryParams[i+2] = hostUUIDs[i]
			}
			b.WriteRune(')')
			hostRestrictionFragment = b.String()
		}
		queryCtx, queryCancel = context.WithTimeout(ctx, s.metadata.Timeout)
		defer queryCancel()
		err = tx.QueryRowContext(queryCtx, `
SELECT
  hosts.host_id, hosts.host_app_id, hosts.host_address, hosts_actor_types.actor_idle_timeout
FROM hosts_actor_types, hosts
WHERE
  hosts_actor_types.actor_type = ?
  AND hosts.host_id = hosts_actor_types.host_id
  AND hosts.host_last_healthcheck >= ?
  `+hostRestrictionFragment+`
ORDER BY random() LIMIT 1
`,
			queryParams...,
		).Scan(
			&hostID, &res.AppID, &res.Address, &res.IdleTimeout,
		)

		// If the query returned ErrNoRows, it means that there's no host capable of hosting actors of this kind (or at least none in the list of allowed hosts)
		if errors.Is(err, sql.ErrNoRows) {
			return res, actorstore.ErrNoActorHost
		} else if err != nil {
			return res, fmt.Errorf("query error: failed to retrieve actor host: %w", err)
		}

		// Parse the UUID
		var hostUUID uuid.UUID
		hostUUID, err = uuid.FromBytes(hostID)
		if err != nil {
			return res, fmt.Errorf("invalid host ID, not a UUID: %w", err)
		}
		res.HostID = hostUUID.String()

		// Last step: activate the actor on the host we selected
		// We use a REPLACE here because there could already be a row, which would indicate the actor is active on a un-healthy host, so it's ok to replace the row
		queryCtx, queryCancel = context.WithTimeout(ctx, s.metadata.Timeout)
		defer queryCancel()
		_, err = tx.ExecContext(queryCtx, `
REPLACE INTO actors (actor_type, actor_id, host_id, actor_idle_timeout, actor_activation)
VALUES (?, ?, ?, ?, ?)
`,
			ref.ActorType, ref.ActorID, hostID, res.IdleTimeout, now,
		)
		if err != nil {
			return res, fmt.Errorf("query error: failed to activate actor: %w", err)
		}

		// All set!
		return res, nil
	})
}

func (s *SQLite) RemoveActor(ctx context.Context, ref actorstore.ActorRef) error {
	if ref.ActorType == "" || ref.ActorID == "" {
		return actorstore.ErrInvalidRequestMissingParameters
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, s.metadata.Timeout)
	defer queryCancel()
	res, err := s.db.ExecContext(queryCtx, "DELETE FROM actors WHERE actor_type = ? AND actor_id = ?", ref.ActorType, ref.ActorID)
	if err != nil {
		return fmt.Errorf("failed to remove actor: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to count affected rows: %w", err)
	}
	if affected == 0 {
		return actorstore.ErrActorNotFound
	}

	return nil
}
