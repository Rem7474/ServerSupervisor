-- Migration: Add per-host bot detection cache
-- Created: 2026-03-26

ALTER TABLE hosts
ADD COLUMN IF NOT EXISTS bot_detection JSONB;

CREATE INDEX IF NOT EXISTS idx_hosts_bot_detection
ON hosts ((bot_detection IS NOT NULL));
