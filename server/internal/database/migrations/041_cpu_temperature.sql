-- Add CPU temperature (Celsius) collected by the Linux agent when available.
ALTER TABLE system_metrics
ADD COLUMN IF NOT EXISTS cpu_temperature DOUBLE PRECISION;
