-- BREAKING: Full database baseline consolidation
-- Fresh install: apply this baseline and mark legacy migrations as applied.
-- Existing install: this file is skipped by migrate() when hosts table already exists.

-- ===== BEGIN 001_core.sql =====
-- Core tables: users, refresh_tokens, hosts, system_metrics, disk_info,
-- audit_logs, alert_rules, alert_incidents

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'viewer',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) UNIQUE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id);

CREATE TABLE IF NOT EXISTS hosts (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    hostname VARCHAR(255) NOT NULL DEFAULT '',
    ip_address VARCHAR(45) NOT NULL,
    os VARCHAR(255) NOT NULL DEFAULT '',
    api_key VARCHAR(255) NOT NULL,
    tags JSONB DEFAULT '[]',
    status VARCHAR(20) NOT NULL DEFAULT 'offline',
    last_seen TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS system_metrics (
    id BIGSERIAL PRIMARY KEY,
    host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    cpu_usage_percent DOUBLE PRECISION,
    cpu_cores INTEGER,
    cpu_model VARCHAR(255),
    load_avg_1 DOUBLE PRECISION,
    load_avg_5 DOUBLE PRECISION,
    load_avg_15 DOUBLE PRECISION,
    memory_total BIGINT,
    memory_used BIGINT,
    memory_free BIGINT,
    memory_percent DOUBLE PRECISION,
    swap_total BIGINT,
    swap_used BIGINT,
    network_rx_bytes BIGINT,
    network_tx_bytes BIGINT,
    uptime BIGINT,
    hostname VARCHAR(255)
);

CREATE INDEX IF NOT EXISTS idx_system_metrics_host_time ON system_metrics(host_id, timestamp DESC);

CREATE TABLE IF NOT EXISTS disk_info (
    id BIGSERIAL PRIMARY KEY,
    metrics_id BIGINT REFERENCES system_metrics(id) ON DELETE CASCADE,
    mount_point VARCHAR(255),
    device VARCHAR(255),
    fs_type VARCHAR(50),
    total_bytes BIGINT,
    used_bytes BIGINT,
    free_bytes BIGINT,
    used_percent DOUBLE PRECISION
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(255) NOT NULL,
    action VARCHAR(100) NOT NULL,
    host_id VARCHAR(64),
    ip_address VARCHAR(45),
    details TEXT,
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_user_action ON audit_logs(username, action, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_host ON audit_logs(host_id, created_at DESC);

CREATE TABLE IF NOT EXISTS alert_rules (
    id SERIAL PRIMARY KEY,
    host_id VARCHAR(64),
    metric VARCHAR(50) NOT NULL,
    operator VARCHAR(5) NOT NULL,
    threshold DOUBLE PRECISION,
    duration_seconds INTEGER DEFAULT 60,
    channel VARCHAR(50) NOT NULL DEFAULT '',
    channel_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS alert_incidents (
    id BIGSERIAL PRIMARY KEY,
    rule_id INTEGER REFERENCES alert_rules(id) ON DELETE CASCADE,
    host_id VARCHAR(64),
    triggered_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    resolved_at TIMESTAMP WITH TIME ZONE,
    value DOUBLE PRECISION
);

CREATE INDEX IF NOT EXISTS idx_alert_incidents_rule ON alert_incidents(rule_id, triggered_at DESC);

-- ===== END 001_core.sql =====

-- ===== BEGIN 002_aggregates.sql =====
-- Metrics aggregates table for downsampling (5min, hourly, daily)

CREATE TABLE IF NOT EXISTS metrics_aggregates (
    id BIGSERIAL PRIMARY KEY,
    host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
    aggregation_type VARCHAR(20) NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    cpu_usage_avg DOUBLE PRECISION,
    cpu_usage_max DOUBLE PRECISION,
    memory_usage_avg BIGINT,
    memory_usage_max BIGINT,
    disk_usage_avg DOUBLE PRECISION,
    network_rx_bytes BIGINT,
    network_tx_bytes BIGINT,
    sample_count INTEGER DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_metrics_aggregates_host_time ON metrics_aggregates(host_id, aggregation_type, timestamp DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_metrics_aggregates_unique ON metrics_aggregates(host_id, aggregation_type, timestamp);

-- ===== END 002_aggregates.sql =====

-- ===== BEGIN 003_docker.sql =====
-- Docker containers, apt_status, and legacy apt_commands table

CREATE TABLE IF NOT EXISTS docker_containers (
    id VARCHAR(64) PRIMARY KEY,
    host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
    container_id VARCHAR(64),
    name VARCHAR(255),
    image VARCHAR(512),
    image_tag VARCHAR(255),
    image_id VARCHAR(255),
    state VARCHAR(50),
    status VARCHAR(255),
    created TIMESTAMP WITH TIME ZONE,
    ports TEXT,
    labels JSONB DEFAULT '{}',
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_docker_containers_host ON docker_containers(host_id);

CREATE TABLE IF NOT EXISTS apt_status (
    id BIGSERIAL PRIMARY KEY,
    host_id VARCHAR(64) UNIQUE REFERENCES hosts(id) ON DELETE CASCADE,
    last_update TIMESTAMP WITH TIME ZONE,
    last_upgrade TIMESTAMP WITH TIME ZONE,
    pending_packages INTEGER DEFAULT 0,
    package_list JSONB DEFAULT '[]',
    security_updates INTEGER DEFAULT 0,
    cve_list JSONB DEFAULT '[]',
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Legacy table: replaced by remote_commands in 008_remote_commands.sql
CREATE TABLE IF NOT EXISTS apt_commands (
    id BIGSERIAL PRIMARY KEY,
    host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
    command VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    output TEXT DEFAULT '',
    audit_log_id BIGINT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    ended_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_apt_commands_host_status ON apt_commands(host_id, status);

CREATE TABLE IF NOT EXISTS tracked_repos (
    id SERIAL PRIMARY KEY,
    owner VARCHAR(255) NOT NULL,
    repo VARCHAR(255) NOT NULL,
    display_name VARCHAR(255),
    latest_version VARCHAR(255) DEFAULT '',
    latest_date TIMESTAMP WITH TIME ZONE,
    release_url TEXT DEFAULT '',
    docker_image VARCHAR(512) DEFAULT '',
    checked_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(owner, repo)
);

-- ===== END 003_docker.sql =====

-- ===== BEGIN 004_topology.sql =====
-- Docker networks, container envs, network topology config, compose projects,
-- and legacy docker_commands table

CREATE TABLE IF NOT EXISTS docker_networks (
    id VARCHAR(64) PRIMARY KEY,
    host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
    network_id VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    driver VARCHAR(50) DEFAULT 'bridge',
    scope VARCHAR(20) DEFAULT 'local',
    container_ids JSONB DEFAULT '[]'::jsonb,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_docker_networks_host ON docker_networks(host_id);

CREATE TABLE IF NOT EXISTS container_envs (
    id BIGSERIAL PRIMARY KEY,
    host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
    container_name VARCHAR(255) NOT NULL,
    env_vars JSONB DEFAULT '{}'::jsonb,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_container_envs_host ON container_envs(host_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_container_envs_host_name ON container_envs(host_id, container_name);

CREATE TABLE IF NOT EXISTS network_topology_config (
    id SERIAL PRIMARY KEY,
    root_label VARCHAR(255) DEFAULT 'Infrastructure',
    root_ip VARCHAR(45) DEFAULT '',
    excluded_ports JSONB DEFAULT '[]'::jsonb,
    service_map TEXT DEFAULT '{}',
    show_proxy_links BOOLEAN DEFAULT TRUE,
    host_overrides TEXT DEFAULT '{}',
    manual_services TEXT DEFAULT '[]',
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

INSERT INTO network_topology_config (id, root_label)
SELECT 1, 'Infrastructure' WHERE NOT EXISTS (SELECT 1 FROM network_topology_config);

CREATE UNIQUE INDEX IF NOT EXISTS network_topology_config_singleton ON network_topology_config (id) WHERE id = 1;

CREATE TABLE IF NOT EXISTS compose_projects (
    id VARCHAR(255) PRIMARY KEY,
    host_id VARCHAR(64) NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    working_dir TEXT NOT NULL DEFAULT '',
    config_file TEXT NOT NULL DEFAULT '',
    services TEXT NOT NULL DEFAULT '[]',
    raw_config TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_compose_projects_host_id ON compose_projects(host_id);

-- Legacy table: replaced by remote_commands in 008_remote_commands.sql
CREATE TABLE IF NOT EXISTS docker_commands (
    id BIGSERIAL PRIMARY KEY,
    host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
    container_name VARCHAR(255) NOT NULL,
    action VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    output TEXT DEFAULT '',
    triggered_by VARCHAR(255) DEFAULT 'system',
    audit_log_id BIGINT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    ended_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_docker_commands_host_status ON docker_commands(host_id, status);

-- ===== END 004_topology.sql =====

-- ===== BEGIN 005_disk.sql =====
-- Disk metrics (detailed usage per mount point with inodes) and disk health (SMART)

CREATE TABLE IF NOT EXISTS disk_metrics (
    id BIGSERIAL PRIMARY KEY,
    host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    mount_point VARCHAR(255) NOT NULL,
    filesystem VARCHAR(255) NOT NULL DEFAULT '',
    size_gb DOUBLE PRECISION DEFAULT 0,
    used_gb DOUBLE PRECISION DEFAULT 0,
    avail_gb DOUBLE PRECISION DEFAULT 0,
    used_percent DOUBLE PRECISION DEFAULT 0,
    inodes_total BIGINT DEFAULT 0,
    inodes_used BIGINT DEFAULT 0,
    inodes_free BIGINT DEFAULT 0,
    inodes_percent DOUBLE PRECISION DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_disk_metrics_host_time ON disk_metrics(host_id, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_disk_metrics_host_mount ON disk_metrics(host_id, mount_point, timestamp DESC);

CREATE TABLE IF NOT EXISTS disk_health (
    id BIGSERIAL PRIMARY KEY,
    host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    device VARCHAR(255) NOT NULL,
    model VARCHAR(255) NOT NULL DEFAULT '',
    serial_number VARCHAR(255) NOT NULL DEFAULT '',
    smart_status VARCHAR(50) NOT NULL DEFAULT 'UNKNOWN',
    temperature INTEGER DEFAULT 0,
    power_on_hours BIGINT DEFAULT 0,
    power_cycles BIGINT DEFAULT 0,
    realloc_sectors INTEGER DEFAULT 0,
    pending_sectors INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_disk_health_host_time ON disk_health(host_id, timestamp DESC);

-- ===== END 005_disk.sql =====

-- ===== BEGIN 006_settings.sql =====
-- Login events table for security auditing and brute-force detection,
-- and dynamic settings table (DB overrides env vars, no restart needed)

CREATE TABLE IF NOT EXISTS login_events (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(255) NOT NULL,
    ip_address VARCHAR(45) NOT NULL DEFAULT '',
    success BOOLEAN NOT NULL,
    user_agent VARCHAR(500) DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_login_events_ip_time ON login_events(ip_address, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_login_events_user_time ON login_events(username, created_at DESC);

CREATE TABLE IF NOT EXISTS settings (
    key VARCHAR(100) PRIMARY KEY,
    value TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ===== END 006_settings.sql =====

-- ===== BEGIN 007_alter_columns.sql =====
-- Column additions via ALTER TABLE (applied after all base tables are created)

-- hosts: add missing columns for older databases
ALTER TABLE IF EXISTS hosts ADD COLUMN IF NOT EXISTS name VARCHAR(255) NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS hosts ADD COLUMN IF NOT EXISTS tags JSONB DEFAULT '[]'::jsonb;

ALTER TABLE IF EXISTS hosts ADD COLUMN IF NOT EXISTS agent_version VARCHAR(20) DEFAULT '';

-- apt_status: convert package_list from TEXT to JSONB
ALTER TABLE IF EXISTS apt_status ALTER COLUMN package_list DROP DEFAULT;

ALTER TABLE IF EXISTS apt_status ALTER COLUMN package_list TYPE JSONB USING COALESCE(package_list::jsonb, '[]'::jsonb);

ALTER TABLE IF EXISTS apt_status ALTER COLUMN package_list SET DEFAULT '[]'::jsonb;

-- apt_status: add CVE tracking column
ALTER TABLE IF EXISTS apt_status ADD COLUMN IF NOT EXISTS cve_list JSONB DEFAULT '[]'::jsonb;

ALTER TABLE IF EXISTS apt_status ALTER COLUMN cve_list DROP DEFAULT;

ALTER TABLE IF EXISTS apt_status ALTER COLUMN cve_list TYPE JSONB USING COALESCE(cve_list::jsonb, '[]'::jsonb);

ALTER TABLE IF EXISTS apt_status ALTER COLUMN cve_list SET DEFAULT '[]'::jsonb;

-- users: TOTP and RBAC fields
ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS totp_secret TEXT DEFAULT '';

ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS backup_codes TEXT DEFAULT '[]';

ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS mfa_enabled BOOLEAN DEFAULT FALSE;

-- users: convert backup_codes from TEXT to JSONB
ALTER TABLE IF EXISTS users ALTER COLUMN backup_codes DROP DEFAULT;

ALTER TABLE IF EXISTS users ALTER COLUMN backup_codes TYPE JSONB USING COALESCE(backup_codes::jsonb, '[]'::jsonb);

ALTER TABLE IF EXISTS users ALTER COLUMN backup_codes SET DEFAULT '[]'::jsonb;

-- users: first-login password change enforcement
ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT FALSE;

-- apt_commands: who launched it and link to audit_logs
ALTER TABLE IF EXISTS apt_commands ADD COLUMN IF NOT EXISTS triggered_by VARCHAR(255) DEFAULT 'system';

ALTER TABLE IF EXISTS apt_commands ADD COLUMN IF NOT EXISTS audit_log_id BIGINT;

-- alert_rules: extended notification config columns
ALTER TABLE IF EXISTS alert_rules ADD COLUMN IF NOT EXISTS name VARCHAR(255);

ALTER TABLE IF EXISTS alert_rules ADD COLUMN IF NOT EXISTS channels JSONB DEFAULT '[]'::jsonb;

ALTER TABLE IF EXISTS alert_rules ADD COLUMN IF NOT EXISTS smtp_to VARCHAR(255);

ALTER TABLE IF EXISTS alert_rules ADD COLUMN IF NOT EXISTS ntfy_topic VARCHAR(255);

ALTER TABLE IF EXISTS alert_rules ADD COLUMN IF NOT EXISTS cooldown INTEGER;

ALTER TABLE IF EXISTS alert_rules ADD COLUMN IF NOT EXISTS last_fired TIMESTAMP WITH TIME ZONE;

ALTER TABLE IF EXISTS alert_rules ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();

-- network_topology_config: Authelia/Internet topology node fields
ALTER TABLE network_topology_config ADD COLUMN IF NOT EXISTS authelia_label VARCHAR(255) DEFAULT 'Authelia';

ALTER TABLE network_topology_config ADD COLUMN IF NOT EXISTS authelia_ip VARCHAR(45) DEFAULT '';

ALTER TABLE network_topology_config ADD COLUMN IF NOT EXISTS internet_label VARCHAR(255) DEFAULT 'Internet';

ALTER TABLE network_topology_config ADD COLUMN IF NOT EXISTS internet_ip VARCHAR(45) DEFAULT '';

-- docker_containers: extended container metadata
ALTER TABLE IF EXISTS docker_containers ADD COLUMN IF NOT EXISTS env_vars JSONB DEFAULT '{}'::jsonb;

ALTER TABLE IF EXISTS docker_containers ADD COLUMN IF NOT EXISTS volumes JSONB DEFAULT '[]'::jsonb;

ALTER TABLE IF EXISTS docker_containers ADD COLUMN IF NOT EXISTS networks JSONB DEFAULT '[]'::jsonb;

-- docker_commands: compose project working directory
ALTER TABLE IF EXISTS docker_commands ADD COLUMN IF NOT EXISTS working_dir TEXT DEFAULT '';

-- ===== END 007_alter_columns.sql =====

-- ===== BEGIN 008_remote_commands.sql =====
-- Unified remote_commands table (replaces legacy docker_commands + apt_commands)

CREATE TABLE IF NOT EXISTS remote_commands (
    id           VARCHAR(36)  PRIMARY KEY,
    host_id      VARCHAR(64)  REFERENCES hosts(id) ON DELETE CASCADE,
    module       VARCHAR(50)  NOT NULL,
    action       VARCHAR(100) NOT NULL,
    target       VARCHAR(255) NOT NULL DEFAULT '',
    payload      TEXT         NOT NULL DEFAULT '{}',
    status       VARCHAR(20)  NOT NULL DEFAULT 'pending',
    output       TEXT         NOT NULL DEFAULT '',
    triggered_by VARCHAR(255) NOT NULL DEFAULT 'system',
    audit_log_id BIGINT,
    created_at   TIMESTAMPTZ  DEFAULT NOW(),
    started_at   TIMESTAMPTZ,
    ended_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_remote_commands_host_status ON remote_commands(host_id, status);

DROP TABLE IF EXISTS docker_commands;

DROP TABLE IF EXISTS apt_commands;

-- ===== END 008_remote_commands.sql =====

-- ===== BEGIN 009_alert_actions.sql =====
-- Consolidate alert_rules notification config into a single actions JSONB column
-- (Sprint 3c: replaces channel, channel_config, channels, smtp_to, ntfy_topic, cooldown)

ALTER TABLE IF EXISTS alert_rules ADD COLUMN IF NOT EXISTS actions JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE alert_rules SET actions = jsonb_build_object(
    'channels', COALESCE(channels, '[]'::jsonb),
    'smtp_to', COALESCE(smtp_to, ''),
    'ntfy_topic', COALESCE(ntfy_topic, ''),
    'cooldown', COALESCE(cooldown, 0)
) WHERE actions = '{}'::jsonb OR actions IS NULL;

ALTER TABLE IF EXISTS alert_rules DROP COLUMN IF EXISTS channel;

ALTER TABLE IF EXISTS alert_rules DROP COLUMN IF EXISTS channel_config;

ALTER TABLE IF EXISTS alert_rules DROP COLUMN IF EXISTS channels;

ALTER TABLE IF EXISTS alert_rules DROP COLUMN IF EXISTS smtp_to;

ALTER TABLE IF EXISTS alert_rules DROP COLUMN IF EXISTS ntfy_topic;

ALTER TABLE IF EXISTS alert_rules DROP COLUMN IF EXISTS cooldown;

-- ===== END 009_alert_actions.sql =====

-- ===== BEGIN 010_timescaledb.sql =====
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

-- ===== END 010_timescaledb.sql =====

-- ===== BEGIN 011_refactor.sql =====
-- Refactor sprint: indexes, FK integrity, TEXT→JSONB conversions, container_envs cleanup.
-- All statements are idempotent (IF NOT EXISTS / DO blocks).

-- ── 1. Missing indexes ──────────────────────────────────────────────────────

CREATE INDEX IF NOT EXISTS idx_remote_commands_created
    ON remote_commands(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_remote_commands_triggered
    ON remote_commands(triggered_by);

CREATE INDEX IF NOT EXISTS idx_docker_containers_state
    ON docker_containers(state);

CREATE INDEX IF NOT EXISTS idx_audit_logs_created
    ON audit_logs(created_at DESC);

-- ── 2. FK remote_commands.audit_log_id → audit_logs(id) ────────────────────

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE constraint_name = 'fk_remote_commands_audit'
      AND table_name = 'remote_commands'
  ) THEN
    -- Nullify any dangling references before adding the FK
    UPDATE remote_commands
    SET audit_log_id = NULL
    WHERE audit_log_id IS NOT NULL
      AND NOT EXISTS (SELECT 1 FROM audit_logs WHERE id = remote_commands.audit_log_id);

    ALTER TABLE remote_commands
      ADD CONSTRAINT fk_remote_commands_audit
      FOREIGN KEY (audit_log_id) REFERENCES audit_logs(id) ON DELETE SET NULL;
  END IF;
END $$;

-- ── 3. network_topology_config: TEXT → JSONB ────────────────────────────────

DO $$
BEGIN
  IF (
    SELECT data_type FROM information_schema.columns
    WHERE table_name = 'network_topology_config' AND column_name = 'service_map'
  ) = 'text' THEN
    -- Drop existing TEXT defaults first so PostgreSQL can change the column type
    ALTER TABLE network_topology_config
      ALTER COLUMN service_map     DROP DEFAULT,
      ALTER COLUMN host_overrides  DROP DEFAULT,
      ALTER COLUMN manual_services DROP DEFAULT;

    ALTER TABLE network_topology_config
      ALTER COLUMN service_map     TYPE JSONB USING COALESCE(NULLIF(service_map,    '')::jsonb, '{}'::jsonb),
      ALTER COLUMN host_overrides  TYPE JSONB USING COALESCE(NULLIF(host_overrides, '')::jsonb, '{}'::jsonb),
      ALTER COLUMN manual_services TYPE JSONB USING COALESCE(NULLIF(manual_services,'')::jsonb, '[]'::jsonb);

    ALTER TABLE network_topology_config
      ALTER COLUMN service_map     SET DEFAULT '{}'::jsonb,
      ALTER COLUMN host_overrides  SET DEFAULT '{}'::jsonb,
      ALTER COLUMN manual_services SET DEFAULT '[]'::jsonb;
  END IF;
END $$;

-- ── 4. compose_projects.services: TEXT → JSONB ──────────────────────────────

DO $$
BEGIN
  IF (
    SELECT data_type FROM information_schema.columns
    WHERE table_name = 'compose_projects' AND column_name = 'services'
  ) = 'text' THEN
    ALTER TABLE compose_projects
      ALTER COLUMN services DROP DEFAULT;

    ALTER TABLE compose_projects
      ALTER COLUMN services TYPE JSONB USING COALESCE(NULLIF(services,'')::jsonb, '[]'::jsonb);

    ALTER TABLE compose_projects
      ALTER COLUMN services SET DEFAULT '[]'::jsonb;
  END IF;
END $$;

-- ── 5. Drop container_envs (data lives in docker_containers.env_vars) ───────
-- Network topology inference is migrated to query docker_containers instead.

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_name = 'container_envs'
  ) THEN
    DROP TABLE container_envs;
  END IF;
END $$;

-- ===== END 011_refactor.sql =====

-- ===== BEGIN 012_container_netstats.sql =====
ALTER TABLE docker_containers ADD COLUMN IF NOT EXISTS net_rx_bytes BIGINT DEFAULT 0;
ALTER TABLE docker_containers ADD COLUMN IF NOT EXISTS net_tx_bytes BIGINT DEFAULT 0;

-- ===== END 012_container_netstats.sql =====

-- ===== BEGIN 013_alert_incidents_fk.sql =====
-- Change alert_incidents.rule_id FK from ON DELETE CASCADE to ON DELETE SET NULL.
-- This preserves historical incidents when an alert rule is deleted instead of
-- silently purging them (which caused {notifications: [], total: 0} in the UI).
ALTER TABLE alert_incidents DROP CONSTRAINT IF EXISTS alert_incidents_rule_id_fkey;
ALTER TABLE alert_incidents
    ADD CONSTRAINT alert_incidents_rule_id_fkey
    FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE SET NULL;

-- ===== END 013_alert_incidents_fk.sql =====

-- ===== BEGIN 014_scheduled_tasks.sql =====
-- Migration 014: Scheduled Tasks
-- Stores per-host cron-scheduled remote commands (apt, docker, systemd, custom, …)

CREATE TABLE IF NOT EXISTS scheduled_tasks (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    host_id         VARCHAR(64) NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    name            TEXT        NOT NULL,
    module          TEXT        NOT NULL,
    action          TEXT        NOT NULL,
    target          TEXT        NOT NULL DEFAULT '',
    payload         JSONB       NOT NULL DEFAULT '{}',
    cron_expression TEXT        NOT NULL,
    enabled         BOOLEAN     NOT NULL DEFAULT TRUE,
    last_run_at     TIMESTAMPTZ,
    next_run_at     TIMESTAMPTZ,
    last_run_status TEXT,
    created_by      TEXT        NOT NULL DEFAULT 'system',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_host    ON scheduled_tasks(host_id);
CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_enabled ON scheduled_tasks(enabled) WHERE enabled = TRUE;

-- ===== END 014_scheduled_tasks.sql =====

-- ===== BEGIN 015_scheduled_task_link.sql =====
-- Migration 015: Link remote commands back to their originating scheduled task
-- Allows ReportCommandResult to propagate final status (completed/failed) to the task.

ALTER TABLE remote_commands
    ADD COLUMN IF NOT EXISTS scheduled_task_id UUID REFERENCES scheduled_tasks(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_remote_commands_scheduled_task
    ON remote_commands(scheduled_task_id) WHERE scheduled_task_id IS NOT NULL;

-- Cache the list of custom tasks declared in the agent's tasks.yaml
ALTER TABLE hosts
    ADD COLUMN IF NOT EXISTS custom_tasks JSONB NOT NULL DEFAULT '[]';

-- Clean up any stale scheduled tasks left in "pending" from before this fix.
-- A task stuck pending for more than 10 minutes is considered failed.
UPDATE scheduled_tasks
    SET last_run_status = 'failed', last_run_at = NOW()
    WHERE last_run_status = 'pending'
      AND last_run_at < NOW() - INTERVAL '10 minutes';

-- ===== END 015_scheduled_task_link.sql =====

-- ===== BEGIN 016_git_webhooks.sql =====
-- Git Webhooks: trigger custom tasks on VMs from GitHub/GitLab/Gitea/Forgejo push events
CREATE TABLE IF NOT EXISTS git_webhooks (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              TEXT NOT NULL,
    secret            TEXT NOT NULL,
    provider          TEXT NOT NULL DEFAULT 'github',
    repo_filter       TEXT NOT NULL DEFAULT '',
    branch_filter     TEXT NOT NULL DEFAULT '',
    event_filter      TEXT NOT NULL DEFAULT 'push',
    host_id           VARCHAR(64) NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    custom_task_id    TEXT NOT NULL,
    notify_channels   TEXT[] NOT NULL DEFAULT '{}',
    notify_on_success BOOLEAN NOT NULL DEFAULT FALSE,
    notify_on_failure BOOLEAN NOT NULL DEFAULT TRUE,
    enabled           BOOLEAN NOT NULL DEFAULT TRUE,
    last_triggered_at TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_git_webhooks_host ON git_webhooks(host_id);
CREATE INDEX IF NOT EXISTS idx_git_webhooks_enabled ON git_webhooks(enabled) WHERE enabled = TRUE;

CREATE TABLE IF NOT EXISTS git_webhook_executions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id     UUID NOT NULL REFERENCES git_webhooks(id) ON DELETE CASCADE,
    command_id     VARCHAR(36) REFERENCES remote_commands(id) ON DELETE SET NULL,
    provider       TEXT NOT NULL DEFAULT '',
    repo_name      TEXT NOT NULL DEFAULT '',
    branch         TEXT NOT NULL DEFAULT '',
    commit_sha     TEXT NOT NULL DEFAULT '',
    commit_message TEXT NOT NULL DEFAULT '',
    pusher         TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'pending',
    triggered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at   TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_git_webhook_executions_webhook ON git_webhook_executions(webhook_id);
CREATE INDEX IF NOT EXISTS idx_git_webhook_executions_command ON git_webhook_executions(command_id);

-- ===== END 016_git_webhooks.sql =====

-- ===== BEGIN 017_release_trackers.sql =====
-- Release Trackers: poll external repos for new releases and trigger custom tasks on VMs
CREATE TABLE IF NOT EXISTS release_trackers (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              TEXT NOT NULL,
    provider          TEXT NOT NULL DEFAULT 'github',
    repo_owner        TEXT NOT NULL,
    repo_name         TEXT NOT NULL,
    host_id           VARCHAR(64) NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    custom_task_id    TEXT NOT NULL,
    last_release_tag  TEXT NOT NULL DEFAULT '',
    last_checked_at   TIMESTAMPTZ,
    last_triggered_at TIMESTAMPTZ,
    notify_channels   TEXT[] NOT NULL DEFAULT '{}',
    notify_on_release BOOLEAN NOT NULL DEFAULT TRUE,
    enabled           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_release_trackers_host    ON release_trackers(host_id);
CREATE INDEX IF NOT EXISTS idx_release_trackers_enabled ON release_trackers(enabled) WHERE enabled = TRUE;

CREATE TABLE IF NOT EXISTS release_tracker_executions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tracker_id   UUID NOT NULL REFERENCES release_trackers(id) ON DELETE CASCADE,
    command_id   VARCHAR(36) REFERENCES remote_commands(id) ON DELETE SET NULL,
    tag_name     TEXT NOT NULL DEFAULT '',
    release_url  TEXT NOT NULL DEFAULT '',
    release_name TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'pending',
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_release_tracker_executions_tracker ON release_tracker_executions(tracker_id);
CREATE INDEX IF NOT EXISTS idx_release_tracker_executions_command ON release_tracker_executions(command_id);

-- ===== END 017_release_trackers.sql =====

-- ===== BEGIN 018_release_tracker_error.sql =====
-- Add last_error to release_trackers to surface API/network errors
ALTER TABLE release_trackers ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '';

-- ===== END 018_release_tracker_error.sql =====

-- ===== BEGIN 019_release_tracker_docker_image.sql =====
-- Add optional docker_image field to release_trackers for dashboard version comparison
ALTER TABLE release_trackers ADD COLUMN IF NOT EXISTS docker_image TEXT NOT NULL DEFAULT '';

-- ===== END 019_release_tracker_docker_image.sql =====

-- ===== BEGIN 020_image_digest.sql =====
-- Add image digest for running container (RepoDigest / manifest sha256)
ALTER TABLE docker_containers ADD COLUMN IF NOT EXISTS image_digest TEXT NOT NULL DEFAULT '';

-- Add latest image digest for release tracker (manifest sha256 of latest release tag)
ALTER TABLE release_trackers ADD COLUMN IF NOT EXISTS latest_image_digest TEXT NOT NULL DEFAULT '';

-- ===== END 020_image_digest.sql =====

-- ===== BEGIN 021_performance_indexes.sql =====
-- Performance indexes for common query patterns.
-- All statements are idempotent (IF NOT EXISTS).

-- system_metrics: composite index for latest-metrics-per-host queries
CREATE INDEX IF NOT EXISTS idx_metrics_host_time
    ON system_metrics(host_id, timestamp DESC);

-- remote_commands: pending-command lookup + history filtered by host/status
CREATE INDEX IF NOT EXISTS idx_commands_host_status
    ON remote_commands(host_id, status, created_at DESC);

-- audit_logs: time-ordered listing for dashboard and audit page
CREATE INDEX IF NOT EXISTS idx_audit_timestamp
    ON audit_logs(created_at DESC);

-- git_webhook_executions: latest executions per webhook
CREATE INDEX IF NOT EXISTS idx_webhook_exec_webhook_time
    ON git_webhook_executions(webhook_id, triggered_at DESC);

-- release_tracker_executions: latest executions per tracker
CREATE INDEX IF NOT EXISTS idx_tracker_exec_tracker_time
    ON release_tracker_executions(tracker_id, triggered_at DESC);

-- ===== END 021_performance_indexes.sql =====

-- ===== BEGIN 022_ip_block_overrides.sql =====
-- Persist manual IP unblocks so they survive server restarts.
-- When an admin unblocks an IP, we record the timestamp here.
-- isIPBlocked only counts login failures that occurred AFTER the last unblock.

CREATE TABLE IF NOT EXISTS ip_block_overrides (
    ip_address  VARCHAR(45) PRIMARY KEY,
    unblocked_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    unblocked_by VARCHAR(255) NOT NULL DEFAULT ''
);

-- ===== END 022_ip_block_overrides.sql =====

-- ===== BEGIN 023_tracker_tag_digests.sql =====
-- Historical digest→tag mapping for release trackers.
-- Each time the poller detects a new release and fetches its manifest digest,
-- it stores the (tag, digest) pair here. This allows resolving the running
-- version of a container using :latest by matching its image digest.
CREATE TABLE IF NOT EXISTS release_tracker_tag_digests (
    tracker_id  UUID    NOT NULL REFERENCES release_trackers(id) ON DELETE CASCADE,
    tag         TEXT    NOT NULL,
    digest      TEXT    NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tracker_id, tag)
);

CREATE INDEX IF NOT EXISTS idx_rttd_tracker_digest ON release_tracker_tag_digests (tracker_id, digest);

-- ===== END 023_tracker_tag_digests.sql =====

-- ===== BEGIN 024_topology_positions.sql =====
-- Add node_positions to persist graph layout across sessions
ALTER TABLE network_topology_config
  ADD COLUMN IF NOT EXISTS node_positions JSONB DEFAULT '{}'::jsonb;

-- ===== END 024_topology_positions.sql =====

-- ===== BEGIN 025_push_notifications.sql =====
-- Web Push subscriptions: one row per browser/device per user.
-- endpoint is globally unique — each subscription identifies one browser install.
CREATE TABLE IF NOT EXISTS push_subscriptions (
    id          SERIAL PRIMARY KEY,
    username    TEXT NOT NULL,
    endpoint    TEXT NOT NULL,
    p256dh_key  TEXT NOT NULL,
    auth_key    TEXT NOT NULL,
    user_agent  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(endpoint)
);

-- Server-side "read up to" timestamp per user.
-- Replaces per-device localStorage so marking as read on PC1 syncs to PC2/mobile.
CREATE TABLE IF NOT EXISTS notification_read_at (
    username TEXT PRIMARY KEY,
    read_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ===== END 025_push_notifications.sql =====

-- ===== BEGIN 026_memory_percent_aggregate.sql =====
-- Add memory_percent_avg to metrics_aggregates for correct chart display beyond 24h

ALTER TABLE metrics_aggregates
    ADD COLUMN IF NOT EXISTS memory_percent_avg DOUBLE PRECISION NOT NULL DEFAULT 0;

-- ===== END 026_memory_percent_aggregate.sql =====

-- ===== BEGIN 027_proxmox.sql =====
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

-- ===== END 027_proxmox.sql =====

-- ===== BEGIN 028_proxmox_links.sql =====
-- Migration 028: Proxmox guest ↔ host links + metrics source selection

CREATE TABLE IF NOT EXISTS proxmox_guest_links (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Reference to the Proxmox guest (deleted automatically when the guest is cleaned up)
    guest_id       UUID NOT NULL REFERENCES proxmox_guests(id) ON DELETE CASCADE,
    -- Reference to the ServerSupervisor host (agent) — TEXT to match hosts.id (VARCHAR)
    host_id        TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
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

-- ===== END 028_proxmox_links.sql =====

-- ===== BEGIN 029_proxmox_extended.sql =====
-- Migration 029: Extended Proxmox monitoring
-- Adds tasks, backup jobs/runs, physical disks, and node update counters (read-only, PVEAuditor compatible).

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

-- ===== END 029_proxmox_extended.sql =====

-- ===== BEGIN 030_repair_alert_rules.sql =====
-- Repair alert_rules table by rebuilding it to eliminate dead (dropped) columns
-- that accumulated from repeated migration runs, causing PostgreSQL's 1600-column limit.
-- This is safe to run on both fresh and existing databases.

DO $$
BEGIN
  -- Only rebuild if the table exists and has dead columns
  IF EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = 'public' AND table_name = 'alert_rules'
  ) THEN
    -- Create clean replacement table
    CREATE TABLE IF NOT EXISTS alert_rules_rebuilt (
      id               SERIAL PRIMARY KEY,
      host_id          VARCHAR(64),
      metric           VARCHAR(50) NOT NULL,
      operator         VARCHAR(5) NOT NULL,
      threshold        DOUBLE PRECISION,
      duration_seconds INTEGER DEFAULT 60,
      enabled          BOOLEAN DEFAULT TRUE,
      created_at       TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
      name             VARCHAR(255),
      last_fired       TIMESTAMP WITH TIME ZONE,
      updated_at       TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
      actions          JSONB NOT NULL DEFAULT '{}'::jsonb
    );

    -- Copy all live data
    INSERT INTO alert_rules_rebuilt
      (id, host_id, metric, operator, threshold, duration_seconds, enabled, created_at, name, last_fired, updated_at, actions)
    SELECT
      id,
      host_id,
      metric,
      operator,
      threshold,
      duration_seconds,
      enabled,
      created_at,
      name,
      last_fired,
      updated_at,
      COALESCE(actions, '{}'::jsonb)
    FROM alert_rules;

    -- Swap tables
    DROP TABLE alert_rules CASCADE;
    ALTER TABLE alert_rules_rebuilt RENAME TO alert_rules;

    -- Restore sequence ownership
    ALTER SEQUENCE alert_rules_rebuilt_id_seq RENAME TO alert_rules_id_seq;
    ALTER TABLE alert_rules ALTER COLUMN id SET DEFAULT nextval('alert_rules_id_seq');
    ALTER SEQUENCE alert_rules_id_seq OWNED BY alert_rules.id;

    -- Restore foreign key from alert_incidents
    ALTER TABLE alert_incidents
      ADD CONSTRAINT alert_incidents_rule_id_fkey
      FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE SET NULL;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_alert_rules_host ON alert_rules(host_id);


-- ===== END 030_repair_alert_rules.sql =====

-- ===== BEGIN 031_tracker_type.sql =====
-- Add tracker_type to distinguish Git release trackers from Docker image trackers.
-- tracker_type: 'git' (default, existing rows) | 'docker'
-- docker_tag: the specific image tag to monitor for docker trackers (e.g., 'latest', 'stable')

ALTER TABLE release_trackers
    ADD COLUMN IF NOT EXISTS tracker_type TEXT NOT NULL DEFAULT 'git',
    ADD COLUMN IF NOT EXISTS docker_tag TEXT NOT NULL DEFAULT '';

-- ===== END 031_tracker_type.sql =====

-- ===== BEGIN 032_proxmox_node_metrics.sql =====
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

-- ===== END 032_proxmox_node_metrics.sql =====

-- ===== BEGIN 033_proxmox_guest_metrics.sql =====
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

-- ===== END 033_proxmox_guest_metrics.sql =====

-- ===== BEGIN 034_tracker_monitor_only.sql =====
-- Allow Docker trackers to work in "monitor-only" mode without a linked host/task.
-- host_id becomes nullable and custom_task_id defaults to empty string.
ALTER TABLE release_trackers
    ALTER COLUMN host_id       DROP NOT NULL,
    ALTER COLUMN custom_task_id SET DEFAULT '';

-- Drop the FK constraint on host_id so it can be NULL
ALTER TABLE release_trackers
    DROP CONSTRAINT IF EXISTS release_trackers_host_id_fkey;

-- Re-add the FK as nullable (ON DELETE SET NULL)
ALTER TABLE release_trackers
    ADD CONSTRAINT release_trackers_host_id_fkey
        FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE SET NULL;

-- ===== END 034_tracker_monitor_only.sql =====

-- ===== BEGIN 035_missing_indexes.sql =====
-- Migration 035: indexes manquants identifiés à l'audit
-- Tous les statements sont idempotents (IF NOT EXISTS).

-- remote_commands.audit_log_id : utilisé par CleanupStalledCommands et UpdateAuditLogStatus
-- Sans cet index, chaque suppression d'audit_logs provoque un full scan de remote_commands.
CREATE INDEX IF NOT EXISTS idx_commands_audit_log_id
    ON remote_commands(audit_log_id)
    WHERE audit_log_id IS NOT NULL;

-- release_tracker_tag_digests: accès par (tracker_id, created_at) pour le cleanup keepPerTracker
CREATE INDEX IF NOT EXISTS idx_rttd_tracker_created
    ON release_tracker_tag_digests(tracker_id, created_at DESC);

-- ===== END 035_missing_indexes.sql =====

-- ===== BEGIN 036_host_permissions.sql =====
-- Per-host access control: allows admins to restrict or grant users
-- specific access to individual hosts. If a user has NO entries in this
-- table their global role applies to all hosts (backward-compatible).
-- If they have ANY entries, they are restricted to only those hosts.
CREATE TABLE IF NOT EXISTS host_permissions (
    username   TEXT NOT NULL,
    host_id    TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    level      TEXT NOT NULL DEFAULT 'viewer'
                   CHECK (level IN ('viewer', 'operator')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (username, host_id)
);

CREATE INDEX IF NOT EXISTS idx_host_permissions_username ON host_permissions(username);
CREATE INDEX IF NOT EXISTS idx_host_permissions_host_id  ON host_permissions(host_id);

-- ===== END 036_host_permissions.sql =====

-- ===== BEGIN 037_bot_detection.sql =====
-- Migration 037: Add bot_detection JSONB cache column to hosts
-- Created: 2026-03-27

ALTER TABLE hosts
ADD COLUMN IF NOT EXISTS bot_detection JSONB;

CREATE INDEX IF NOT EXISTS idx_hosts_bot_detection
ON hosts ((bot_detection IS NOT NULL));

-- ===== END 037_bot_detection.sql =====

-- ===== BEGIN 038_npm_analytics.sql =====
-- Migration 038: Add npm_analytics JSONB cache column to hosts
-- Created: 2026-03-27

ALTER TABLE hosts
ADD COLUMN IF NOT EXISTS npm_analytics JSONB;

CREATE INDEX IF NOT EXISTS idx_hosts_npm_analytics
ON hosts ((npm_analytics IS NOT NULL));

-- ===== END 038_npm_analytics.sql =====

-- ===== BEGIN 039_web_logs.sql =====
-- Migration 039: Replace bot_detection/npm_analytics JSONB with relational web logs tables
-- Created: 2026-03-27

ALTER TABLE hosts
ADD COLUMN IF NOT EXISTS web_log_source TEXT,
ADD COLUMN IF NOT EXISTS web_log_collected_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS web_log_total_requests INTEGER NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS web_log_total_bytes BIGINT NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS web_log_errors_4xx INTEGER NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS web_log_errors_5xx INTEGER NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS web_log_suspicious_requests INTEGER NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS web_log_suspicious_ips INTEGER NOT NULL DEFAULT 0;

DROP INDEX IF EXISTS idx_hosts_bot_detection;
DROP INDEX IF EXISTS idx_hosts_npm_analytics;

ALTER TABLE hosts DROP COLUMN IF EXISTS bot_detection;
ALTER TABLE hosts DROP COLUMN IF EXISTS npm_analytics;

CREATE TABLE IF NOT EXISTS web_log_snapshots (
  id                  BIGSERIAL PRIMARY KEY,
  host_id             VARCHAR(64) NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
  captured_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  source              TEXT NOT NULL,
  total_requests      INTEGER NOT NULL DEFAULT 0,
  total_bytes         BIGINT NOT NULL DEFAULT 0,
  errors_4xx          INTEGER NOT NULL DEFAULT 0,
  errors_5xx          INTEGER NOT NULL DEFAULT 0,
  suspicious_requests INTEGER NOT NULL DEFAULT 0,
  suspicious_ips      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_web_log_snapshots_host_captured
ON web_log_snapshots (host_id, captured_at DESC);

CREATE INDEX IF NOT EXISTS idx_web_log_snapshots_host_source_captured
ON web_log_snapshots (host_id, source, captured_at DESC);

CREATE TABLE IF NOT EXISTS web_log_requests (
  id          BIGSERIAL PRIMARY KEY,
  snapshot_id BIGINT NOT NULL REFERENCES web_log_snapshots(id) ON DELETE CASCADE,
  host_id     VARCHAR(64) NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
  captured_at TIMESTAMPTZ NOT NULL,
  source      TEXT NOT NULL,
  ip          TEXT NOT NULL,
  method      TEXT NOT NULL,
  path        TEXT NOT NULL,
  status      INTEGER NOT NULL,
  bytes       BIGINT NOT NULL DEFAULT 0,
  user_agent  TEXT,
  domain      TEXT,
  category    TEXT,
  suspicious  BOOLEAN NOT NULL DEFAULT FALSE,
  fingerprint TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_web_log_requests_host_captured
ON web_log_requests (host_id, captured_at DESC);

CREATE INDEX IF NOT EXISTS idx_web_log_requests_ip_captured
ON web_log_requests (ip, captured_at DESC);

CREATE INDEX IF NOT EXISTS idx_web_log_requests_source_captured
ON web_log_requests (source, captured_at DESC);

CREATE INDEX IF NOT EXISTS idx_web_log_requests_suspicious_captured
ON web_log_requests (suspicious, captured_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS ux_web_log_requests_host_source_fingerprint
ON web_log_requests (host_id, source, fingerprint);

-- ===== END 039_web_logs.sql =====

-- ===== BEGIN 040_web_logs_dedup_fingerprint.sql =====
-- Migration 040: Add dedup fingerprint and unique key for web log requests
-- Goal: make ingestion idempotent across agent restarts and aggressive log rotation.

ALTER TABLE web_log_requests
ADD COLUMN IF NOT EXISTS fingerprint TEXT;

UPDATE web_log_requests
SET fingerprint = md5(CONCAT_WS('|',
  host_id::text,
  source,
  captured_at::text,
  ip,
  method,
  path,
  status::text,
  bytes::text,
  COALESCE(user_agent, ''),
  COALESCE(domain, ''),
  COALESCE(category, ''),
  suspicious::text
))
WHERE fingerprint IS NULL;

WITH ranked AS (
  SELECT
    id,
    ROW_NUMBER() OVER (
      PARTITION BY host_id, source, fingerprint
      ORDER BY id
    ) AS rn
  FROM web_log_requests
)
DELETE FROM web_log_requests w
USING ranked r
WHERE w.id = r.id
  AND r.rn > 1;

ALTER TABLE web_log_requests
ALTER COLUMN fingerprint SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_web_log_requests_host_source_fingerprint
ON web_log_requests (host_id, source, fingerprint);

-- ===== END 040_web_logs_dedup_fingerprint.sql =====

-- ===== BEGIN 041_cpu_temperature.sql =====
-- Add CPU temperature (Celsius) collected by the Linux agent when available.
ALTER TABLE system_metrics
ADD COLUMN IF NOT EXISTS cpu_temperature DOUBLE PRECISION;

-- ===== END 041_cpu_temperature.sql =====

-- ===== BEGIN 042_proxmox_cpu_temp_source.sql =====
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

-- ===== END 042_proxmox_cpu_temp_source.sql =====

-- Mark all legacy migration files as applied so they are skipped after this baseline.
INSERT INTO schema_migrations (filename) VALUES
    ('001_core.sql'),
    ('002_aggregates.sql'),
    ('003_docker.sql'),
    ('004_topology.sql'),
    ('005_disk.sql'),
    ('006_settings.sql'),
    ('007_alter_columns.sql'),
    ('008_remote_commands.sql'),
    ('009_alert_actions.sql'),
    ('010_timescaledb.sql'),
    ('011_refactor.sql'),
    ('012_container_netstats.sql'),
    ('013_alert_incidents_fk.sql'),
    ('014_scheduled_tasks.sql'),
    ('015_scheduled_task_link.sql'),
    ('016_git_webhooks.sql'),
    ('017_release_trackers.sql'),
    ('018_release_tracker_error.sql'),
    ('019_release_tracker_docker_image.sql'),
    ('020_image_digest.sql'),
    ('021_performance_indexes.sql'),
    ('022_ip_block_overrides.sql'),
    ('023_tracker_tag_digests.sql'),
    ('024_topology_positions.sql'),
    ('025_push_notifications.sql'),
    ('026_memory_percent_aggregate.sql'),
    ('027_proxmox.sql'),
    ('028_proxmox_links.sql'),
    ('029_proxmox_extended.sql'),
    ('030_repair_alert_rules.sql'),
    ('031_tracker_type.sql'),
    ('032_proxmox_node_metrics.sql'),
    ('033_proxmox_guest_metrics.sql'),
    ('034_tracker_monitor_only.sql'),
    ('035_missing_indexes.sql'),
    ('036_host_permissions.sql'),
    ('037_bot_detection.sql'),
    ('038_npm_analytics.sql'),
    ('039_web_logs.sql'),
    ('040_web_logs_dedup_fingerprint.sql'),
    ('041_cpu_temperature.sql'),
    ('042_proxmox_cpu_temp_source.sql')
ON CONFLICT DO NOTHING;
