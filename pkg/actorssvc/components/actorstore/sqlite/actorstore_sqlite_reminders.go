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
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"math"
	mrand "math/rand"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dapr/dapr/pkg/actorssvc/components/actorstore"
	"github.com/dapr/kit/ptr"
)

func (s *SQLite) GetReminder(ctx context.Context, req actorstore.ReminderRef) (res actorstore.GetReminderResponse, err error) {
	if !req.IsValid() {
		return res, actorstore.ErrInvalidRequestMissingParameters
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, s.metadata.Timeout)
	defer queryCancel()

	var (
		executionTime int64
		ttl           *int64
	)
	err = s.db.
		QueryRowContext(queryCtx, `
SELECT
	reminder_execution_time,
	reminder_period,
	reminder_ttl,
	reminder_data
FROM reminders
WHERE
	actor_type = ?
	AND actor_id = ?
	AND reminder_name = ?`,
			req.ActorType, req.ActorID, req.Name,
		).
		Scan(&executionTime, &res.Period, &ttl, &res.Data)
	if errors.Is(err, sql.ErrNoRows) {
		return res, actorstore.ErrReminderNotFound
	} else if err != nil {
		return res, fmt.Errorf("failed to retrieve reminder: %w", err)
	}

	// The query returns the execution time and TTL as UNIX timestamps with milliseconds
	res.ExecutionTime = time.UnixMilli(executionTime)
	if ttl != nil && *ttl > 0 {
		res.TTL = ptr.Of(time.UnixMilli(*ttl))
	}

	return res, nil
}

func (s *SQLite) CreateReminder(ctx context.Context, req actorstore.CreateReminderRequest) error {
	if !req.IsValid() {
		return actorstore.ErrInvalidRequestMissingParameters
	}

	var executionTime time.Time
	if !req.ExecutionTime.IsZero() {
		executionTime = req.ExecutionTime
	} else {
		// Note that delay could be zero
		executionTime = s.clock.Now().Add(req.Delay)
	}

	var ttl int64
	if req.TTL != nil {
		ttl = req.TTL.UnixMilli()
	}

	reminderID, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("failed to generate reminder ID: %w", err)
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, s.metadata.Timeout)
	defer queryCancel()
	q := `
REPLACE INTO reminders
    (reminder_id, actor_type, actor_id, reminder_name, reminder_execution_time, reminder_period, reminder_ttl, reminder_data, reminder_lease_time, reminder_lease_pid)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL)`
	_, err = s.db.ExecContext(queryCtx, q,
		reminderID[:],
		req.ActorType, req.ActorID, req.Name,
		executionTime.UnixMilli(), req.Period, ttl, req.Data)
	if err != nil {
		return fmt.Errorf("failed to create reminder: %w", err)
	}
	return nil
}

func (s *SQLite) CreateLeasedReminder(ctx context.Context, req actorstore.CreateLeasedReminderRequest) (*actorstore.FetchedReminder, error) {
	if !req.Reminder.IsValid() {
		return nil, actorstore.ErrInvalidRequestMissingParameters
	}

	panic("TODO")
}

func (s *SQLite) DeleteReminder(ctx context.Context, req actorstore.ReminderRef) error {
	if !req.IsValid() {
		return actorstore.ErrInvalidRequestMissingParameters
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, s.metadata.Timeout)
	defer queryCancel()
	res, err := s.db.ExecContext(queryCtx,
		`DELETE FROM reminders WHERE actor_type = ? AND actor_id = ? AND reminder_name = ?`,
		req.ActorType, req.ActorID, req.Name,
	)
	if err != nil {
		return fmt.Errorf("failed to delete reminder: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to count affected rows: %w", err)
	}
	if affected == 0 {
		return actorstore.ErrReminderNotFound
	}
	return nil
}

