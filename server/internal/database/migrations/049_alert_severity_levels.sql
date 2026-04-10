-- Refactor alert thresholds to support two severity levels: warn and crit.
-- This migration replaces the single threshold with warn/crit pair.
-- Backward compatibility: existing threshold values become threshold_crit, with threshold_warn = 80% of crit.

ALTER TABLE alert_rules_rebuilt RENAME COLUMN threshold TO threshold_crit;
ALTER TABLE alert_rules_rebuilt RENAME COLUMN threshold_clear TO threshold_clear_crit;

ALTER TABLE alert_rules_rebuilt
  ADD COLUMN threshold_warn DOUBLE PRECISION,
  ADD COLUMN threshold_clear_warn DOUBLE PRECISION;

-- For existing rules, set warn threshold to 80% of crit (or adjust as needed)
-- For metrics where higher is worse: warn = crit * 0.8
UPDATE alert_rules_rebuilt
SET threshold_warn = CASE
    -- For metrics where higher is worse (>, >=), warn is 80% of crit
    WHEN operator IN ('>', '>=') THEN threshold_crit * 0.8
    -- For metrics where lower is worse (<, <=), warn is 120% of crit
    WHEN operator IN ('<', '<=') THEN threshold_crit * 1.2
    ELSE threshold_crit * 0.8
  END
WHERE threshold_warn IS NULL AND threshold_crit IS NOT NULL;

COMMENT ON COLUMN alert_rules_rebuilt.threshold_warn IS 'Warning level threshold. Incidents triggered when threshold_warn is exceeded.';
COMMENT ON COLUMN alert_rules_rebuilt.threshold_crit IS 'Critical level threshold. Incidents triggered when threshold_crit is exceeded.';
COMMENT ON COLUMN alert_rules_rebuilt.threshold_clear_warn IS 'Hysteresis threshold for resolving warning incidents.';
COMMENT ON COLUMN alert_rules_rebuilt.threshold_clear_crit IS 'Hysteresis threshold for resolving critical incidents.';

-- Add severity column to alert_incidents to track warn vs crit
ALTER TABLE alert_incidents
  ADD COLUMN severity VARCHAR(10) DEFAULT 'crit' CHECK (severity IN ('warn', 'crit'));

COMMENT ON COLUMN alert_incidents.severity IS 'Severity level of the incident (warn or crit)';
