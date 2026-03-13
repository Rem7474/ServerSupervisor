-- Proxmox VE supervision: API-based polling without agent on the hypervisor.
-- Manages multiple Proxmox connections (clusters or standalone nodes).

CREATE TABLE IF NOT EXISTS proxmox_connections (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                 TEXT NOT NULL,
    api_url              TEXT NOT NULL,
    token_id             TEXT NOT NULL,
    token_secret         TEXT NOT NULL,
    insecure_skip_verify BOOLEAN NOT NULL DEFAULT FALSE,
    enabled              BOOLEAN NOT NULL DEFAULT TRUE,
    poll_interval_sec    INT NOT NULL DEFAULT 60,
    last_error           TEXT NOT NULL DEFAULT '',
    last_error_at        TIMESTAMPTZ,
    last_success_at      TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS proxmox_nodes (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID NOT NULL REFERENCES proxmox_connections(id) ON DELETE CASCADE,
    node_name     TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'unknown',
    cpu_count     INT NOT NULL DEFAULT 0,
    cpu_usage     FLOAT NOT NULL DEFAULT 0,
    mem_total     BIGINT NOT NULL DEFAULT 0,
    mem_used      BIGINT NOT NULL DEFAULT 0,
    uptime        BIGINT NOT NULL DEFAULT 0,
    pve_version   TEXT NOT NULL DEFAULT '',
    cluster_name  TEXT NOT NULL DEFAULT '',
    ip_address    TEXT NOT NULL DEFAULT '',
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (connection_id, node_name)
);

CREATE TABLE IF NOT EXISTS proxmox_guests (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID NOT NULL REFERENCES proxmox_connections(id) ON DELETE CASCADE,
    node_name     TEXT NOT NULL,
    guest_type    TEXT NOT NULL, -- vm | lxc
    vmid          INT NOT NULL,
    name          TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'unknown',
    cpu_alloc     FLOAT NOT NULL DEFAULT 0,
    cpu_usage     FLOAT NOT NULL DEFAULT 0,
    mem_alloc     BIGINT NOT NULL DEFAULT 0,
    mem_usage     BIGINT NOT NULL DEFAULT 0,
    disk_alloc    BIGINT NOT NULL DEFAULT 0,
    tags          TEXT NOT NULL DEFAULT '',
    uptime        BIGINT NOT NULL DEFAULT 0,
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (connection_id, node_name, vmid)
);

CREATE TABLE IF NOT EXISTS proxmox_storages (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID NOT NULL REFERENCES proxmox_connections(id) ON DELETE CASCADE,
    node_name     TEXT NOT NULL,
    storage_name  TEXT NOT NULL,
    storage_type  TEXT NOT NULL DEFAULT '',
    total         BIGINT NOT NULL DEFAULT 0,
    used          BIGINT NOT NULL DEFAULT 0,
    avail         BIGINT NOT NULL DEFAULT 0,
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    shared        BOOLEAN NOT NULL DEFAULT FALSE,
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (connection_id, node_name, storage_name)
);

CREATE INDEX IF NOT EXISTS idx_proxmox_nodes_conn     ON proxmox_nodes(connection_id);
CREATE INDEX IF NOT EXISTS idx_proxmox_guests_conn    ON proxmox_guests(connection_id);
CREATE INDEX IF NOT EXISTS idx_proxmox_guests_node    ON proxmox_guests(connection_id, node_name);
CREATE INDEX IF NOT EXISTS idx_proxmox_storages_conn  ON proxmox_storages(connection_id);
CREATE INDEX IF NOT EXISTS idx_proxmox_storages_node  ON proxmox_storages(connection_id, node_name);
