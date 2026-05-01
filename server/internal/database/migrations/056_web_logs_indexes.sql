CREATE INDEX IF NOT EXISTS idx_wlr_host_suspicious_at
    ON web_log_requests (host_id, suspicious, captured_at DESC);

CREATE INDEX IF NOT EXISTS idx_wlr_host_blocked_at
    ON web_log_requests (host_id, blocked, captured_at DESC);
