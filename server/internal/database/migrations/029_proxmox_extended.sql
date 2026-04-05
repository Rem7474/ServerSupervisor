-- Migration 029: Extended Proxmox monitoring
-- Adds tasks, backup jobs/runs, physical disks, and node update counters (read-only, PVEAuditor-aligned).

-- ─── Task history per node ────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS proxmox_tasks (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID NOT NULL REFERENCES proxmox_connections(id) ON DELETE CASCADE,
    node_name     TEXT NOT NULL DEFAULT '',
    upid          TEXT NOT NULL,
    task_type     TEXT NOT NULL DEFAULT '',    -- vzdump | qmstart | qmstop | …
    status        TEXT NOT NULL DEFAULT 'stopped', -- running | stopped
    user_name     TEXT NOT NULL DEFAULT '',
    start_time    TIMESTAMPTZ,
    end_time      TIMESTAMPTZ,
    exit_status   TEXT NOT NULL DEFAULT '',   -- OK | error message | '' while running
    object_id     TEXT NOT NULL DEFAULT '',   -- vmid or other Proxmox object ID
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (connection_id, upid)
);
CREATE INDEX IF NOT EXISTS idx_proxmox_tasks_conn_node  ON proxmox_tasks(connection_id, node_name);
CREATE INDEX IF NOT EXISTS idx_proxmox_tasks_start_time ON proxmox_tasks(start_time DESC);
CREATE INDEX IF NOT EXISTS idx_proxmox_tasks_type       ON proxmox_tasks(task_type);

-- ─── Backup job configurations (GET /cluster/backup) ─────────────────────────
CREATE TABLE IF NOT EXISTS proxmox_backup_jobs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID NOT NULL REFERENCES proxmox_connections(id) ON DELETE CASCADE,
    job_id        TEXT NOT NULL,
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    schedule      TEXT NOT NULL DEFAULT '',
    storage       TEXT NOT NULL DEFAULT '',
    mode          TEXT NOT NULL DEFAULT 'snapshot', -- snapshot | suspend | stop
    compress      TEXT NOT NULL DEFAULT '',
    vmids         TEXT NOT NULL DEFAULT '',          -- comma-separated VMIDs or 'all'
    mail_to       TEXT NOT NULL DEFAULT '',
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (connection_id, job_id)
);
CREATE INDEX IF NOT EXISTS idx_proxmox_backup_jobs_conn ON proxmox_backup_jobs(connection_id);

-- ─── Latest backup run per VM (derived from vzdump tasks) ────────────────────
CREATE TABLE IF NOT EXISTS proxmox_backup_runs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID NOT NULL REFERENCES proxmox_connections(id) ON DELETE CASCADE,
    node_name     TEXT NOT NULL DEFAULT '',
    vmid          INT  NOT NULL,
    task_upid     TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT '',   -- OK | error | running
    start_time    TIMESTAMPTZ,
    end_time      TIMESTAMPTZ,
    exit_status   TEXT NOT NULL DEFAULT '',
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (connection_id, vmid)
);
CREATE INDEX IF NOT EXISTS idx_proxmox_backup_runs_conn ON proxmox_backup_runs(connection_id);

-- ─── Physical disks per node (GET /nodes/{node}/disks/list) ──────────────────
CREATE TABLE IF NOT EXISTS proxmox_disks (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID NOT NULL REFERENCES proxmox_connections(id) ON DELETE CASCADE,
    node_name     TEXT NOT NULL,
    dev_path      TEXT NOT NULL,
    model         TEXT NOT NULL DEFAULT '',
    serial        TEXT NOT NULL DEFAULT '',
    size_bytes    BIGINT NOT NULL DEFAULT 0,
    disk_type     TEXT NOT NULL DEFAULT '',   -- ssd | hdd | nvme | unknown
    health        TEXT NOT NULL DEFAULT 'UNKNOWN', -- PASSED | FAILED | UNKNOWN
    wearout       INT  NOT NULL DEFAULT -1,   -- SSD wear % (100=new, -1=N/A)
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (connection_id, node_name, dev_path)
);
CREATE INDEX IF NOT EXISTS idx_proxmox_disks_node ON proxmox_disks(connection_id, node_name);

-- ─── Enrich proxmox_nodes with pending update counters ───────────────────────
ALTER TABLE proxmox_nodes ADD COLUMN IF NOT EXISTS pending_updates      INT NOT NULL DEFAULT 0;
ALTER TABLE proxmox_nodes ADD COLUMN IF NOT EXISTS security_updates     INT NOT NULL DEFAULT 0;
ALTER TABLE proxmox_nodes ADD COLUMN IF NOT EXISTS last_update_check_at TIMESTAMPTZ;

