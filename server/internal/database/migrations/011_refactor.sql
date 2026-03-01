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
