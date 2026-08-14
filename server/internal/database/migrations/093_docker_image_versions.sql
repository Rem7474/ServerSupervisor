-- Ambient Docker image version cache: the single source of truth for "what is
-- the latest digest published for image X, tag Y".
--
-- Before this table, the only thing that ever asked a registry was the
-- release-tracker poller, once per manually-configured release_trackers row —
-- so a container with no tracker could never show a version badge, and two
-- trackers on the same image polled the registry twice. The background job in
-- internal/services/dockerversions now refreshes one row per distinct
-- (image, tag) actually running across every host (plus the image refs named
-- by docker trackers, so a tracker whose image is temporarily not running
-- keeps working), and both consumers read from here:
--   * ws.buildVersionComparisons — version badge for EVERY container.
--   * releasetracker.Poller.checkOneDocker — no longer calls the registry.
--
-- latest_tag is the resolved semver-looking tag for latest_digest when the
-- registry exposes one (e.g. "v5.13.2" behind a moving "v5"), else the polled
-- tag itself. last_error keeps the previous digest intact so a transient
-- registry failure degrades to "stale" rather than "unknown".
CREATE TABLE docker_image_versions (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    image text NOT NULL,
    image_tag text NOT NULL,
    registry text NOT NULL DEFAULT '',
    latest_digest text NOT NULL DEFAULT '',
    latest_tag text NOT NULL DEFAULT '',
    registry_credentials_id uuid REFERENCES registry_credentials(id) ON DELETE SET NULL,
    checked_at timestamp with time zone,
    last_error text NOT NULL DEFAULT '',
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT docker_image_versions_ref_key UNIQUE (image, image_tag)
);

-- The refresh job scans by staleness; the snapshot builders read the whole
-- (small) table, so the unique key above already covers point lookups.
CREATE INDEX idx_docker_image_versions_checked_at ON docker_image_versions (checked_at);