func (s *SQLite) FetchNextReminders(ctx context.Context, req actorstore.FetchNextRemindersRequest) ([]*actorstore.FetchedReminder, error) {
	// If there's no host or supported actor types, that means there's nothing to return
	if len(req.Hosts) == 0 && len(req.ActorTypes) == 0 {
		return nil, nil
	}

	// Parse all req.Hosts as uuid.UUID
	hosts, err := newHostListFromReq(req.Hosts)
	if err != nil {
		return nil, err
	}

	// We need to do this in a transaction for consistency
	return executeInTransaction(ctx, s.logger, s.db, s.metadata.Timeout, func(ctx context.Context, tx *sql.Tx) ([]*actorstore.FetchedReminder, error) {
		now := s.clock.Now().UnixMilli()
		minLastHealthCheck := now - s.metadata.Config.FailedInterval().Milliseconds()
		minReminderLease := now - s.metadata.Config.RemindersLeaseDuration.Milliseconds()
		maxFetchAhead := now + s.metadata.Config.RemindersFetchAheadInterval.Milliseconds()

		// To start, load the initial capacity of each host, based on how many reminders are already active on that host
		hostCapacities := make(hostCapacitiesMap, len(req.ActorTypes))
		queryParams := make([]any, len(hosts)+2)
		queryParams[0] = minReminderLease
		queryParams[1] = minLastHealthCheck
		hostsList := hosts.getQuerySegment("hosts.", queryParams, 2) // This also appends to queryParams

		queryCtx, queryCancel := context.WithTimeout(ctx, s.metadata.Timeout)
		defer queryCancel()
		//nolint:gosec
		rows, err := tx.QueryContext(queryCtx, `
SELECT
  hat.host_id,
  hat.actor_type,
  (
    SELECT COUNT(reminders.rowid)
    FROM reminders
    LEFT JOIN actors
      USING (actor_id, actor_type)
    WHERE
      actors.host_id = hat.host_id
      AND reminders.actor_type = hat.actor_type
      AND reminders.reminder_lease_time >= ?
  ),
  hat.actor_concurrent_reminders
FROM hosts_actor_types AS hat
LEFT JOIN hosts
  ON hat.host_id = hosts.host_id
WHERE
  hosts.host_last_healthcheck >= ?
  AND (`+hostsList+`)`,
			queryParams...,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load initial capacities of hosts: %w", err)
		}
		hostSet := make(map[uuid.UUID]struct{}, len(hosts))
		for rows.Next() {
			var (
				hostIDB   []byte
				hostID    uuid.UUID
				actorType string
				used, max int32
			)
			err = rows.Scan(&hostIDB, &actorType, &used, &max)
			if err != nil {
				return nil, fmt.Errorf("failed to load initial capacities of hosts: %w", err)
			}
			hostID, err = uuid.FromBytes(hostIDB)
			if err != nil {
				return nil, fmt.Errorf("failed to load initial capacities of hosts: invalid host ID: %w", err)
			}
			if max <= 0 {
				max = math.MaxInt32
			}

			// Only add hosts that have capacity
			if used < max {
				hostCapacities.addHost(actorType, hostID, max-used, len(hosts))
				hostSet[hostID] = struct{}{}
			}
		}

		// If there's no host (with capacity), just return here
		if len(hostSet) == 0 {
			return nil, nil
		}

		// Update hosts
		hosts.updatFromSet(hostSet)

		// Allocate a slice for the reminder IDs and for the actors to allocate
		fetchedReminderIDs := make([][]byte, 0, s.metadata.Config.RemindersFetchAheadBatchSize)
		allocateActors := make([]allocateActor, 0, s.metadata.Config.RemindersFetchAheadBatchSize)

		// Load all upcoming reminders for all actors that are active on hosts in the capacities table (all of which have some capacity)
		// This also loads reminders for actors that are not active, but which can be executed on hosts currently connected
		queryParams = make([]any, 4+len(hosts)+len(req.ActorTypes))
		queryParams[0] = minLastHealthCheck
		queryParams[1] = maxFetchAhead
		queryParams[2] = minReminderLease
		actorTypesList := stringList(req.ActorTypes).getQuerySegment("rr.actor_type", queryParams, 3) // This appends to queryParams
		hostsList = hosts.getQuerySegment("hosts.", queryParams, 3+len(req.ActorTypes))               // This appends to queryParams
		queryParams[3+len(hosts)+len(req.ActorTypes)] = s.metadata.Config.RemindersFetchAheadBatchSize

		queryCtx, queryCancel = context.WithTimeout(ctx, s.metadata.Timeout)
		defer queryCancel()
		//nolint:gosec
		rows, err = tx.QueryContext(queryCtx, `
SELECT
  rr.reminder_id, actors.host_id,
  rr.actor_type, rr.actor_id,
  rr.reminder_execution_time
FROM reminders AS rr
LEFT JOIN actors
  USING (actor_type, actor_id)
LEFT JOIN hosts
  ON actors.host_id = hosts.host_id AND hosts.host_last_healthcheck >= ?
WHERE
  rr.reminder_execution_time <= ?
  AND (
    rr.reminder_lease_id IS NULL
    OR rr.reminder_lease_time IS NULL
    OR rr.reminder_lease_time < ?
  )
  AND (
	(
      hosts.host_id IS NULL
      AND (`+actorTypesList+`)
    )
	OR `+hostsList+`
  )
ORDER BY rr.reminder_execution_time ASC
LIMIT ?
`, queryParams...)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch upcoming reminders: %w", err)
		}
		for rows.Next() {
			var (
				hostID []byte
				act    allocateActor
			)
			err = rows.Scan(&act.reminderID, &hostID, &act.actorType, &act.actorID, &act.reminderExecutionTime)
			if err != nil {
				return nil, fmt.Errorf("failed to load initial capacities of hosts: %w", err)
			}

			// For the reminders that have an active actor, filter based on the capacity
			if len(hostID) > 0 && hostCapacities.DecreaseCapacityWithHostIDBytes(act.actorType, hostID) {
				fetchedReminderIDs = append(fetchedReminderIDs, act.reminderID)
			} else if len(hostID) == 0 {
				// This reminder does not have an active actor, so we need to record that to be activated later
				allocateActors = append(allocateActors, act)
			}
		}

		// Allocate actors that were not already active
		allocatedSet := make(map[string]struct{}, len(allocateActors))
		for _, act := range allocateActors {
			key := act.ActorKey()

			// Check if this was already allocated
			_, ok := allocatedSet[key]
			if !ok {
				// Actor needs to be allocated
				// Pick a random host that can host this actor
				// This also decreases the host's capacity
				hostID, ok := hostCapacities.SelectHostForActor(act.actorType)
				if !ok {
					// Could not get a host with capacity
					continue
				}

				// Create the actor now
				// Here we can do an upsert because we know that, if the row is present, it means the actor was active on a host that is dead but not GC'd yet
				// We set the activation to the current timestamp + the delay
				activation := act.reminderExecutionTime
				if now > activation {
					activation = now
				}
				queryCtx, queryCancel = context.WithTimeout(ctx, s.metadata.Timeout)
				defer queryCancel()
				_, err = tx.QueryContext(queryCtx, `
	REPLACE INTO actors
	  (actor_type, actor_id, host_id, actor_activation, actor_idle_timeout)
	SELECT 
	  ?, ?, ?, ?,
	  (
		SELECT hat.actor_idle_timeout
		FROM hosts_actor_types AS hat
		WHERE hat.actor_type = ? AND hat.host_id = ?
	  )
	`, act.actorType, act.actorID, hostID[:], activation, act.actorType, hostID[:])
				if err != nil {
					return nil, fmt.Errorf("failed to activate actor: %w", err)
				}

				// Mark this actor as activated so avoid re-activating it
				allocatedSet[key] = struct{}{}
			}

			// Actor is allocated now
			// We can now add the reminder ID to those that have been fetched
			fetchedReminderIDs = append(fetchedReminderIDs, act.reminderID)
		}

		// Now that we have all the reminder IDs, return the fetched reminders
		res := make([]*actorstore.FetchedReminder, len(fetchedReminderIDs))
		for i, reminderID := range fetchedReminderIDs {
			fr, err := s.fetchReminderWithID(ctx, tx, reminderID, now)
			if err != nil {
				return nil, err
			}
			res[i] = fr
		}
		return res, nil
	})
}

