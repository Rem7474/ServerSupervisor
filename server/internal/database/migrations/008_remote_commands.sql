-- Unified remote_commands table (replaces legacy docker_commands + apt_commands)

CREATE TABLE IF NOT EXISTS remote_commands (
    id           VARCHAR(36)  PRIMARY KEY,
    host_id      VARCHAR(64)  REFERENCES hosts(id) ON DELETE CASCADE,
    module       VARCHAR(50)  NOT NULL,
    action       VARCHAR(100) NOT NULL,
    target       VARCHAR(255) NOT NULL DEFAULT '',
    payload      TEXT         NOT NULL DEFAULT '{}',
    status       VARCHAR(20)  NOT NULL DEFAULT 'pending',
    output       TEXT         NOT NULL DEFAULT '',
    triggered_by VARCHAR(255) NOT NULL DEFAULT 'system',
    audit_log_id BIGINT,
    created_at   TIMESTAMPTZ  DEFAULT NOW(),
    started_at   TIMESTAMPTZ,
    ended_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_remote_commands_host_status ON remote_commands(host_id, status);

DROP TABLE IF EXISTS docker_commands;

DROP TABLE IF EXISTS apt_commands;
