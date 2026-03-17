-- Add tracker_type to distinguish Git release trackers from Docker image trackers.
-- tracker_type: 'git' (default, existing rows) | 'docker'
-- docker_tag: the specific image tag to monitor for docker trackers (e.g., 'latest', 'stable')

ALTER TABLE release_trackers
    ADD COLUMN IF NOT EXISTS tracker_type TEXT NOT NULL DEFAULT 'git',
    ADD COLUMN IF NOT EXISTS docker_tag TEXT NOT NULL DEFAULT '';
