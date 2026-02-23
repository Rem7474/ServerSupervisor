-- Migration: Enhance alert_rules table with advanced features
-- Adds support for rule naming, multi-channel notifications, cooldown periods, and tracking

-- Add new columns to existing alert_rules table
ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS name TEXT;
ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS channels JSONB DEFAULT '[]'::jsonb;
ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS smtp_to TEXT;
ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS ntfy_topic TEXT;
ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS cooldown INTEGER DEFAULT 3600;
ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS last_fired TIMESTAMP WITH TIME ZONE;
ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();

-- Update constraint for operator to include more options
DO $$ 
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'alert_rules_operator_check') THEN
        ALTER TABLE alert_rules DROP CONSTRAINT alert_rules_operator_check;
    END IF;
    ALTER TABLE alert_rules ADD CONSTRAINT alert_rules_operator_check 
        CHECK (operator IN ('>', '<', '>=', '<=', '==', '!='));
END $$;

-- Add indexes for new query patterns
CREATE INDEX IF NOT EXISTS idx_alert_rules_enabled ON alert_rules(enabled);
CREATE INDEX IF NOT EXISTS idx_alert_rules_metric ON alert_rules(metric);
CREATE INDEX IF NOT EXISTS idx_alert_rules_last_fired ON alert_rules(last_fired) WHERE last_fired IS NOT NULL;

-- Add comments for documentation
COMMENT ON COLUMN alert_rules.name IS 'Human-readable name for the alert rule';
COMMENT ON COLUMN alert_rules.channels IS 'Array of notification channels (e.g., ["smtp", "ntfy"])';
COMMENT ON COLUMN alert_rules.smtp_to IS 'Email recipients for SMTP notifications (comma-separated)';
COMMENT ON COLUMN alert_rules.ntfy_topic IS 'ntfy.sh topic name for push notifications';
COMMENT ON COLUMN alert_rules.cooldown IS 'Minimum seconds between consecutive alert notifications';
COMMENT ON COLUMN alert_rules.last_fired IS 'Timestamp when this rule last triggered an alert';
COMMENT ON COLUMN alert_rules.updated_at IS 'Last modification timestamp';
