-- Supports the host-exposure correlation feature (GetHostExposure): looking
-- up aggregated web-log traffic for a set of NPM domain names is a
-- domain-first, time-bounded query with no existing matching index (the
-- current indexes are all host_id/source/ip-first).
CREATE INDEX idx_web_log_requests_domain_captured ON web_log_requests (domain, captured_at DESC);
