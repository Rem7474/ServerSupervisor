-- Speed up the global dashboard metrics summary, which filters/groups on
-- `timestamp` alone (no host_id). The existing idx_system_metrics_host_time
-- has host_id as its leading column, so it can't serve a timestamp-only range
-- scan efficiently on plain PostgreSQL (TimescaleDB relies on chunk exclusion
-- instead, but the index is harmless there).
--
-- CONCURRENTLY avoids locking the (potentially large) table for writes while the
-- index builds. Migrations here are not wrapped in a transaction, so this is safe.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_system_metrics_timestamp
    ON system_metrics(timestamp);
