-- Per-Proxmox-node CPU temperature source host mapping.
ALTER TABLE proxmox_nodes
ADD COLUMN IF NOT EXISTS cpu_temp_source_host_id VARCHAR(64);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_proxmox_nodes_cpu_temp_source_host'
    ) THEN
        ALTER TABLE proxmox_nodes
        ADD CONSTRAINT fk_proxmox_nodes_cpu_temp_source_host
        FOREIGN KEY (cpu_temp_source_host_id) REFERENCES hosts(id)
        ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_proxmox_nodes_cpu_temp_source_host
ON proxmox_nodes(cpu_temp_source_host_id);
