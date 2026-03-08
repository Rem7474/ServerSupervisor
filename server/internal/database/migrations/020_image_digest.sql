-- Add image digest for running container (RepoDigest / manifest sha256)
ALTER TABLE docker_containers ADD COLUMN IF NOT EXISTS image_digest TEXT NOT NULL DEFAULT '';

-- Add latest image digest for release tracker (manifest sha256 of latest release tag)
ALTER TABLE release_trackers ADD COLUMN IF NOT EXISTS latest_image_digest TEXT NOT NULL DEFAULT '';
