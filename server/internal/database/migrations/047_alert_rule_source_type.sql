-- Rebuild alert_rules with explicit source_type and first-class Proxmox scope.
-- This avoids ALTER TABLE on oversized tables that may already be near PostgreSQL's
-- column limit because of historical dead-column accumulation.

DROP TABLE IF EXISTS alert_rules_rebuilt CASCADE;

CREATE TABLE alert_rules_rebuilt (
  id SERIAL PRIMARY KEY,
  name VARCHAR(255),
  source_type VARCHAR(20) NOT NULL DEFAULT 'agent',
  host_id VARCHAR(64),
  proxmox_scope JSONB,
  metric VARCHAR(50) NOT NULL,
  operator VARCHAR(5) NOT NULL,
  threshold DOUBLE PRECISION,
  duration_seconds INTEGER DEFAULT 60,
  actions JSONB NOT NULL DEFAULT '{}'::jsonb,
  last_fired TIMESTAMP WITH TIME ZONE,
  enabled BOOLEAN DEFAULT TRUE,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  CONSTRAINT chk_alert_rules_source_type CHECK (source_type IN ('agent', 'proxmox'))
);

INSERT INTO alert_rules_rebuilt (
  id,
  name,
  source_type,
  host_id,
  proxmox_scope,
  metric,
  operator,
  threshold,
  duration_seconds,
  actions,
  last_fired,
  enabled,
  created_at,
  updated_at
)
SELECT
  id,
  name,
  CASE WHEN metric LIKE 'proxmox_%' THEN 'proxmox' ELSE 'agent' END,
  host_id,
  CASE WHEN metric LIKE 'proxmox_%' THEN actions -> 'proxmox_scope' ELSE NULL END,
  metric,
  operator,
  threshold,
  duration_seconds,
  COALESCE(actions, '{}'::jsonb),
  last_fired,
  enabled,
  created_at,
  COALESCE(updated_at, created_at, NOW())
FROM alert_rules;

DROP TABLE alert_rules CASCADE;
ALTER TABLE alert_rules_rebuilt RENAME TO alert_rules;

ALTER SEQUENCE alert_rules_rebuilt_id_seq RENAME TO alert_rules_id_seq;
ALTER TABLE alert_rules ALTER COLUMN id SET DEFAULT nextval('alert_rules_id_seq');
ALTER SEQUENCE alert_rules_id_seq OWNED BY alert_rules.id;

ALTER TABLE alert_incidents
  DROP CONSTRAINT IF EXISTS alert_incidents_rule_id_fkey;

ALTER TABLE alert_incidents
  ADD CONSTRAINT alert_incidents_rule_id_fkey
  FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_alert_rules_host ON alert_rules(host_id);
CREATE INDEX IF NOT EXISTS idx_alert_rules_source_type ON alert_rules(source_type);
