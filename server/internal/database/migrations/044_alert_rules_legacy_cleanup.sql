-- Normalize historical alert rule metrics/operators and disable NPM metrics no longer used.
-- Safe idempotent cleanup after migration to canonical alert vocabulary.

-- 1) Normalize historical metric aliases to canonical names.
UPDATE alert_rules
SET metric = CASE metric
  WHEN 'cpu_percent' THEN 'cpu'
  WHEN 'ram_percent' THEN 'memory'
  WHEN 'disk_percent' THEN 'disk'
  ELSE metric
END,
updated_at = NOW()
WHERE metric IN ('cpu_percent', 'ram_percent', 'disk_percent');

-- 2) Normalize historical textual operators to canonical symbols.
UPDATE alert_rules
SET operator = CASE operator
  WHEN 'gt' THEN '>'
  WHEN 'lt' THEN '<'
  WHEN 'gte' THEN '>='
  WHEN 'lte' THEN '<='
  ELSE operator
END,
updated_at = NOW()
WHERE operator IN ('gt', 'lt', 'gte', 'lte');

-- 3) Disable NPM metric-based rules no longer supported.
UPDATE alert_rules
SET enabled = FALSE,
    updated_at = NOW()
WHERE metric IN ('npm_requests', 'npm_traffic_bytes', 'npm_5xx_errors')
  AND enabled = TRUE;

