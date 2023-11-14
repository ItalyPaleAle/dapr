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
	// Blank import for the embed package.
	_ "embed"
)

var (
	//go:embed queries/migrations-1.sql
	migration1Query string
	//go:embed queries/migrations-2.sql
	migration2Query string
	//go:embed queries/migrations-3.sql
	migration3Query string
)

// Query for looking up an actor, or creating it ex novo.
//
// The purpose of this query is to perform an atomic "load or set". Given an actor ID and type, it will:
// - If there's already a row in the table with the same actor ID and type, AND the last healthcheck hasn't expired, returns the row
// - If there's no row in the table with the same actor ID and type, OR if there's a row but the last healthcheck has expired, inserts a new row (performing an "upsert" if the row already exists)
// In both cases, the query lookups up the actor host's ID, and then returns the actor host's address and app ID, and the idle timeout configured for the actor type
//
// Note that in case of 2 requests at the same time when the row doesn't exist, this may fail with a race condition.
// You will get an empty result. The query can be retried in that case.
//
// Query arguments:
// 1. Actor type, as `string`
// 2. Actor ID, as `string`
// 3. Health check interval, as `time.Duration`
//
// fmt.Sprintf arguments:
// 1. Name of the "hosts" table
// 2. Name of the "hosts_actor_types" table
// 3. Name of the "actors" table
//
// Inspired by: https://stackoverflow.com/a/72033548/192024
// The additional "WHERE" in the "ON CONFLICT" clause is to prevent race conditions with other callers executing the same INSERT ... ON CONFLICT DO UPDATE at the same time
const lookupActorQuery = `WITH new_row AS (
  INSERT INTO %[3]s (actor_type, actor_id, host_id, actor_idle_timeout, actor_activation)
    SELECT $1, $2, %[2]s.host_id, %[2]s.actor_idle_timeout, CURRENT_TIMESTAMP
      FROM %[2]s, %[1]s
      WHERE
        %[2]s.actor_type = $1
        AND %[1]s.host_id = %[2]s.host_id
        AND %[1]s.host_last_healthcheck >= CURRENT_TIMESTAMP - $3::interval
        AND NOT EXISTS (
          SELECT %[3]s.host_id
            FROM %[3]s, %[1]s
            WHERE
              %[3]s.actor_type = $1
              AND %[3]s.actor_id = $2
              AND %[3]s.host_id = %[1]s.host_id
              AND %[1]s.host_last_healthcheck >= CURRENT_TIMESTAMP - $3::interval
        )
      ORDER BY random() LIMIT 1
    ON CONFLICT (actor_type, actor_id) DO UPDATE
      SET
        host_id = EXCLUDED.host_id, actor_idle_timeout = EXCLUDED.actor_idle_timeout, actor_activation = EXCLUDED.actor_activation
      WHERE
        EXISTS (
          SELECT 1
            FROM %[1]s
            WHERE
              %[3]s.host_id = %[1]s.host_id
              AND %[1]s.host_last_healthcheck < CURRENT_TIMESTAMP - $3::interval
        )
    RETURNING host_id, actor_idle_timeout
)
(
  SELECT %[1]s.host_id, %[1]s.host_app_id, %[1]s.host_address, %[3]s.actor_idle_timeout
    FROM %[3]s, %[1]s
    WHERE
      %[3]s.actor_type = $1
      AND %[3]s.actor_id = $2
      AND %[3]s.host_id = %[1]s.host_id
      AND %[1]s.host_last_healthcheck >= CURRENT_TIMESTAMP - $3::interval
  UNION ALL
  SELECT %[1]s.host_id, %[1]s.host_app_id, %[1]s.host_address, new_row.actor_idle_timeout
    FROM new_row, %[1]s
    WHERE
      new_row.host_id = %[1]s.host_id
      AND %[1]s.host_last_healthcheck >= CURRENT_TIMESTAMP - $3::interval
) LIMIT 1;`

// Query for looking up an actor, or creating it ex novo, but limited to a set of host IDs.
//
// This is a variation of `lookupActorQuery` which restricts actors to be activated on a list of host IDs.
//
// Arguments are the same as for `lookupActorQuery`, but with an additional one:
// 4. IDs of actor hosts on which the actor could be activated, as a `string[]`
const lookupActorQueryWithHostRestriction = `WITH new_row AS (
  INSERT INTO %[3]s (actor_type, actor_id, host_id, actor_idle_timeout, actor_activation)
    SELECT $1, $2, %[2]s.host_id, %[2]s.actor_idle_timeout, CURRENT_TIMESTAMP
      FROM %[2]s, %[1]s
      WHERE
        %[2]s.actor_type = $1
        AND %[1]s.host_id = %[2]s.host_id
        AND %[1]s.host_id = ANY($4)
        AND %[1]s.host_last_healthcheck >= CURRENT_TIMESTAMP - $3::interval
        AND NOT EXISTS (
          SELECT %[3]s.host_id
            FROM %[3]s, %[1]s
            WHERE
              %[3]s.actor_type = $1
              AND %[3]s.actor_id = $2
              AND %[3]s.host_id = %[1]s.host_id
              AND %[1]s.host_last_healthcheck >= CURRENT_TIMESTAMP - $3::interval
        )
      ORDER BY random() LIMIT 1
    ON CONFLICT (actor_type, actor_id) DO UPDATE
      SET
        host_id = EXCLUDED.host_id, actor_idle_timeout = EXCLUDED.actor_idle_timeout, actor_activation = EXCLUDED.actor_activation
      WHERE
        EXISTS (
          SELECT 1
            FROM %[1]s
            WHERE
              %[3]s.host_id = %[1]s.host_id
              AND %[1]s.host_last_healthcheck < CURRENT_TIMESTAMP - $3::interval
        )
    RETURNING host_id, actor_idle_timeout
)
(
  SELECT %[1]s.host_id, %[1]s.host_app_id, %[1]s.host_address, %[3]s.actor_idle_timeout
    FROM %[3]s, %[1]s
    WHERE
      %[3]s.actor_type = $1
      AND %[3]s.actor_id = $2
      AND %[3]s.host_id = %[1]s.host_id
      AND %[1]s.host_id = ANY($4)
      AND %[1]s.host_last_healthcheck >= CURRENT_TIMESTAMP - $3::interval
  UNION ALL
  SELECT %[1]s.host_id, %[1]s.host_app_id, %[1]s.host_address, new_row.actor_idle_timeout
    FROM new_row, %[1]s
    WHERE
      new_row.host_id = %[1]s.host_id
      AND %[1]s.host_last_healthcheck >= CURRENT_TIMESTAMP - $3::interval
) LIMIT 1;`

