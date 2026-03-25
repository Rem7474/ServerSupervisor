-- Migration: Add per-host NPM analytics cache
-- Created: 2026-03-25

ALTER TABLE hosts
ADD COLUMN IF NOT EXISTS npm_analytics JSONB;

CREATE INDEX IF NOT EXISTS idx_hosts_npm_analytics
ON hosts ((npm_analytics IS NOT NULL));
