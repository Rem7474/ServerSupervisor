-- Per-host access control: allows admins to restrict or grant users
-- specific access to individual hosts. If a user has NO entries in this
-- table their global role applies to all hosts.
-- If they have ANY entries, they are restricted to only those hosts.
CREATE TABLE IF NOT EXISTS host_permissions (
    username   TEXT NOT NULL,
    host_id    TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    level      TEXT NOT NULL DEFAULT 'viewer'
                   CHECK (level IN ('viewer', 'operator')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (username, host_id)
);

CREATE INDEX IF NOT EXISTS idx_host_permissions_username ON host_permissions(username);
CREATE INDEX IF NOT EXISTS idx_host_permissions_host_id  ON host_permissions(host_id);

