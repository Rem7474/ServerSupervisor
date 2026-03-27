-- Migration 037: Add bot_detection JSONB cache column to hosts
-- Created: 2026-03-27

ALTER TABLE hosts
ADD COLUMN IF NOT EXISTS bot_detection JSONB;

CREATE INDEX IF NOT EXISTS idx_hosts_bot_detection
ON hosts ((bot_detection IS NOT NULL));