func (s *SQLite) fetchReminderWithID(ctx context.Context, tx *sql.Tx, reminderID []byte, now int64) (*actorstore.FetchedReminder, error) {
	leaseID := make([]byte, 16)
	_, err := io.ReadFull(rand.Reader, leaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate lease ID: %w", err)
	}

	var (
		actorType, actorID, name string
		executionTime            int64
	)
	lease := leaseData{
		reminderID: reminderID,
		leaseID:    leaseID,
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, s.metadata.Timeout)
	defer queryCancel()
	// Linter complains that this query should use ExecContext, but we need QueryRowContext because it has a RETURNING clause
	//nolint:execinquery
	err = tx.
		QueryRowContext(queryCtx, `
UPDATE reminders
SET
  reminder_lease_id = ?,
  reminder_lease_time = ?,
  reminder_lease_pid = ?
WHERE reminder_id = ?
RETURNING
  actor_type, actor_id, reminder_name, reminder_execution_time
`,
			leaseID, now, s.metadata.PID, reminderID,
		).
		Scan(&actorType, &actorID, &name, &executionTime)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch upcoming reminder: %w", err)
	}

	r := actorstore.NewFetchedReminder(
		actorType+"||"+actorID+"||"+name,
		time.UnixMilli(executionTime),
		lease,
	)
	return &r, nil
}

