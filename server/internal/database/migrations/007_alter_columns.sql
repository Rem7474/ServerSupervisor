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
