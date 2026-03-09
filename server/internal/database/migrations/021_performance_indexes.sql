-- Performance indexes for common query patterns.
-- All statements are idempotent (IF NOT EXISTS).

-- host_metrics: primary time-series query (latest metrics per host, history view)
CREATE INDEX IF NOT EXISTS idx_metrics_host_time
    ON host_metrics(host_id, timestamp DESC);

-- remote_commands: pending-command lookup + history filtered by host/status
CREATE INDEX IF NOT EXISTS idx_commands_host_status
    ON remote_commands(host_id, status, created_at DESC);

-- audit_logs: time-ordered listing for dashboard and audit page
CREATE INDEX IF NOT EXISTS idx_audit_timestamp
    ON audit_logs(timestamp DESC);

-- git_webhook_executions: latest executions per webhook
CREATE INDEX IF NOT EXISTS idx_webhook_exec_webhook_time
    ON git_webhook_executions(webhook_id, triggered_at DESC);

-- release_tracker_executions: latest executions per tracker
CREATE INDEX IF NOT EXISTS idx_tracker_exec_tracker_time
    ON release_tracker_executions(tracker_id, triggered_at DESC);
