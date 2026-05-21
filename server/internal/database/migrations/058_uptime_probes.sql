-- Uptime / synthetic monitoring: HTTP(S) and TCP probes executed from the server.
-- Each probe has its own interval, and the results history feeds uptime % and latency charts.

CREATE TABLE IF NOT EXISTS uptime_probes (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                TEXT NOT NULL,
    type                TEXT NOT NULL CHECK (type IN ('http', 'tcp')),
    target              TEXT NOT NULL,                -- URL for http, host:port for tcp
    interval_sec        INTEGER NOT NULL DEFAULT 60,
    timeout_sec         INTEGER NOT NULL DEFAULT 10,
    expected_status     INTEGER NOT NULL DEFAULT 200, -- HTTP probes only
    expected_body_regex TEXT NOT NULL DEFAULT '',     -- HTTP probes only; empty = skip body check
    follow_redirects    BOOLEAN NOT NULL DEFAULT TRUE,
    verify_tls          BOOLEAN NOT NULL DEFAULT TRUE,
    enabled             BOOLEAN NOT NULL DEFAULT TRUE,
    -- Cached current state for fast listing without scanning history
    last_status         TEXT NOT NULL DEFAULT 'unknown', -- up | down | unknown
    last_latency_ms     INTEGER,
    last_status_code    INTEGER,
    last_error          TEXT NOT NULL DEFAULT '',
    last_checked_at     TIMESTAMPTZ,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_uptime_probes_enabled ON uptime_probes(enabled) WHERE enabled = TRUE;

CREATE TABLE IF NOT EXISTS uptime_probe_results (
    id          BIGSERIAL PRIMARY KEY,
    probe_id    UUID NOT NULL REFERENCES uptime_probes(id) ON DELETE CASCADE,
    checked_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    success     BOOLEAN NOT NULL,
    status_code INTEGER,                    -- HTTP only
    latency_ms  INTEGER NOT NULL DEFAULT 0,
    error       TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_uptime_probe_results_probe_time
    ON uptime_probe_results(probe_id, checked_at DESC);
