ALTER TABLE web_log_snapshots ADD COLUMN IF NOT EXISTS crowdsec_top_blocked JSONB NOT NULL DEFAULT '[]';
