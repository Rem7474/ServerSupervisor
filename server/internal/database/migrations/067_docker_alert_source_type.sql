-- Allow source_type = 'docker' in alert_rules.
--
-- NOT VALID skips the full-table scan on ADD, avoiding the internal table
-- rewrite that triggers a "duplicate key on alert_rules_rebuilt_pkey" error
-- when this migration is retried after a previous failed attempt.
-- VALIDATE CONSTRAINT then checks existing rows without rewriting the table.

-- Clean up any leftover rebuild artifact from a previous failed run.
DROP TABLE IF EXISTS alert_rules_rebuilt;

ALTER TABLE alert_rules
    DROP CONSTRAINT IF EXISTS chk_alert_rules_source_type;

ALTER TABLE alert_rules
    ADD CONSTRAINT chk_alert_rules_source_type
    CHECK (source_type IN ('agent', 'proxmox', 'synthetic', 'docker'))
    NOT VALID;

ALTER TABLE alert_rules
    VALIDATE CONSTRAINT chk_alert_rules_source_type;
