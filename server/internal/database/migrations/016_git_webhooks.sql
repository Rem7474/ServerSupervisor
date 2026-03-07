-- Git Webhooks: trigger custom tasks on VMs from GitHub/GitLab/Gitea/Forgejo push events
CREATE TABLE IF NOT EXISTS git_webhooks (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              TEXT NOT NULL,
    secret            TEXT NOT NULL,
    provider          TEXT NOT NULL DEFAULT 'github',
    repo_filter       TEXT NOT NULL DEFAULT '',
    branch_filter     TEXT NOT NULL DEFAULT '',
    event_filter      TEXT NOT NULL DEFAULT 'push',
    host_id           VARCHAR(64) NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    custom_task_id    TEXT NOT NULL,
    notify_channels   TEXT[] NOT NULL DEFAULT '{}',
    notify_on_success BOOLEAN NOT NULL DEFAULT FALSE,
    notify_on_failure BOOLEAN NOT NULL DEFAULT TRUE,
    enabled           BOOLEAN NOT NULL DEFAULT TRUE,
    last_triggered_at TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_git_webhooks_host ON git_webhooks(host_id);
CREATE INDEX IF NOT EXISTS idx_git_webhooks_enabled ON git_webhooks(enabled) WHERE enabled = TRUE;

CREATE TABLE IF NOT EXISTS git_webhook_executions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id     UUID NOT NULL REFERENCES git_webhooks(id) ON DELETE CASCADE,
    command_id     VARCHAR(36) REFERENCES remote_commands(id) ON DELETE SET NULL,
    provider       TEXT NOT NULL DEFAULT '',
    repo_name      TEXT NOT NULL DEFAULT '',
    branch         TEXT NOT NULL DEFAULT '',
    commit_sha     TEXT NOT NULL DEFAULT '',
    commit_message TEXT NOT NULL DEFAULT '',
    pusher         TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'pending',
    triggered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at   TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_git_webhook_executions_webhook ON git_webhook_executions(webhook_id);
CREATE INDEX IF NOT EXISTS idx_git_webhook_executions_command ON git_webhook_executions(command_id);
