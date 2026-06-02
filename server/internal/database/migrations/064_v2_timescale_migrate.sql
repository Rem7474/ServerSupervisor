-- Migration 064 (V2): drop legacy aggregate/disk tables and convert the
-- append-only time-series tables to TimescaleDB hypertables on EXISTING installs.
--
-- The V2 baseline (000_full_baseline_breaking.sql) builds everything fresh for
-- new installs, but existing deployments skip the baseline (the runner pre-marks
-- it applied when `hosts` already exists). This migration is NOT subsumed by the
-- baseline, so it runs on every install — guarded to be a no-op where the
-- conversion already happened.
--
-- The continuous aggregate (system_metrics_5min) is NOT created here: a
-- continuous aggregate cannot be created inside a transaction/DO block, and we
-- need it gated on TimescaleDB availability. It is created from Go at startup
-- (DB.ensureTimescaleObjects), which also keeps fresh + existing installs in sync.
--
-- WARNING: create_hypertable(..., migrate_data => true) rewrites existing data.
-- On large tables this can take a while at first boot — back up and use a
-- maintenance window before upgrading a production database.

-- V2 schema cleanup (safe on any PostgreSQL, Timescale or not):
--   * disk_info duplicated disk_metrics (both fed by the same agent report) and
--     its FK to system_metrics(id) blocked the hypertable conversion. Equivalent
--     current data already lives in disk_metrics; historical disk_info is dropped.
--   * metrics_aggregates is superseded by the system_metrics_5min continuous
--     aggregate.
DROP TABLE IF EXISTS disk_info CASCADE;
DROP TABLE IF EXISTS metrics_aggregates CASCADE;

-- Hypertable conversion (TimescaleDB only). Skipped gracefully on a plain
-- PostgreSQL instance — the application then keeps using the raw-query code
-- paths (db.hasTimescaleDB == false).
DO $$
DECLARE
  tsdb_available BOOLEAN := FALSE;
