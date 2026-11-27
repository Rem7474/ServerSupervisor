-- Constrain alert_rules.target_type to the values the alert engine handles:
--   ''    : Docker, Proxmox and synthetic rules, which carry their own scope
--   host  : the single host in host_id
--   all   : every host
--   group : every current member of host_group_id (which must be set)
--   tag   : every host carrying at least one of target_tags
--
-- NOT VALID then VALIDATE CONSTRAINT checks existing rows without rewriting
-- the table (same pattern as chk_alert_rules_source_type).
ALTER TABLE alert_rules
    DROP CONSTRAINT IF EXISTS chk_alert_rules_target_type;

ALTER TABLE alert_rules
    ADD CONSTRAINT chk_alert_rules_target_type
    CHECK (
        target_type IN ('', 'host', 'all', 'group', 'tag')
        AND (target_type <> 'group' OR host_group_id IS NOT NULL)
    )
    NOT VALID;

ALTER TABLE alert_rules
    VALIDATE CONSTRAINT chk_alert_rules_target_type;
