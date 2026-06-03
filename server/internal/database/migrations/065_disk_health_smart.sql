-- Migration 065: persist two SMART health fields the agent already collects but
-- the server previously dropped — uncorrectable_sectors (pending/offline
-- uncorrectable sector count) and percentage_used (SSD/NVMe wear indicator).
-- power_cycles already exists on disk_health; the agent now populates it too.
--
-- Idempotent and safe on fresh + existing installs (the baseline creates
-- disk_health without these columns; this migration is not subsumed by it).
ALTER TABLE disk_health
    ADD COLUMN IF NOT EXISTS uncorrectable_sectors INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS percentage_used       INTEGER NOT NULL DEFAULT 0;
