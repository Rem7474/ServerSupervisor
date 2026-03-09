-- Historical digest→tag mapping for release trackers.
-- Each time the poller detects a new release and fetches its manifest digest,
-- it stores the (tag, digest) pair here. This allows resolving the running
-- version of a container using :latest by matching its image digest.
CREATE TABLE IF NOT EXISTS release_tracker_tag_digests (
    tracker_id  UUID    NOT NULL REFERENCES release_trackers(id) ON DELETE CASCADE,
    tag         TEXT    NOT NULL,
    digest      TEXT    NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tracker_id, tag)
);

CREATE INDEX IF NOT EXISTS idx_rttd_tracker_digest ON release_tracker_tag_digests (tracker_id, digest);
