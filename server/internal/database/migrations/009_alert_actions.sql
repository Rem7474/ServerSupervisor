-- Consolidate alert_rules notification config into a single actions JSONB column
-- (Sprint 3c: replaces channel, channel_config, channels, smtp_to, ntfy_topic, cooldown)

ALTER TABLE IF EXISTS alert_rules ADD COLUMN IF NOT EXISTS actions JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE alert_rules SET actions = jsonb_build_object(
    'channels', COALESCE(channels, '[]'::jsonb),
    'smtp_to', COALESCE(smtp_to, ''),
    'ntfy_topic', COALESCE(ntfy_topic, ''),
    'cooldown', COALESCE(cooldown, 0)
) WHERE actions = '{}'::jsonb OR actions IS NULL;

ALTER TABLE IF EXISTS alert_rules DROP COLUMN IF EXISTS channel;

ALTER TABLE IF EXISTS alert_rules DROP COLUMN IF EXISTS channel_config;

ALTER TABLE IF EXISTS alert_rules DROP COLUMN IF EXISTS channels;

ALTER TABLE IF EXISTS alert_rules DROP COLUMN IF EXISTS smtp_to;

ALTER TABLE IF EXISTS alert_rules DROP COLUMN IF EXISTS ntfy_topic;

ALTER TABLE IF EXISTS alert_rules DROP COLUMN IF EXISTS cooldown;
