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
