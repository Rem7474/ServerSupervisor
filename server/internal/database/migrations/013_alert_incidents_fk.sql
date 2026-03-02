-- Change alert_incidents.rule_id FK from ON DELETE CASCADE to ON DELETE SET NULL.
-- This preserves historical incidents when an alert rule is deleted instead of
-- silently purging them (which caused {notifications: [], total: 0} in the UI).
ALTER TABLE alert_incidents DROP CONSTRAINT IF EXISTS alert_incidents_rule_id_fkey;
ALTER TABLE alert_incidents
    ADD CONSTRAINT alert_incidents_rule_id_fkey
    FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE SET NULL;
