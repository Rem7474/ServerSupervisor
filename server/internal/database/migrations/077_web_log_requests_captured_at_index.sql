-- The Traffic/Threats dashboards default to the "all hosts" view (no host_id
-- filter), and every existing index on these two tables leads with host_id,
-- source, ip, blocked or suspicious — none can serve a plain
-- `captured_at >= $1` predicate. That forces a full sequential scan for each
-- of the ~15 aggregate queries GetWebLogsSummary fires per page load, which
-- is what pushed the dashboard past its 30s request timeout on a
-- large/long-running table. A plain time-descending index lets Postgres use
-- an index/bitmap scan for the time-bound instead.
CREATE INDEX idx_web_log_requests_captured_at ON web_log_requests (captured_at DESC);
CREATE INDEX idx_web_log_snapshots_captured_at ON web_log_snapshots (captured_at DESC);
