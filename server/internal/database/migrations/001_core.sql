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
