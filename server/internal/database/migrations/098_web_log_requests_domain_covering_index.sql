-- Make the host-exposure traffic aggregate an index-only scan.
--
-- getWebLogStatsByDomains (db_host_exposure.go) powers GET /hosts/:id/exposure:
--
--   SELECT domain, COUNT(*), SUM(bytes),
--          SUM(CASE WHEN status BETWEEN 400 AND 499 ...),
--          SUM(CASE WHEN status >= 500 ...),
--          SUM(CASE WHEN suspicious ...), SUM(CASE WHEN blocked ...)
--   FROM web_log_requests
--   WHERE domain = ANY($1) AND captured_at >= $2
--   GROUP BY domain
--
-- Migration 076 gave it (domain, captured_at DESC), so locating the rows is
-- already indexed — but every one of the four aggregated columns lives in the
-- heap, so the plan pays one random heap fetch per matching row. On a busy
-- install that is what dominates: a production capture measured this endpoint
-- at 14.5 s for the default 24 h window.
--
-- Adding those four columns as INCLUDE payload lets the same lookup finish as
-- an index-only scan, with no heap access at all (subject to the visibility
-- map being current, i.e. autovacuum keeping up — which is the normal state
-- for an append-mostly table like this one).
--
-- The replacement carries the same key columns as 076's index, so every query
-- that could use the old one can use this one; dropping it avoids maintaining
-- two indexes with identical keys on the single most-inserted table.
--
-- This takes a write lock on web_log_requests for the duration of the build,
-- same as migrations 076 and 077 did. It is not CONCURRENTLY: the runner
-- applies statements one by one outside a transaction so CONCURRENTLY would
-- work, but a failed concurrent build leaves an INVALID index behind that a
-- later IF NOT EXISTS silently accepts — a startup stall is easier to reason
-- about than an index that exists and is never used.

DROP INDEX IF EXISTS idx_web_log_requests_domain_captured;

CREATE INDEX IF NOT EXISTS idx_web_log_requests_domain_captured
    ON web_log_requests (domain, captured_at DESC)
    INCLUDE (bytes, status, suspicious, blocked);
