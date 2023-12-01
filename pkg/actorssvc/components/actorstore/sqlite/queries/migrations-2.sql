-- Query for performing migration #2
-- This creates tables for reminders
CREATE TABLE reminders (
  reminder_id blob PRIMARY KEY NOT NULL, 
  actor_type text NOT NULL,
  actor_id text NOT NULL,
  reminder_name text NOT NULL,
  reminder_execution_time integer NOT NULL,
  reminder_period text,
  reminder_ttl integer,
  reminder_data blob,
  reminder_lease_id text,
  reminder_lease_time integer,
  reminder_lease_pid text
);

CREATE UNIQUE INDEX ON reminders (actor_type, actor_id, reminder_name);
CREATE INDEX ON reminders (reminder_execution_time);
CREATE INDEX ON reminders (reminder_lease_pid);
