-- Query for performing migration #1
-- This creates the tables for actor state

-- "hosts" table
CREATE TABLE hosts (
  host_id blob PRIMARY KEY NOT NULL,
  host_address text NOT NULL,
  host_app_id text NOT NULL,
  host_actors_api_level integer NOT NULL,
  host_last_reported_api_level integer NOT NULL DEFAULT 0,
  host_last_healthcheck integer NOT NULL
);

CREATE UNIQUE INDEX host_address_idx ON hosts (host_address);
CREATE INDEX host_last_healthcheck_idx ON hosts (host_last_healthcheck);
CREATE INDEX host_actors_api_level_idx ON hosts (host_actors_api_level);
CREATE INDEX host_last_reported_api_level_idx ON hosts (host_last_reported_api_level);

-- "hosts_actor_types" table
CREATE TABLE hosts_actor_types (
  host_id blob NOT NULL,
  actor_type text NOT NULL,
  actor_idle_timeout integer NOT NULL,
  actor_concurrent_reminders integer NOT NULL DEFAULT 0,
  PRIMARY KEY (host_id, actor_type),
  FOREIGN KEY (host_id) REFERENCES hosts (host_id) ON DELETE CASCADE
);

CREATE INDEX actor_type_idx ON hosts_actor_types (actor_type);

-- "actors" table
CREATE TABLE actors (
  actor_type text NOT NULL,
  actor_id text NOT NULL,
  host_id blob NOT NULL,
  actor_idle_timeout integer NOT NULL,
  actor_activation integer NOT NULL DEFAULT 0,
  PRIMARY KEY (actor_type, actor_id),
  FOREIGN KEY (host_id) REFERENCES hosts (host_id) ON DELETE CASCADE
);
