-- Per-category audit log retention + export (ROADMAP.md item #13). action is
-- free text with no existing taxonomy (any CreateAuditLog caller can pass
-- anything) — category is a small, low-maintenance bucket computed once at
-- write time by models.CategorizeAuditAction, not a strict enum: alert
-- (alert_fired/resolved/escalated), auth (unblock_ip and future
-- MFA/session actions), settings (update_settings/cleanup_*), and command
-- (everything else — the large "an operation was dispatched or executed"
-- bucket: apt_*/docker_*/journalctl/webhook_trigger/agent_update/...).
ALTER TABLE audit_logs
    ADD COLUMN category character varying(20) NOT NULL DEFAULT 'command';

UPDATE audit_logs SET category = CASE
    WHEN action LIKE 'alert_%' THEN 'alert'
    WHEN action = 'unblock_ip' THEN 'auth'
    WHEN action IN ('update_settings', 'cleanup_metrics', 'cleanup_audit_logs') THEN 'settings'
    ELSE 'command'
END;

CREATE INDEX idx_audit_logs_category ON audit_logs(category, created_at);
