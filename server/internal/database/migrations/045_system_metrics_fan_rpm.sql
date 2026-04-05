-- Add fan RPM metric (average RPM across detected fans when available).
ALTER TABLE system_metrics
ADD COLUMN IF NOT EXISTS fan_rpm DOUBLE PRECISION;

UPDATE system_metrics
SET fan_rpm = 0
WHERE fan_rpm IS NULL;

ALTER TABLE system_metrics
ALTER COLUMN fan_rpm SET DEFAULT 0;

ALTER TABLE system_metrics
ALTER COLUMN fan_rpm SET NOT NULL;