func (s *SQLite) GetReminderWithLease(ctx context.Context, fr *actorstore.FetchedReminder) (res actorstore.Reminder, err error) {
	if fr == nil {
		return res, errors.New("reminer object is nil")
	}
	lease, ok := fr.Lease().(leaseData)
	if !ok || !lease.IsValid() {
		return res, errors.New("invalid reminder lease object")
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, s.metadata.Timeout)
	defer queryCancel()

	var (
		executionTime int64
		ttl           *int64
	)
	err = s.db.
		QueryRowContext(queryCtx, `
SELECT
	actor_type, actor_id, reminder_name,
	reminder_execution_time, reminder_period, reminder_ttl, reminder_data
FROM reminders
WHERE
	reminder_id = ?
	AND reminder_lease_id = ?
	AND reminder_lease_pid = ?
	AND reminder_lease_time IS NOT NULL
	AND reminder_lease_time >= ?
`,
			lease.reminderID, lease.leaseID, s.metadata.PID,
			s.clock.Now().Add(-1*s.metadata.Config.RemindersLeaseDuration).UnixMilli(),
		).
		Scan(
			&res.ActorType, &res.ActorID, &res.Name,
			&executionTime, &res.Period, &ttl, &res.Data,
		)
	if errors.Is(err, sql.ErrNoRows) {
		return res, actorstore.ErrReminderNotFound
	} else if err != nil {
		return res, fmt.Errorf("failed to retrieve reminder: %w", err)
	}

	// The query returns the execution time and TTL as UNIX timestamps with milliseconds
	res.ExecutionTime = time.UnixMilli(executionTime)
	if ttl != nil && *ttl > 0 {
		res.TTL = ptr.Of(time.UnixMilli(*ttl))
	}

	return res, nil
}

func (s *SQLite) UpdateReminderWithLease(ctx context.Context, fr *actorstore.FetchedReminder, req actorstore.UpdateReminderWithLeaseRequest) (err error) {
	if fr == nil {
		return errors.New("reminer object is nil")
	}
	lease, ok := fr.Lease().(leaseData)
	if !ok || !lease.IsValid() {
		return errors.New("invalid reminder lease object")
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, s.metadata.Timeout)
	defer queryCancel()

	now := s.clock.Now()

	var ttl int64
	if req.TTL != nil {
		ttl = req.TTL.UnixMilli()
	}

	// Unless KeepLease is true, we also release the lease on the reminder
	var res sql.Result
	if !req.KeepLease {
		res, err = s.db.ExecContext(queryCtx, `
UPDATE reminders SET
	reminder_lease_id = NULL,
	reminder_lease_time = NULL,
	reminder_lease_pid = NULL,
	reminder_execution_time = ?,
	reminder_period = ?,
	reminder_ttl = ?
WHERE
	reminder_id = ?
	AND reminder_lease_id = ?
	AND reminder_lease_pid = ?
	AND reminder_lease_time IS NOT NULL
	AND reminder_lease_time >= ?`,
			req.ExecutionTime.UnixMilli(), req.Period, ttl,
			lease.reminderID, lease.leaseID, s.metadata.PID,
			now.Add(-1*s.metadata.Config.RemindersLeaseDuration).UnixMilli(),
		)
	} else {
		// Refresh the lease without releasing it
		res, err = s.db.ExecContext(queryCtx, `
UPDATE reminders SET
	reminder_lease_time = ?,
	reminder_execution_time = ?,
	reminder_period = ?,
	reminder_ttl = ?
WHERE
	reminder_id = ?
	AND reminder_lease_id = ?
	AND reminder_lease_pid = ?
	AND reminder_lease_time IS NOT NULL
	AND reminder_lease_time >= ?`,
			now.UnixMilli(),
			req.ExecutionTime.UnixMilli(), req.Period, ttl,
			lease.reminderID, lease.leaseID, s.metadata.PID,
			now.Add(-1*s.metadata.Config.RemindersLeaseDuration).UnixMilli(),
		)
	}
	if err != nil {
		return fmt.Errorf("failed to update reminder: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to count affected rows: %w", err)
	}
	if affected == 0 {
		return actorstore.ErrReminderNotFound
	}
	return nil
}

