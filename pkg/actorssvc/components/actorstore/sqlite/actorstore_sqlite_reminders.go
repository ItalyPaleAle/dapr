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
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dapr/dapr/pkg/actorssvc/components/actorstore"
)

func (p *SQLite) GetReminder(ctx context.Context, req actorstore.ReminderRef) (res actorstore.GetReminderResponse, err error) {
	if !req.IsValid() {
		return res, actorstore.ErrInvalidRequestMissingParameters
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, p.metadata.Timeout)
	defer queryCancel()

	var delay int
	err = p.db.
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
			req.ActorType, req.ActorID, req.Name).
		Scan(&delay, &res.Period, &res.TTL, &res.Data)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return res, actorstore.ErrReminderNotFound
		}
		return res, fmt.Errorf("failed to retrieve reminder: %w", err)
	}

	// The query doesn't return an exact time, but rather the number of seconds from present, to make sure we always use the clock of the DB server and avoid clock skews
	res.ExecutionTime = p.clock.Now().Add(time.Duration(delay) * time.Second)

	return res, nil
}

func (p *SQLite) CreateReminder(ctx context.Context, req actorstore.CreateReminderRequest) error {
	if !req.IsValid() {
		return actorstore.ErrInvalidRequestMissingParameters
	}

	// Do not store the exact time, but rather the delay from now, to use the DB server's clock
	var executionTime time.Duration
	if !req.ExecutionTime.IsZero() {
		executionTime = req.ExecutionTime.Sub(p.clock.Now())
	} else {
		// Note that delay could be zero
		executionTime = req.Delay
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, p.metadata.Timeout)
	defer queryCancel()
	q := `
INSERT INTO reminders
	(actor_type, actor_id, reminder_name, reminder_execution_time, reminder_period, reminder_ttl, reminder_data)
VALUES ($1, $2, $3, now() + $4::interval, $5, $6, $7)
ON CONFLICT (actor_type, actor_id, reminder_name) DO UPDATE SET
	reminder_execution_time = EXCLUDED.reminder_execution_time,
	reminder_period = EXCLUDED.reminder_period,
	reminder_ttl = EXCLUDED.reminder_ttl,
	reminder_data = EXCLUDED.reminder_data,
	reminder_lease_time = NULL,
	reminder_lease_pid = NULL`
	_, err := p.db.ExecContext(queryCtx, q, req.ActorType, req.ActorID, req.Name, executionTime, req.Period, req.TTL, req.Data)
	if err != nil {
		return fmt.Errorf("failed to create reminder: %w", err)
	}
	return nil
}

func (p *SQLite) CreateLeasedReminder(ctx context.Context, req actorstore.CreateLeasedReminderRequest) (*actorstore.FetchedReminder, error) {
	if !req.Reminder.IsValid() {
		return nil, actorstore.ErrInvalidRequestMissingParameters
	}

	panic("TODO")
}

func (p *SQLite) DeleteReminder(ctx context.Context, req actorstore.ReminderRef) error {
	if !req.IsValid() {
		return actorstore.ErrInvalidRequestMissingParameters
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, p.metadata.Timeout)
	defer queryCancel()
	res, err := p.db.ExecContext(queryCtx,
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

func (p *SQLite) FetchNextReminders(ctx context.Context, req actorstore.FetchNextRemindersRequest) ([]*actorstore.FetchedReminder, error) {
	// If there's no host or supported actor types, that means there's nothing to return
	if len(req.Hosts) == 0 && len(req.ActorTypes) == 0 {
		return nil, nil
	}

	panic("TODO")
}

func (p *SQLite) GetReminderWithLease(ctx context.Context, fr *actorstore.FetchedReminder) (res actorstore.Reminder, err error) {
	if fr == nil {
		return res, errors.New("reminer object is nil")
	}
	lease, ok := fr.Lease().(leaseData)
	if !ok || !lease.IsValid() {
		return res, errors.New("invalid reminder lease object")
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, p.metadata.Timeout)
	defer queryCancel()

	q := `SELECT
	actor_type, actor_id, reminder_name,
	EXTRACT(EPOCH FROM reminder_execution_time - now())::int,
	reminder_period, reminder_ttl, reminder_data
FROM reminders
WHERE
	reminder_id = ?
	AND reminder_lease_id = ?
	AND reminder_lease_pid = ?
	AND reminder_lease_time IS NOT NULL
	AND reminder_lease_time >= now() - ?`
	var delay int
	err = p.db.
		QueryRowContext(queryCtx, q, lease.reminderID, *lease.leaseID, p.metadata.PID, p.metadata.Config.RemindersLeaseDuration).
		Scan(
			&res.ActorType, &res.ActorID, &res.Name,
			&delay, &res.Period, &res.TTL, &res.Data,
		)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return res, actorstore.ErrReminderNotFound
		}
		return res, fmt.Errorf("failed to retrieve reminder: %w", err)
	}

	// The query doesn't return an exact time, but rather the number of seconds from present, to make sure we always use the clock of the DB server and avoid clock skews
	res.ExecutionTime = p.clock.Now().Add(time.Duration(delay) * time.Second)

	return res, nil
}