BEGIN
  SELECT EXISTS(SELECT 1 FROM pg_available_extensions WHERE name = 'timescaledb')
    INTO tsdb_available;

  IF NOT tsdb_available THEN
    RAISE NOTICE 'TimescaleDB not available; skipping V2 hypertable conversion. '
                 'Install the timescaledb extension (timescale/timescaledb image) '
                 'to enable hypertables, compression and continuous aggregates.';
    RETURN;
  END IF;

  CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

  -- TimescaleDB requires every unique/primary key to include the partition
  -- column, so we replace the plain "id" PK with a composite (id, <time col>)
  -- and keep a UNIQUE(id) for stable row identity. Each block is guarded so
  -- re-runs (and fresh installs that already have hypertables) skip it.

  -- ── system_metrics (partition: timestamp) ──────────────────────────────────
  IF NOT EXISTS (SELECT 1 FROM timescaledb_information.hypertables
                 WHERE hypertable_name = 'system_metrics') THEN
    ALTER TABLE system_metrics DROP CONSTRAINT system_metrics_pkey;
    ALTER TABLE system_metrics ADD PRIMARY KEY (id, timestamp);
    ALTER TABLE system_metrics ADD CONSTRAINT system_metrics_id_unique UNIQUE (id);
    PERFORM create_hypertable('system_metrics', 'timestamp', migrate_data => true);
    ALTER TABLE system_metrics SET (timescaledb.compress, timescaledb.compress_segmentby = 'host_id');
    PERFORM add_compression_policy('system_metrics', INTERVAL '7 days');
    PERFORM add_retention_policy('system_metrics', INTERVAL '30 days');
  END IF;

  -- ── disk_metrics (partition: timestamp) ────────────────────────────────────
  IF NOT EXISTS (SELECT 1 FROM timescaledb_information.hypertables
                 WHERE hypertable_name = 'disk_metrics') THEN
    ALTER TABLE disk_metrics DROP CONSTRAINT disk_metrics_pkey;
    ALTER TABLE disk_metrics ADD PRIMARY KEY (id, timestamp);
    ALTER TABLE disk_metrics ADD CONSTRAINT disk_metrics_id_unique UNIQUE (id);
    PERFORM create_hypertable('disk_metrics', 'timestamp', migrate_data => true);
    ALTER TABLE disk_metrics SET (timescaledb.compress, timescaledb.compress_segmentby = 'host_id,mount_point');
    PERFORM add_compression_policy('disk_metrics', INTERVAL '7 days');
    PERFORM add_retention_policy('disk_metrics', INTERVAL '30 days');
  END IF;

  -- ── disk_health (partition: timestamp) ─────────────────────────────────────
  IF NOT EXISTS (SELECT 1 FROM timescaledb_information.hypertables
                 WHERE hypertable_name = 'disk_health') THEN
    ALTER TABLE disk_health DROP CONSTRAINT disk_health_pkey;
    ALTER TABLE disk_health ADD PRIMARY KEY (id, timestamp);
    ALTER TABLE disk_health ADD CONSTRAINT disk_health_id_unique UNIQUE (id);
    PERFORM create_hypertable('disk_health', 'timestamp', migrate_data => true);
    ALTER TABLE disk_health SET (timescaledb.compress, timescaledb.compress_segmentby = 'host_id,device');
    PERFORM add_compression_policy('disk_health', INTERVAL '7 days');
    PERFORM add_retention_policy('disk_health', INTERVAL '90 days');
  END IF;

  -- ── login_events (partition: created_at) ───────────────────────────────────
  IF NOT EXISTS (SELECT 1 FROM timescaledb_information.hypertables
                 WHERE hypertable_name = 'login_events') THEN
    ALTER TABLE login_events DROP CONSTRAINT login_events_pkey;
    ALTER TABLE login_events ADD PRIMARY KEY (id, created_at);
    ALTER TABLE login_events ADD CONSTRAINT login_events_id_unique UNIQUE (id);
    PERFORM create_hypertable('login_events', 'created_at', migrate_data => true);
    PERFORM add_retention_policy('login_events', INTERVAL '90 days');
  END IF;

  -- ── proxmox_node_metrics (partition: timestamp) ────────────────────────────
  IF NOT EXISTS (SELECT 1 FROM timescaledb_information.hypertables
                 WHERE hypertable_name = 'proxmox_node_metrics') THEN
    ALTER TABLE proxmox_node_metrics DROP CONSTRAINT proxmox_node_metrics_pkey;
    ALTER TABLE proxmox_node_metrics ADD PRIMARY KEY (id, timestamp);
    ALTER TABLE proxmox_node_metrics ADD CONSTRAINT proxmox_node_metrics_id_unique UNIQUE (id);
    PERFORM create_hypertable('proxmox_node_metrics', 'timestamp', migrate_data => true);
    ALTER TABLE proxmox_node_metrics SET (timescaledb.compress, timescaledb.compress_segmentby = 'node_id');
    PERFORM add_compression_policy('proxmox_node_metrics', INTERVAL '7 days');
    PERFORM add_retention_policy('proxmox_node_metrics', INTERVAL '30 days');
  END IF;

  -- ── proxmox_guest_metrics (partition: timestamp) ───────────────────────────
  IF NOT EXISTS (SELECT 1 FROM timescaledb_information.hypertables
                 WHERE hypertable_name = 'proxmox_guest_metrics') THEN
    ALTER TABLE proxmox_guest_metrics DROP CONSTRAINT proxmox_guest_metrics_pkey;
    ALTER TABLE proxmox_guest_metrics ADD PRIMARY KEY (id, timestamp);
    ALTER TABLE proxmox_guest_metrics ADD CONSTRAINT proxmox_guest_metrics_id_unique UNIQUE (id);
    PERFORM create_hypertable('proxmox_guest_metrics', 'timestamp', migrate_data => true);
    ALTER TABLE proxmox_guest_metrics SET (timescaledb.compress, timescaledb.compress_segmentby = 'guest_id');
    PERFORM add_compression_policy('proxmox_guest_metrics', INTERVAL '7 days');
    PERFORM add_retention_policy('proxmox_guest_metrics', INTERVAL '30 days');
  END IF;
END $$;
