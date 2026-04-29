ALTER TABLE web_log_snapshots ADD COLUMN IF NOT EXISTS crowdsec_blocked_ips INT NOT NULL DEFAULT 0;
