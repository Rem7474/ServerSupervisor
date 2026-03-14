-- Migration 028: Proxmox guest ↔ host links + metrics source selection

CREATE TABLE IF NOT EXISTS proxmox_guest_links (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Reference to the Proxmox guest (deleted automatically when the guest is cleaned up)
    guest_id       UUID NOT NULL REFERENCES proxmox_guests(id) ON DELETE CASCADE,
    -- Reference to the ServerSupervisor host (agent)
    host_id        UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    -- Link lifecycle: suggested (auto-detected) | confirmed (user-validated) | ignored (user-dismissed)
    status         TEXT NOT NULL DEFAULT 'suggested'
                       CHECK (status IN ('suggested', 'confirmed', 'ignored')),
    -- Which source to use for CPU/RAM/disk metrics in host views
    -- auto: prefer proxmox when online, fallback to agent
    -- agent: always use agent metrics
    -- proxmox: always use proxmox metrics
    metrics_source TEXT NOT NULL DEFAULT 'auto'
                       CHECK (metrics_source IN ('auto', 'agent', 'proxmox')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- One guest can only be linked to one host
    UNIQUE (guest_id)
);

CREATE INDEX IF NOT EXISTS idx_proxmox_guest_links_host_id  ON proxmox_guest_links(host_id);
CREATE INDEX IF NOT EXISTS idx_proxmox_guest_links_status   ON proxmox_guest_links(status);
