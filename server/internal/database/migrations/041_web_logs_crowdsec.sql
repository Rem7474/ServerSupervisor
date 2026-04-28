-- Migration 041: Add CrowdSec correlation columns to web_log_requests
-- Created: 2026-04-28

ALTER TABLE web_log_requests
ADD COLUMN IF NOT EXISTS blocked BOOLEAN NOT NULL DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS blocked_source TEXT,
ADD COLUMN IF NOT EXISTS blocked_reason TEXT,
ADD COLUMN IF NOT EXISTS blocked_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS blocked_until TIMESTAMPTZ;

-- Index for filtering blocked IPs efficiently
CREATE INDEX IF NOT EXISTS idx_web_log_requests_blocked_captured
ON web_log_requests (blocked, captured_at DESC);

-- Index for querying by blocked status and IP
CREATE INDEX IF NOT EXISTS idx_web_log_requests_ip_blocked
ON web_log_requests (ip, blocked, captured_at DESC);