func (s *SQLite) DeleteReminderWithLease(ctx context.Context, fr *actorstore.FetchedReminder) error {
	if fr == nil {
		return errors.New("reminer object is nil")
	}
	lease, ok := fr.Lease().(leaseData)
	if !ok || !lease.IsValid() {
		return errors.New("invalid reminder lease object")
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, s.metadata.Timeout)
	defer queryCancel()
	res, err := s.db.ExecContext(queryCtx, `
DELETE FROM reminders
WHERE
	reminder_id = ?
	AND reminder_lease_id = ?
	AND reminder_lease_pid = ?
	AND reminder_lease_time IS NOT NULL
	AND reminder_lease_time >= ?`,
		lease.reminderID, lease.leaseID, s.metadata.PID,
		s.clock.Now().Add(-1*s.metadata.Config.RemindersLeaseDuration).UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("failed to delete reminder: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to count affected rows: %w", err)
	}
	if affected == 0 {
		return actorstore.ErrReminderNotFound
	}
	return nil
}

func (s *SQLite) RenewReminderLeases(ctx context.Context, req actorstore.RenewReminderLeasesRequest) (int64, error) {
	// If there's no connected host or no supported actor type, do not renew any lease
	if len(req.Hosts) == 0 && len(req.ActorTypes) == 0 {
		return 0, nil
	}

	hosts, err := newHostListFromReq(req.Hosts)
	if err != nil {
		return 0, err
	}

	now := s.clock.Now().UnixMilli()
	minReminderLease := now - s.metadata.Config.RemindersLeaseDuration.Milliseconds()

	queryParams := make([]any, 3+len(hosts)+len(req.ActorTypes))
	queryParams[0] = now
	queryParams[1] = s.metadata.PID
	queryParams[2] = minReminderLease
	actorTypesList := stringList(req.ActorTypes).getQuerySegment("reminders.actor_type", queryParams, 3) // This appends to queryParams
	hostsList := hosts.getQuerySegment("actors.", queryParams, 3+len(req.ActorTypes))                    // This appends to queryParams

	queryCtx, queryCancel := context.WithTimeout(ctx, s.metadata.Timeout)
	defer queryCancel()

	res, err := s.db.ExecContext(queryCtx, `
UPDATE reminders
SET reminder_lease_time = ?
WHERE reminder_id IN (
  SELECT reminder_id
  FROM reminders
  LEFT JOIN actors
    ON actors.actor_type = reminders.actor_type AND actors.actor_id = reminders.actor_id
  WHERE
    reminders.reminder_lease_pid = ?
    AND reminders.reminder_lease_time IS NOT NULL
    AND reminders.reminder_lease_time >= ?
    AND reminders.reminder_lease_id IS NOT NULL
    AND (
      (
        actors.host_id IS NULL
        AND (`+actorTypesList+`)
      )
      OR `+hostsList+`
    )
)`,
		queryParams...,
	)
	if err != nil {
		return 0, fmt.Errorf("database error: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to count affected rows: %w", err)
	}

	return affected, nil

}

func (s *SQLite) RelinquishReminderLease(ctx context.Context, fr *actorstore.FetchedReminder) error {
	if fr == nil {
		return errors.New("reminer object is nil")
	}
	lease, ok := fr.Lease().(leaseData)
	if !ok || !lease.IsValid() {
		return errors.New("invalid reminder lease object")
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, s.metadata.Timeout)
	defer queryCancel()
	res, err := s.db.ExecContext(queryCtx,
		// Note that here we don't check for `reminder_lease_time` as we are relinquishing the lease anyways
		`UPDATE reminders
			SET
				reminder_lease_id = NULL,
				reminder_lease_time = NULL,
				reminder_lease_pid = NULL
			WHERE
				reminder_id = ?
				AND reminder_lease_id = ?
				AND reminder_lease_pid = ?`,
		lease.reminderID, lease.leaseID, s.metadata.PID,
	)
	if err != nil {
		return fmt.Errorf("failed to relinquish lease for reminder: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to count affected rows: %w", err)
	}
	if affected == 0 {
		return actorstore.ErrReminderNotFound
	}
	return nil
}

