-- Query for performing migration #1
-- This creates the tables for actor state
--
-- fmt.Sprintf arguments:
-- 1. Name of the "hosts" table
-- 2. Name of the "hosts_actor_types" table
-- 3. Name of the "actors" table
-- 4. Name of the "metadata" table
-- 5. Prefix for the tables/functions

CREATE TABLE %[1]s (
  host_id uuid PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  host_address text NOT NULL,
  host_app_id text NOT NULL,
  host_actors_api_level integer NOT NULL,
  host_last_reported_api_level integer NOT NULL DEFAULT 0,
  host_last_healthcheck timestamp with time zone NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ON %[1]s (host_address);
CREATE INDEX ON %[1]s (host_last_healthcheck);
CREATE INDEX ON %[1]s (host_actors_api_level);
CREATE INDEX ON %[1]s (host_last_reported_api_level);

CREATE TABLE %[2]s (
  host_id uuid NOT NULL,
  actor_type text NOT NULL,
  actor_idle_timeout integer NOT NULL,
  actor_concurrent_reminders integer NOT NULL DEFAULT 0,
  PRIMARY KEY (host_id, actor_type),
  FOREIGN KEY (host_id) REFERENCES %[1]s (host_id) ON DELETE CASCADE
);

CREATE INDEX ON %[2]s (actor_type);

CREATE TABLE %[3]s (
  actor_type text NOT NULL,
  actor_id text NOT NULL,
  host_id uuid NOT NULL,
  actor_idle_timeout integer NOT NULL,
  actor_activation timestamp with time zone NOT NULL DEFAULT now(),
  PRIMARY KEY (actor_type, actor_id),
  FOREIGN KEY (host_id) REFERENCES %[1]s (host_id) ON DELETE CASCADE
);

-- This procedure updates the key 'actors-api-level' in the metadata table with the minimum API level from the hosts table
-- It only updates it if the new level is higher than the previous
CREATE FUNCTION %[5]supdate_metadata_min_api_level()
RETURNS trigger
AS $func$
BEGIN
  INSERT INTO %[4]s ("key", "value")
    SELECT 'actors-api-level', COALESCE(MIN(host_actors_api_level), 0)
    FROM %[1]s
  ON CONFLICT ("key") DO UPDATE
    SET value = EXCLUDED.value
    WHERE EXCLUDED.value::integer > COALESCE(%[4]s.value, '0')::integer;
  RETURN NULL;
END;
$func$ LANGUAGE plpgsql;

-- Create triggers to invoke the update_metadata_min_api_level procedure when the hosts table is updated
CREATE TRIGGER %[5]supdate_metadata_min_api_level_upsert_trigger
  AFTER INSERT OR UPDATE OF host_actors_api_level
  ON %[1]s
  FOR EACH STATEMENT
  EXECUTE FUNCTION %[5]supdate_metadata_min_api_level();
CREATE TRIGGER %[5]supdate_metadata_min_api_level_delete_trigger
  AFTER DELETE
  ON %[1]s
  FOR EACH STATEMENT
  EXECUTE FUNCTION %[5]supdate_metadata_min_api_level();

-- This function enforces that a host meets the minimum API level
CREATE FUNCTION %[5]senforce_min_api_level()
RETURNS trigger
AS $func$
DECLARE
  current_version INTEGER;
BEGIN
  SELECT COALESCE("value", '0')::integer INTO current_version FROM %[4]s WHERE "key" = 'actors-api-level';
  IF NEW.host_actors_api_level < current_version THEN
    RAISE EXCEPTION 'Value of column host_actors_api_level is lower than the current API level in the cluster';
  END IF;
  RETURN NEW;
END
$func$ LANGUAGE plpgsql;

-- Create a trigger to invoke the enforce_min_api_level function before a row is added to the hosts table
CREATE TRIGGER %[5]senforce_min_api_level_trigger
  BEFORE INSERT OR UPDATE OF host_actors_api_level
  ON %[1]s
  FOR EACH ROW
  EXECUTE FUNCTION %[5]senforce_min_api_level();
