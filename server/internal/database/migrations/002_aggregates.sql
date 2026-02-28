-- Metrics aggregates table for downsampling (5min, hourly, daily)

CREATE TABLE IF NOT EXISTS metrics_aggregates (
    id BIGSERIAL PRIMARY KEY,
    host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
    aggregation_type VARCHAR(20) NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    cpu_usage_avg DOUBLE PRECISION,
    cpu_usage_max DOUBLE PRECISION,
    memory_usage_avg BIGINT,
    memory_usage_max BIGINT,
    disk_usage_avg DOUBLE PRECISION,
    network_rx_bytes BIGINT,
    network_tx_bytes BIGINT,
    sample_count INTEGER DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_metrics_aggregates_host_time ON metrics_aggregates(host_id, aggregation_type, timestamp DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_metrics_aggregates_unique ON metrics_aggregates(host_id, aggregation_type, timestamp);
