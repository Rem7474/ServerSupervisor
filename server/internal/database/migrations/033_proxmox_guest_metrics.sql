-- Per-guest CPU/RAM snapshots recorded at each Proxmox poll.
-- Used to serve historical charts in HostDetailView when metrics_source=proxmox.
CREATE TABLE IF NOT EXISTS proxmox_guest_metrics (
    id          BIGSERIAL   PRIMARY KEY,
    guest_id    UUID        NOT NULL REFERENCES proxmox_guests(id) ON DELETE CASCADE,
    cpu_usage   FLOAT       NOT NULL DEFAULT 0, -- ratio 0‒1 (raw Proxmox value)
    mem_total   BIGINT      NOT NULL DEFAULT 0,
    mem_used    BIGINT      NOT NULL DEFAULT 0,
    timestamp   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_proxmox_guest_metrics_guest_ts
    ON proxmox_guest_metrics(guest_id, timestamp DESC);
