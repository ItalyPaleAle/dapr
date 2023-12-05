-- This file contains queries that are executed while setting up the environment for conformance tests
-- Queries should be idempotent here

-- Sources:
-- - https://gist.github.com/thehesiod/d0314d599c721216f075375c667e2d9a
-- - https://dba.stackexchange.com/questions/69988/how-can-i-fake-inet-client-addr-for-unit-tests-in-postgresql/70009#70009

-- This allows overriding the "now()" function to be able to mock time

CREATE TABLE IF NOT EXISTS conftests.freeze_time (
    key text NOT NULL PRIMARY KEY,
    value jsonb
);

INSERT INTO conftests.freeze_time VALUES
    ('enabled', 'false'),
    ('timestamp', 'null'),
    ('tick', 'false')
ON CONFLICT (key) DO NOTHING;

CREATE OR REPLACE FUNCTION conftests.freeze_time(freeze_time timestamp with time zone, tick bool DEFAULT false)
    RETURNS void AS
$func$
BEGIN
  INSERT INTO conftests.freeze_time
  VALUES
    ('enabled', 'true'),
    ('timestamp',  EXTRACT(EPOCH from freeze_time)::text::jsonb),
    ('tick', tick::text::jsonb)
  ON CONFLICT (key) DO UPDATE SET
      value = excluded.value;
END
$func$ LANGUAGE plpgsql;

CREATE OR REPLACE function conftests.unfreeze_time()
    RETURNS void AS
$func$
BEGIN
  INSERT INTO conftests.freeze_time
  VALUES ('enabled', 'false')
  ON CONFLICT (key) DO UPDATE SET
    value = excluded.value;
END
$func$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION conftests.now()
  RETURNS timestamptz AS
$func$
DECLARE
  enabled text;
  tick text;
  timestamp timestamp;
BEGIN
    SELECT
      INTO enabled
      VALUE FROM conftests.freeze_time
      WHERE key = 'enabled';
    SELECT
      INTO tick
      VALUE FROM conftests.freeze_time
      WHERE key = 'tick';

    IF enabled THEN
      SELECT
        INTO timestamp to_timestamp(value::text::decimal)
        FROM conftests.freeze_time
        WHERE key = 'timestamp';

      IF tick THEN
          timestamp = timestamp + '1 second'::interval;
          UPDATE conftests.freeze_time
            SET value = extract(epoch from timestamp)::text::jsonb
            WHERE key = 'timestamp';
      END IF;

      RETURN timestamp;
    ELSE
      RETURN pg_catalog.now();
    END IF;
END
$func$ LANGUAGE plpgsql;
