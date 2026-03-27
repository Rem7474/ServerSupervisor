-- Migration 039: Replace bot_detection/npm_analytics JSONB with relational web logs tables
-- Created: 2026-03-27

ALTER TABLE hosts
ADD COLUMN IF NOT EXISTS web_log_source TEXT,
ADD COLUMN IF NOT EXISTS web_log_collected_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS web_log_total_requests INTEGER NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS web_log_total_bytes BIGINT NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS web_log_errors_4xx INTEGER NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS web_log_errors_5xx INTEGER NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS web_log_suspicious_requests INTEGER NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS web_log_suspicious_ips INTEGER NOT NULL DEFAULT 0;

DROP INDEX IF EXISTS idx_hosts_bot_detection;
DROP INDEX IF EXISTS idx_hosts_npm_analytics;

ALTER TABLE hosts DROP COLUMN IF EXISTS bot_detection;
ALTER TABLE hosts DROP COLUMN IF EXISTS npm_analytics;

CREATE TABLE IF NOT EXISTS web_log_snapshots (
  id                  BIGSERIAL PRIMARY KEY,
  host_id             UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
  captured_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  source              TEXT NOT NULL,
  total_requests      INTEGER NOT NULL DEFAULT 0,
  total_bytes         BIGINT NOT NULL DEFAULT 0,
  errors_4xx          INTEGER NOT NULL DEFAULT 0,
  errors_5xx          INTEGER NOT NULL DEFAULT 0,
  suspicious_requests INTEGER NOT NULL DEFAULT 0,
  suspicious_ips      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_web_log_snapshots_host_captured
ON web_log_snapshots (host_id, captured_at DESC);

CREATE INDEX IF NOT EXISTS idx_web_log_snapshots_host_source_captured
ON web_log_snapshots (host_id, source, captured_at DESC);

CREATE TABLE IF NOT EXISTS web_log_requests (
  id          BIGSERIAL PRIMARY KEY,
  snapshot_id BIGINT NOT NULL REFERENCES web_log_snapshots(id) ON DELETE CASCADE,
  host_id     UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
  captured_at TIMESTAMPTZ NOT NULL,
  source      TEXT NOT NULL,
  ip          TEXT NOT NULL,
  method      TEXT NOT NULL,
  path        TEXT NOT NULL,
  status      INTEGER NOT NULL,
  bytes       BIGINT NOT NULL DEFAULT 0,
  user_agent  TEXT,
  domain      TEXT,
  category    TEXT,
  suspicious  BOOLEAN NOT NULL DEFAULT FALSE,
  fingerprint TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_web_log_requests_host_captured
ON web_log_requests (host_id, captured_at DESC);

CREATE INDEX IF NOT EXISTS idx_web_log_requests_ip_captured
ON web_log_requests (ip, captured_at DESC);

CREATE INDEX IF NOT EXISTS idx_web_log_requests_source_captured
ON web_log_requests (source, captured_at DESC);

CREATE INDEX IF NOT EXISTS idx_web_log_requests_suspicious_captured
ON web_log_requests (suspicious, captured_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS ux_web_log_requests_host_source_fingerprint
ON web_log_requests (host_id, source, fingerprint);
