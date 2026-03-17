-- Historical metrics snapshots for Proxmox nodes.
-- Stored at each poll cycle to power time-series charts in the dashboard.

CREATE TABLE IF NOT EXISTS proxmox_node_metrics (
    id            BIGSERIAL    PRIMARY KEY,
    node_id       UUID         NOT NULL REFERENCES proxmox_nodes(id) ON DELETE CASCADE,
    connection_id UUID         NOT NULL,
    node_name     TEXT         NOT NULL,
    cpu_usage     FLOAT        NOT NULL DEFAULT 0, -- ratio 0‒1 (raw Proxmox value)
    mem_total     BIGINT       NOT NULL DEFAULT 0,
    mem_used      BIGINT       NOT NULL DEFAULT 0,
    timestamp     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_proxmox_node_metrics_node_ts
    ON proxmox_node_metrics(node_id, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_proxmox_node_metrics_ts
    ON proxmox_node_metrics(timestamp DESC);
