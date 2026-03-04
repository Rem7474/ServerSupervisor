-- Migration 014: Scheduled Tasks
-- Stores per-host cron-scheduled remote commands (apt, docker, systemd, custom, …)

CREATE TABLE IF NOT EXISTS scheduled_tasks (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    host_id         VARCHAR(64) NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    name            TEXT        NOT NULL,
    module          TEXT        NOT NULL,
    action          TEXT        NOT NULL,
    target          TEXT        NOT NULL DEFAULT '',
    payload         JSONB       NOT NULL DEFAULT '{}',
    cron_expression TEXT        NOT NULL,
    enabled         BOOLEAN     NOT NULL DEFAULT TRUE,
    last_run_at     TIMESTAMPTZ,
    next_run_at     TIMESTAMPTZ,
    last_run_status TEXT,
    created_by      TEXT        NOT NULL DEFAULT 'system',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_host    ON scheduled_tasks(host_id);
CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_enabled ON scheduled_tasks(enabled) WHERE enabled = TRUE;
