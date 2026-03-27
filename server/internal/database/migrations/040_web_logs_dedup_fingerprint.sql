-- Migration 040: Add dedup fingerprint and unique key for web log requests
-- Goal: make ingestion idempotent across agent restarts and aggressive log rotation.

ALTER TABLE web_log_requests
ADD COLUMN IF NOT EXISTS fingerprint TEXT;

UPDATE web_log_requests
SET fingerprint = md5(CONCAT_WS('|',
  host_id::text,
  source,
  captured_at::text,
  ip,
  method,
  path,
  status::text,
  bytes::text,
  COALESCE(user_agent, ''),
  COALESCE(domain, ''),
  COALESCE(category, ''),
  suspicious::text
))
WHERE fingerprint IS NULL;

WITH ranked AS (
  SELECT
    id,
    ROW_NUMBER() OVER (
      PARTITION BY host_id, source, fingerprint
      ORDER BY id
    ) AS rn
  FROM web_log_requests
)
DELETE FROM web_log_requests w
USING ranked r
WHERE w.id = r.id
  AND r.rn > 1;

ALTER TABLE web_log_requests
ALTER COLUMN fingerprint SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_web_log_requests_host_source_fingerprint
ON web_log_requests (host_id, source, fingerprint);
