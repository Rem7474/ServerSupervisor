-- Actual used disk bytes for a Proxmox guest, as reported by PVE's own
-- /cluster/resources "disk" field alongside the existing "maxdisk" (already
-- stored as disk_alloc) — reliable for LXC (host-side rootfs inspection),
-- typically 0 for a QEMU VM without further guest-agent calls, same caveat
-- PVE itself has.
ALTER TABLE proxmox_guests ADD COLUMN disk_usage bigint DEFAULT 0 NOT NULL;
