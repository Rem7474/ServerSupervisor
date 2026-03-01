-- TimescaleDB activation: hypertables, compression, and retention policies.
-- Uses pg_available_extensions to check availability upfront rather than a
-- catch-all EXCEPTION, so real errors (e.g. bad ALTER TABLE) still propagate.

DO $$
DECLARE
  tsdb_available BOOLEAN := FALSE;
BEGIN

  -- ── Availability check ─────────────────────────────────────────────────────
  SELECT EXISTS(SELECT 1 FROM pg_available_extensions WHERE name = 'timescaledb')
    INTO tsdb_available;

  IF NOT tsdb_available THEN
    RAISE NOTICE 'TimescaleDB extension package not found on this PostgreSQL installation. '
                 'Install timescaledb-2-postgresql-XX (or equivalent) to enable time-series '
                 'optimizations (hypertables, compression, retention policies).';
    RETURN;
  END IF;

  -- ── Extension ─────────────────────────────────────────────────────────────
  CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

  -- ── system_metrics ────────────────────────────────────────────────────────
  -- TimescaleDB requires all unique constraints (incl. PK) to include the
  -- partition column.  We drop the plain PK, add a composite PK (id, timestamp),
  -- and keep a UNIQUE (id) so disk_info.metrics_id FK continues to work.
  IF NOT EXISTS (
    SELECT 1 FROM timescaledb_information.hypertables
    WHERE hypertable_name = 'system_metrics'
  ) THEN
    ALTER TABLE system_metrics DROP CONSTRAINT system_metrics_pkey;
    ALTER TABLE system_metrics ADD PRIMARY KEY (id, timestamp);
    ALTER TABLE system_metrics ADD CONSTRAINT system_metrics_id_unique UNIQUE (id);
    PERFORM create_hypertable('system_metrics', 'timestamp', migrate_data => true);

    ALTER TABLE system_metrics SET (
      timescaledb.compress,
      timescaledb.compress_segmentby = 'host_id'
    );
    PERFORM add_compression_policy('system_metrics', INTERVAL '7 days');
    PERFORM add_retention_policy('system_metrics', INTERVAL '30 days');
  END IF;

  -- ── disk_metrics ──────────────────────────────────────────────────────────
  IF NOT EXISTS (
    SELECT 1 FROM timescaledb_information.hypertables
    WHERE hypertable_name = 'disk_metrics'
  ) THEN
    ALTER TABLE disk_metrics DROP CONSTRAINT disk_metrics_pkey;
    ALTER TABLE disk_metrics ADD PRIMARY KEY (id, timestamp);
    ALTER TABLE disk_metrics ADD CONSTRAINT disk_metrics_id_unique UNIQUE (id);
    PERFORM create_hypertable('disk_metrics', 'timestamp', migrate_data => true);

    ALTER TABLE disk_metrics SET (
      timescaledb.compress,
      timescaledb.compress_segmentby = 'host_id,mount_point'
    );
    PERFORM add_compression_policy('disk_metrics', INTERVAL '7 days');
    PERFORM add_retention_policy('disk_metrics', INTERVAL '30 days');
  END IF;

  -- ── disk_health ───────────────────────────────────────────────────────────
  IF NOT EXISTS (
    SELECT 1 FROM timescaledb_information.hypertables
    WHERE hypertable_name = 'disk_health'
  ) THEN
    ALTER TABLE disk_health DROP CONSTRAINT disk_health_pkey;
    ALTER TABLE disk_health ADD PRIMARY KEY (id, timestamp);
    ALTER TABLE disk_health ADD CONSTRAINT disk_health_id_unique UNIQUE (id);
    PERFORM create_hypertable('disk_health', 'timestamp', migrate_data => true);

    ALTER TABLE disk_health SET (
      timescaledb.compress,
      timescaledb.compress_segmentby = 'host_id,device'
    );
    PERFORM add_compression_policy('disk_health', INTERVAL '7 days');
    PERFORM add_retention_policy('disk_health', INTERVAL '90 days');
  END IF;

  -- ── metrics_aggregates ───────────────────────────────────────────────────
  -- The existing UNIQUE (host_id, aggregation_type, timestamp) already includes
  -- timestamp, so it stays compatible.  The serial PK gets the same treatment.
  IF NOT EXISTS (
    SELECT 1 FROM timescaledb_information.hypertables
    WHERE hypertable_name = 'metrics_aggregates'
  ) THEN
    ALTER TABLE metrics_aggregates DROP CONSTRAINT metrics_aggregates_pkey;
    ALTER TABLE metrics_aggregates ADD PRIMARY KEY (id, timestamp);
    ALTER TABLE metrics_aggregates ADD CONSTRAINT metrics_aggregates_id_unique UNIQUE (id);
    PERFORM create_hypertable('metrics_aggregates', 'timestamp', migrate_data => true);

    ALTER TABLE metrics_aggregates SET (
      timescaledb.compress,
      timescaledb.compress_segmentby = 'host_id,aggregation_type'
    );
    PERFORM add_compression_policy('metrics_aggregates', INTERVAL '30 days');
    PERFORM add_retention_policy('metrics_aggregates', INTERVAL '365 days');
  END IF;

  -- ── login_events ──────────────────────────────────────────────────────────
  IF NOT EXISTS (
    SELECT 1 FROM timescaledb_information.hypertables
    WHERE hypertable_name = 'login_events'
  ) THEN
    ALTER TABLE login_events DROP CONSTRAINT login_events_pkey;
    ALTER TABLE login_events ADD PRIMARY KEY (id, created_at);
    ALTER TABLE login_events ADD CONSTRAINT login_events_id_unique UNIQUE (id);
    PERFORM create_hypertable('login_events', 'created_at', migrate_data => true);
    PERFORM add_retention_policy('login_events', INTERVAL '90 days');
  END IF;

END $$;
