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
