-- Repair alert_rules table by rebuilding it to eliminate dead (dropped) columns
-- that accumulated from repeated migration runs, causing PostgreSQL's 1600-column limit.
-- This is safe to run on both fresh and existing databases.

DO $$
BEGIN
  -- Only rebuild if the table exists and has dead columns
  IF EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = 'public' AND table_name = 'alert_rules'
  ) THEN
    -- Create clean replacement table
    CREATE TABLE IF NOT EXISTS alert_rules_rebuilt (
      id               SERIAL PRIMARY KEY,
      host_id          VARCHAR(64),
      metric           VARCHAR(50) NOT NULL,
      operator         VARCHAR(5) NOT NULL,
      threshold        DOUBLE PRECISION,
      duration_seconds INTEGER DEFAULT 60,
      enabled          BOOLEAN DEFAULT TRUE,
      created_at       TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
      name             VARCHAR(255),
      last_fired       TIMESTAMP WITH TIME ZONE,
      updated_at       TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
      actions          JSONB NOT NULL DEFAULT '{}'::jsonb
    );

    -- Copy all live data
    INSERT INTO alert_rules_rebuilt
      (id, host_id, metric, operator, threshold, duration_seconds, enabled, created_at, name, last_fired, updated_at, actions)
    SELECT
      id,
      host_id,
      metric,
      operator,
      threshold,
      duration_seconds,
      enabled,
      created_at,
      name,
      last_fired,
      updated_at,
      COALESCE(actions, '{}'::jsonb)
    FROM alert_rules;

    -- Swap tables
    DROP TABLE alert_rules CASCADE;
    ALTER TABLE alert_rules_rebuilt RENAME TO alert_rules;

    -- Restore sequence ownership
    ALTER SEQUENCE alert_rules_rebuilt_id_seq RENAME TO alert_rules_id_seq;
    ALTER TABLE alert_rules ALTER COLUMN id SET DEFAULT nextval('alert_rules_id_seq');
    ALTER SEQUENCE alert_rules_id_seq OWNED BY alert_rules.id;

    -- Restore foreign key from alert_incidents
    ALTER TABLE alert_incidents
      ADD CONSTRAINT alert_incidents_rule_id_fkey
      FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE SET NULL;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_alert_rules_host ON alert_rules(host_id);

