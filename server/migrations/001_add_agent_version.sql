-- Migration: Add agent_version column to hosts table
-- Date: 2026-02-21
-- Description: Track the version of the agent running on each host

-- Add agent_version column (nullable for backward compatibility)
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS agent_version VARCHAR(20) DEFAULT NULL;

-- Create index for faster lookups by version
CREATE INDEX IF NOT EXISTS idx_hosts_agent_version ON hosts(agent_version);

-- Optional: Update existing hosts to show version as 'unknown' instead of NULL
-- UPDATE hosts SET agent_version = 'unknown' WHERE agent_version IS NULL;