type leaseData struct {
	reminderID []byte
	leaseID    []byte
}

func (ld leaseData) IsValid() bool {
	return len(ld.reminderID) > 0 && len(ld.leaseID) > 0
}

type hostList []uuid.UUID

func newHostListFromReq(reqHosts []string) (hostList, error) {
	res := make(hostList, len(reqHosts))
	for i := 0; i < len(reqHosts); i++ {
		var err error
		res[i], err = uuid.Parse(reqHosts[i])
		if err != nil {
			return nil, actorstore.ErrInvalidRequestMissingParameters
		}
	}
	return res, nil
}

func (hosts hostList) getQuerySegment(prefix string, params []any, paramsOffset int) string {
	var b strings.Builder
	for i := range hosts {
		if i != 0 {
			b.WriteString(" OR " + prefix + "host_id = ?")
		} else {
			b.WriteString(prefix + "host_id = ?")
		}
		params[i+paramsOffset] = hosts[i][:]
	}
	return b.String()
}

func (hosts *hostList) updatFromSet(set map[uuid.UUID]struct{}) {
	*hosts = (*hosts)[0:len(set)]
	var i int
	for k := range set {
		(*hosts)[i] = k
		i++
	}
}

type stringList []string

func (l stringList) getQuerySegment(col string, params []any, paramsOffset int) string {
	var b strings.Builder
	for i := range l {
		if i != 0 {
			b.WriteString(" OR " + col + " = ?")
		} else {
			b.WriteString(col + " = ?")
		}
		params[i+paramsOffset] = l[i]
	}
	return b.String()
}

// actorType -> host ID -> capacity
type hostCapacitiesMap map[string]map[uuid.UUID]int32

func (m hostCapacitiesMap) addHost(actorType string, hostID uuid.UUID, capacity int32, allocSize int) {
	if m[actorType] == nil {
		m[actorType] = make(map[uuid.UUID]int32, allocSize)
	}
	m[actorType][hostID] = capacity
}

func (m hostCapacitiesMap) GetCapacity(actorType string, hostID uuid.UUID) int32 {
	if m[actorType] == nil {
		return 0
	}
	return m[actorType][hostID]
}

func (m hostCapacitiesMap) GetCapacityWithHostIDBytes(actorType string, hostID []byte) int32 {
	hostIDUUID, err := uuid.FromBytes(hostID)
	if err != nil {
		return 0
	}

	return m.GetCapacity(actorType, hostIDUUID)
}

func (m hostCapacitiesMap) DecreaseCapacity(actorType string, hostID uuid.UUID) bool {
	if m[actorType] == nil || m[actorType][hostID] <= 0 {
		return false
	}
	m[actorType][hostID]--
	return true
}

func (m hostCapacitiesMap) DecreaseCapacityWithHostIDBytes(actorType string, hostID []byte) bool {
	hostIDUUID, err := uuid.FromBytes(hostID)
	if err != nil {
		return false
	}
	return m.DecreaseCapacity(actorType, hostIDUUID)
}

// SelectHostForActor selects a host for the given actor type that has capacity to host an actor
// It decreases the host's capacity automatically
func (m hostCapacitiesMap) SelectHostForActor(actorType string) (host uuid.UUID, ok bool) {
	at := m[actorType]
	if len(at) == 0 {
		return host, false
	}

	// Filter hosts with capacity
	hosts := make([]uuid.UUID, len(at))
	var n int
	for k, v := range at {
		if v <= 0 {
			// No capacity
			continue
		}
		hosts[n] = k
		n++
	}
	hosts = hosts[:n]

	switch len(hosts) {
	case 0:
		return host, false
	case 1:
		host = hosts[0]
	default:
		// Pick a random one
		// We use math/rand here which is not "secure" as we don't need a CSPRNG
		host = hosts[mrand.Int()%len(hosts)]
	}

	// Decrease the capacity
	m[actorType][host]--
	return host, true
}

type allocateActor struct {
	reminderID            []byte
	actorType, actorID    string
	reminderExecutionTime int64
}

func (a allocateActor) ActorKey() string {
	return a.actorType + "||" + a.actorID
}
