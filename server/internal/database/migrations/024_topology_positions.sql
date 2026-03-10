-- Add node_positions to persist graph layout across sessions
ALTER TABLE network_topology_config
  ADD COLUMN IF NOT EXISTS node_positions JSONB DEFAULT '{}'::jsonb;
