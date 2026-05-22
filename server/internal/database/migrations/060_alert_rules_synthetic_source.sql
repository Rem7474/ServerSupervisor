-- Allow 'synthetic' as a third source_type for alert rules, alongside agent and proxmox.
-- Used by uptime probes (uptime_down_count) and SSL certificate (ssl_min_days_remaining)
-- metrics which are global — they don't target a specific host or Proxmox scope.

ALTER TABLE alert_rules DROP CONSTRAINT IF EXISTS chk_alert_rules_source_type;
ALTER TABLE alert_rules
  ADD CONSTRAINT chk_alert_rules_source_type
  CHECK (source_type IN ('agent', 'proxmox', 'synthetic'));
