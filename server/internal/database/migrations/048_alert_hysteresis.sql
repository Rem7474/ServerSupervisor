-- Add hysteresis support to alert rules with threshold_clear column.
-- Allows defining separate thresholds for alert activation and resolution.
-- When threshold_clear is NULL, it defaults to using threshold (backward compatible).

ALTER TABLE alert_rules_rebuilt
  ADD COLUMN threshold_clear DOUBLE PRECISION;

COMMENT ON COLUMN alert_rules_rebuilt.threshold_clear IS 'Optional threshold for resolving alerts. When set, incidents resolve at this value instead of when the activation condition becomes false.';
