-- SSL/TLS certificate monitoring: track expiration of remote TLS endpoints.
-- Checked daily by the server; alerts fire at J-30 and J-7 via the alert engine.

CREATE TABLE IF NOT EXISTS ssl_certificates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    host            TEXT NOT NULL,
    port            INTEGER NOT NULL DEFAULT 443,
    server_name     TEXT NOT NULL DEFAULT '', -- SNI override; empty means use host
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    last_checked_at TIMESTAMPTZ,
    valid_from      TIMESTAMPTZ,
    valid_to        TIMESTAMPTZ,
    issuer          TEXT NOT NULL DEFAULT '',
    subject         TEXT NOT NULL DEFAULT '',
    serial_number   TEXT NOT NULL DEFAULT '',
    dns_names       TEXT[] NOT NULL DEFAULT '{}',
    last_error      TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (host, port, server_name)
);

CREATE INDEX IF NOT EXISTS idx_ssl_certificates_enabled
    ON ssl_certificates(enabled) WHERE enabled = TRUE;
CREATE INDEX IF NOT EXISTS idx_ssl_certificates_valid_to
    ON ssl_certificates(valid_to) WHERE valid_to IS NOT NULL;
