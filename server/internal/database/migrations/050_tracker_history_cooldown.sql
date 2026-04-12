-- Enable release tracker cooldown and keep Docker tag history per digest (including latest updates).

ALTER TABLE release_trackers
  ADD COLUMN IF NOT EXISTS cooldown_hours INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS last_release_detected_at TIMESTAMPTZ;

-- Previous schema used PRIMARY KEY (tracker_id, tag), which kept only one row per tag.
-- Switch to (tracker_id, tag, digest) to retain successive latest/tag updates over time.
ALTER TABLE release_tracker_tag_digests
  DROP CONSTRAINT IF EXISTS release_tracker_tag_digests_pkey;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'release_tracker_tag_digests_pkey'
      AND conrelid = 'release_tracker_tag_digests'::regclass
  ) THEN
    ALTER TABLE release_tracker_tag_digests
      ADD CONSTRAINT release_tracker_tag_digests_pkey PRIMARY KEY (tracker_id, tag, digest);
  END IF;
END $$;
