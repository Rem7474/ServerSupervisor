-- Add memory_percent_avg to metrics_aggregates for correct chart display beyond 24h

ALTER TABLE metrics_aggregates
    ADD COLUMN IF NOT EXISTS memory_percent_avg DOUBLE PRECISION NOT NULL DEFAULT 0;
