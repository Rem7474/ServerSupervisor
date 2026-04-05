-- Add collectors field to hosts table to track which metrics sources are available
-- This enables context-aware alert metric selection: show only metrics available on the selected host

ALTER TABLE hosts
ADD COLUMN IF NOT EXISTS collectors JSONB DEFAULT '{
  "docker": false,
  "apt": false,
  "smart": false,
  "cpu_temp": false,
  "web_logs": false,
  "systemd": false,
  "journal": false
}'::jsonb;

-- Index for querying hosts by collector availability
CREATE INDEX IF NOT EXISTS idx_hosts_collectors ON hosts USING GIN (collectors);