// Query for fetching the upcoming reminders.
//
// This query retrieves a batch of upcoming reminders, and at the same time it updates the retrieved rows to "lock" them.
// Reminders are retrieved if they are to be executed within the next "fetch ahead" interval and if they don't already have a lease.
// This only fetches reminders for actors that are either one of:
// - Active on hosts that have a connection with the current instance of the actors service (argument $4)
// - Not active on any host, but whose type can be served by an actor host connected to the current instance of the actors service (argument $3)
//
// Query arguments:
// 1. Fetch ahead interval, as a `time.Duration`
// 2. Lease duration, as a `time.Duration`
// 3. IDs of actor hosts that have an active connection to the current instance of the actors service, as a `string[]`
// 4. Actor types that can be served by hosts connected to the current instance of the actors service, as a `string[]`
// 5. Health check interval, as `time.Duration`
// 6. Maximum batch size, as an `int`
// 7. Process ID, as a `string`
//
// fmt.Sprintf arguments:
// 1. Name of the "reminders" table
// 2. Name of the "fetch_reminders" function
const remindersFetchQuery = `
UPDATE %[1]s
SET
    reminder_lease_id = gen_random_uuid(),
    reminder_lease_time = CURRENT_TIMESTAMP,
    reminder_lease_pid = $7
WHERE reminder_id IN (
    SELECT * FROM %[3]s($1, $2, $3, $4, $5, $6)
)
RETURNING
    reminder_id, actor_type, actor_id, reminder_name,
    EXTRACT(EPOCH FROM reminder_execution_time - CURRENT_TIMESTAMP)::int,
    reminder_lease_id
`

// Query for creating (or replacing) a reminder and acquiring a lease at the same time.
//
// This query performs an upsert for the reminder.
// If the reminder that was upserted can be served by an actor host connected to this instance of the actors service (following the same logic as `remindersFetchQuery`), then it also sets the lease on the reminder, atomically.
//
// Query arguments:
// 1. Actor type, as a `string`
// 2. Actor ID, as a `string`
// 3. Reminder name, as a `string`
// 4. Reminder execution delay (from current time), as a `time.Duration`
// 5. Reminder period, as a `*string`
// 6. Reminder TTL, as a `*time.Time`
// 7. Reminder data, as a `[]byte` (can be nil)
// 8. Process ID, as a `string`
// 9. Actor types that can be served by hosts connected to the current instance of the actors service, as a `string[]`
// 10. IDs of actor hosts that have an active connection to the current instance of the actors service, as a `string[]`
//
// fmt.Sprintf arguments:
// 1. Name of the "reminders" table
// 2. Name of the "actors" table
const createReminderWithLeaseQuery = `WITH c AS (
  SELECT
      gen_random_uuid() AS reminder_lease_id,
      CURRENT_TIMESTAMP AS reminder_lease_time,
      $8 AS reminder_lease_pid
  FROM %[2]s
  WHERE
      actor_type = $1
      AND actor_id = $2
      AND (
          (
              host_id IS NULL
              AND actor_type = ANY($9)
          )
          OR host_id = ANY($10)
      )
), lease AS (
  SELECT
      c.reminder_lease_id,
      c.reminder_lease_time,
      c.reminder_lease_pid
  FROM c
  UNION ALL
      SELECT
          NULL AS reminder_lease_id,
          NULL AS reminder_lease_time,
          NULL AS reminder_lease_pid
      WHERE NOT EXISTS (
          SELECT 1 from c
      )
)
INSERT INTO %[1]s
    (actor_type, actor_id, reminder_name, reminder_execution_time, reminder_period, reminder_ttl, reminder_data, reminder_lease_id, reminder_lease_time, reminder_lease_pid)
  SELECT
    $1, $2, $3, CURRENT_TIMESTAMP + $4::interval, $5, $6, $7, lease.reminder_lease_id, lease.reminder_lease_time, lease.reminder_lease_pid
  FROM lease
  ON CONFLICT (actor_type, actor_id, reminder_name) DO UPDATE SET
    reminder_execution_time = EXCLUDED.reminder_execution_time,
    reminder_period = EXCLUDED.reminder_period,
    reminder_ttl = EXCLUDED.reminder_ttl,
    reminder_data = EXCLUDED.reminder_data,
    reminder_lease_id = EXCLUDED.reminder_lease_id,
    reminder_lease_time = EXCLUDED.reminder_lease_time,
    reminder_lease_pid = EXCLUDED.reminder_lease_pid
  RETURNING reminder_id, actor_type, actor_id, reminder_name,
    EXTRACT(EPOCH FROM reminder_execution_time - CURRENT_TIMESTAMP)::int,
    reminder_lease_id;
`
