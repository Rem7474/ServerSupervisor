-- Add optional docker_image field to release_trackers for dashboard version comparison
ALTER TABLE release_trackers ADD COLUMN IF NOT EXISTS docker_image TEXT NOT NULL DEFAULT '';
