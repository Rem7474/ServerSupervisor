-- Migration 002: Add compose_projects table
CREATE TABLE IF NOT EXISTS compose_projects (
    id          TEXT     PRIMARY KEY,
    host_id     TEXT     NOT NULL,
    name        TEXT     NOT NULL,
    working_dir TEXT     NOT NULL DEFAULT '',
    config_file TEXT     NOT NULL DEFAULT '',
    services    TEXT     NOT NULL DEFAULT '[]',
    raw_config  TEXT     NOT NULL DEFAULT '',
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_compose_projects_host_id ON compose_projects(host_id);
