-- Release Trackers: poll external repos for new releases and trigger custom tasks on VMs
CREATE TABLE IF NOT EXISTS release_trackers (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              TEXT NOT NULL,
    provider          TEXT NOT NULL DEFAULT 'github',
    repo_owner        TEXT NOT NULL,
    repo_name         TEXT NOT NULL,
    host_id           VARCHAR(64) NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    custom_task_id    TEXT NOT NULL,
    last_release_tag  TEXT NOT NULL DEFAULT '',
    last_checked_at   TIMESTAMPTZ,
    last_triggered_at TIMESTAMPTZ,
    notify_channels   TEXT[] NOT NULL DEFAULT '{}',
    notify_on_release BOOLEAN NOT NULL DEFAULT TRUE,
    enabled           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_release_trackers_host    ON release_trackers(host_id);
CREATE INDEX IF NOT EXISTS idx_release_trackers_enabled ON release_trackers(enabled) WHERE enabled = TRUE;

CREATE TABLE IF NOT EXISTS release_tracker_executions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tracker_id   UUID NOT NULL REFERENCES release_trackers(id) ON DELETE CASCADE,
    command_id   VARCHAR(36) REFERENCES remote_commands(id) ON DELETE SET NULL,
    tag_name     TEXT NOT NULL DEFAULT '',
    release_url  TEXT NOT NULL DEFAULT '',
    release_name TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'pending',
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_release_tracker_executions_tracker ON release_tracker_executions(tracker_id);
CREATE INDEX IF NOT EXISTS idx_release_tracker_executions_command ON release_tracker_executions(command_id);
