-- Web Push subscriptions: one row per browser/device per user.
-- endpoint is globally unique — each subscription identifies one browser install.
CREATE TABLE IF NOT EXISTS push_subscriptions (
    id          SERIAL PRIMARY KEY,
    username    TEXT NOT NULL,
    endpoint    TEXT NOT NULL,
    p256dh_key  TEXT NOT NULL,
    auth_key    TEXT NOT NULL,
    user_agent  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(endpoint)
);

-- Server-side "read up to" timestamp per user.
-- Replaces per-device localStorage so marking as read on PC1 syncs to PC2/mobile.
CREATE TABLE IF NOT EXISTS notification_read_at (
    username TEXT PRIMARY KEY,
    read_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
