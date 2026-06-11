-- Allow source_type = 'docker' in alert_rules
ALTER TABLE alert_rules
    DROP CONSTRAINT IF EXISTS chk_alert_rules_source_type;

ALTER TABLE alert_rules
    ADD CONSTRAINT chk_alert_rules_source_type
        CHECK (source_type IN ('agent', 'proxmox', 'synthetic', 'docker'));
