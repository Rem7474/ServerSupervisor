-- Introduce explicit alert source typing and first-class Proxmox scope.
-- This migration is additive and keeps backward compatibility with actions.proxmox_scope.

ALTER TABLE IF EXISTS alert_rules
  ADD COLUMN IF NOT EXISTS source_type VARCHAR(20) NOT NULL DEFAULT 'agent';

ALTER TABLE IF EXISTS alert_rules
  ADD COLUMN IF NOT EXISTS proxmox_scope JSONB;

UPDATE alert_rules
SET source_type = CASE
  WHEN metric LIKE 'proxmox_%' THEN 'proxmox'
  ELSE 'agent'
END
WHERE source_type IS NULL
   OR source_type NOT IN ('agent', 'proxmox');

UPDATE alert_rules
SET proxmox_scope = actions -> 'proxmox_scope'
WHERE metric LIKE 'proxmox_%'
  AND proxmox_scope IS NULL
  AND actions ? 'proxmox_scope';

ALTER TABLE IF EXISTS alert_rules
  DROP CONSTRAINT IF EXISTS chk_alert_rules_source_type;

ALTER TABLE IF EXISTS alert_rules
  ADD CONSTRAINT chk_alert_rules_source_type
  CHECK (source_type IN ('agent', 'proxmox'));

CREATE INDEX IF NOT EXISTS idx_alert_rules_source_type ON alert_rules(source_type);
