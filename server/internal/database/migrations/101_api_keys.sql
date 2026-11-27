-- Personal API keys for third-party clients (scripts, Grafana, Home Assistant…).
-- A key is "ssk_<prefix>_<secret>": only the public prefix (lookup + display)
-- and a SHA-256 hash of the whole key are stored, never the key itself.
-- A key acts as its owner with the owner's current role, narrowed by its
-- scope (read = GET/HEAD only); deleting the user deletes their keys.
CREATE TABLE IF NOT EXISTS api_keys (
    id           BIGSERIAL PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         VARCHAR(100) NOT NULL,
    prefix       VARCHAR(32) NOT NULL UNIQUE,
    key_hash     CHAR(64) NOT NULL,
    scope        VARCHAR(16) NOT NULL CHECK (scope IN ('read', 'write')),
    expires_at   TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys (user_id);

-- Audit entries written while serving a request authenticated by an API key
-- record that key. No foreign key: the trace outlives the key (a deleted user
-- takes their keys with them, the audit trail stays).
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS api_key_id BIGINT;
