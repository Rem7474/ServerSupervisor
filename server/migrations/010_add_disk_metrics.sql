-- Migration: Add disk metrics tables
-- Created: 2026-02-23

CREATE TABLE IF NOT EXISTS disk_metrics (
    id BIGSERIAL PRIMARY KEY,
    host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
    mount_point TEXT NOT NULL,
    filesystem TEXT NOT NULL,
    size_gb DOUBLE PRECISION NOT NULL,
    used_gb DOUBLE PRECISION NOT NULL,
    avail_gb DOUBLE PRECISION NOT NULL,
    used_percent DOUBLE PRECISION NOT NULL,
    inodes_total BIGINT DEFAULT 0,
    inodes_used BIGINT DEFAULT 0,
    inodes_free BIGINT DEFAULT 0,
    inodes_percent DOUBLE PRECISION DEFAULT 0
);

CREATE INDEX idx_disk_metrics_host_timestamp ON disk_metrics(host_id, timestamp DESC);
CREATE INDEX idx_disk_metrics_mount ON disk_metrics(host_id, mount_point);
CREATE INDEX idx_disk_metrics_timestamp ON disk_metrics(timestamp);

CREATE TABLE IF NOT EXISTS disk_health (
    id BIGSERIAL PRIMARY KEY,
    host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
    device TEXT NOT NULL,
    model TEXT,
    serial_number TEXT,
    smart_status TEXT CHECK (smart_status IN ('PASSED', 'FAILED', 'UNKNOWN', 'NOT_AVAILABLE')),
    temperature INTEGER,
    power_on_hours BIGINT,
    power_cycles BIGINT,
    realloc_sectors INTEGER DEFAULT 0,
    pending_sectors INTEGER DEFAULT 0
);

CREATE INDEX idx_disk_health_host_timestamp ON disk_health(host_id, timestamp DESC);
CREATE INDEX idx_disk_health_device ON disk_health(host_id, device);
CREATE INDEX idx_disk_health_status ON disk_health(smart_status);

COMMENT ON TABLE disk_metrics IS 'Filesystem usage metrics collected via df';
COMMENT ON TABLE disk_health IS 'SMART disk health data collected via smartctl (optional)';
COMMENT ON COLUMN disk_health.smart_status IS 'Overall SMART health assessment';
