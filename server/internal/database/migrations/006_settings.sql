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
