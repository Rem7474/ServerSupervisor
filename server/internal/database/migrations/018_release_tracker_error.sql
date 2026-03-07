-- Add last_error to release_trackers to surface API/network errors
ALTER TABLE release_trackers ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '';
