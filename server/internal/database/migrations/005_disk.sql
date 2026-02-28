-- Disk metrics (detailed usage per mount point with inodes) and disk health (SMART)

CREATE TABLE IF NOT EXISTS disk_metrics (
    id BIGSERIAL PRIMARY KEY,
    host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    mount_point VARCHAR(255) NOT NULL,
    filesystem VARCHAR(255) NOT NULL DEFAULT '',
    size_gb DOUBLE PRECISION DEFAULT 0,
    used_gb DOUBLE PRECISION DEFAULT 0,
    avail_gb DOUBLE PRECISION DEFAULT 0,
    used_percent DOUBLE PRECISION DEFAULT 0,
    inodes_total BIGINT DEFAULT 0,
    inodes_used BIGINT DEFAULT 0,
    inodes_free BIGINT DEFAULT 0,
    inodes_percent DOUBLE PRECISION DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_disk_metrics_host_time ON disk_metrics(host_id, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_disk_metrics_host_mount ON disk_metrics(host_id, mount_point, timestamp DESC);

CREATE TABLE IF NOT EXISTS disk_health (
    id BIGSERIAL PRIMARY KEY,
    host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    device VARCHAR(255) NOT NULL,
    model VARCHAR(255) NOT NULL DEFAULT '',
    serial_number VARCHAR(255) NOT NULL DEFAULT '',
    smart_status VARCHAR(50) NOT NULL DEFAULT 'UNKNOWN',
    temperature INTEGER DEFAULT 0,
    power_on_hours BIGINT DEFAULT 0,
    power_cycles BIGINT DEFAULT 0,
    realloc_sectors INTEGER DEFAULT 0,
    pending_sectors INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_disk_health_host_time ON disk_health(host_id, timestamp DESC);
