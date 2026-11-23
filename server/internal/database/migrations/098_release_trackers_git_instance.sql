-- Self-hosted GitLab/Gitea support for release trackers: per-tracker instance
-- base URL (empty = the public instance) and access token. The token is never
-- returned by the API (see models.ReleaseTracker.GitToken).
ALTER TABLE release_trackers
    ADD COLUMN IF NOT EXISTS git_base_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS git_token TEXT NOT NULL DEFAULT '';