func (p *SQLite) UpdateReminderWithLease(ctx context.Context, fr *actorstore.FetchedReminder, req actorstore.UpdateReminderWithLeaseRequest) error {
	if fr == nil {
		return errors.New("reminer object is nil")
	}
	lease, ok := fr.Lease().(leaseData)
	if !ok || !lease.IsValid() {
		return errors.New("invalid reminder lease object")
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, p.metadata.Timeout)
	defer queryCancel()

	// Unless KeepLease is true, we also release the lease on the reminder
	var leaseQuery string
	if !req.KeepLease {
		leaseQuery = "reminder_lease_id = NULL, reminder_lease_time = NULL, reminder_lease_pid = NULL,"
	} else {
		// Refresh the lease without releasing it
		leaseQuery = "reminder_lease_time = now(),"
	}

	res, err := p.db.ExecContext(queryCtx, `
UPDATE reminders SET
	`+leaseQuery+`
	reminder_execution_time = ?,
	reminder_period = ?,
	reminder_ttl = ?
WHERE
	reminder_id = ?
	AND reminder_lease_id = ?
	AND reminder_lease_pid = ?
	AND reminder_lease_time IS NOT NULL
	AND reminder_lease_time >= now() - ?::interval`,
		req.ExecutionTime, req.Period, req.TTL,
		lease.reminderID, *lease.leaseID, p.metadata.PID, p.metadata.Config.RemindersLeaseDuration,
	)
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

func (p *SQLite) DeleteReminderWithLease(ctx context.Context, fr *actorstore.FetchedReminder) error {
	if fr == nil {
		return errors.New("reminer object is nil")
	}
	lease, ok := fr.Lease().(leaseData)
	if !ok || !lease.IsValid() {
		return errors.New("invalid reminder lease object")
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, p.metadata.Timeout)
	defer queryCancel()
	res, err := p.db.ExecContext(queryCtx, `
DELETE FROM reminders
WHERE
	reminder_id = ?
	AND reminder_lease_id = ?
	AND reminder_lease_pid = ?
	AND reminder_lease_time IS NOT NULL
	AND reminder_lease_time >= now() - ?::interval`,
		lease.reminderID, *lease.leaseID, p.metadata.PID, p.metadata.Config.RemindersLeaseDuration,
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

func (p *SQLite) RenewReminderLeases(ctx context.Context, req actorstore.RenewReminderLeasesRequest) (int64, error) {
	// If there's no connected host or no supported actor type, do not renew any lease
	if len(req.Hosts) == 0 && len(req.ActorTypes) == 0 {
		return 0, nil
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, p.metadata.Timeout)
	defer queryCancel()

	res, err := p.db.ExecContext(queryCtx, `
UPDATE reminders
SET reminder_lease_time = now()
WHERE reminder_id IN (
	SELECT reminder_id
	FROM reminders
	LEFT JOIN actors
		ON actors.actor_type = reminders.actor_type AND actors.actor_id = reminders.actor_id
	WHERE 
		reminders.reminder_lease_pid = ?
		AND reminders.reminder_lease_time IS NOT NULL
		AND reminders.reminder_lease_time >= now() - ?::interval
		AND reminders.reminder_lease_id IS NOT NULL
		AND (
			(
				actors.host_id IS NULL
				AND reminders.actor_type = ANY(?)
			)
			OR actors.host_id = ANY(?)
		)
)`,
		p.metadata.PID, p.metadata.Config.RemindersLeaseDuration,
		req.ActorTypes, req.Hosts,
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

func (p *SQLite) RelinquishReminderLease(ctx context.Context, fr *actorstore.FetchedReminder) error {
	if fr == nil {
		return errors.New("reminer object is nil")
	}
	lease, ok := fr.Lease().(leaseData)
	if !ok || !lease.IsValid() {
		return errors.New("invalid reminder lease object")
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, p.metadata.Timeout)
	defer queryCancel()
	res, err := p.db.ExecContext(queryCtx,
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
		lease.reminderID, *lease.leaseID, p.metadata.PID,
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

func (p *SQLite) scanFetchedReminderRow(row pgx.Row, now time.Time) (*actorstore.FetchedReminder, error) {
	var (
		actorType, actorID, name string
		delay                    int
		lease                    leaseData
	)
	err := row.Scan(&lease.reminderID, &actorType, &actorID, &name, &delay, &lease.leaseID)
	if err != nil {
		return nil, err
	}

	// If we couldn't get a lease, return nil
	if lease.leaseID == nil || *lease.leaseID == "" {
		return nil, nil
	}

	// The query doesn't return an exact time, but rather the number of seconds from present, to make sure we always use the clock of the DB server and avoid clock skews
	r := actorstore.NewFetchedReminder(
		actorType+"||"+actorID+"||"+name,
		now.Add(time.Duration(delay)*time.Second),
		lease,
	)
	return &r, nil
}

type leaseData struct {
	reminderID string
	leaseID    *string
}

func (ld leaseData) IsValid() bool {
	return ld.reminderID != "" && ld.leaseID != nil && *ld.leaseID != ""
}
