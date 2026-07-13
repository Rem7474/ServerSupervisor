-- Opt-in GitOps-style drift reconciliation for compose-mode docker trackers:
-- when the registry digest hasn't changed (nothing new to deploy) but the
-- actually-running container has drifted from what this tracker last
-- deployed (manual change, silent failure, ...), the poller either
-- re-dispatches pull+up automatically (reconcile_drift=true) or leaves it
-- to be surfaced as a computed "drift_detected" flag on read (false,
-- default) — same opt-in shape as monitor-only trackers already use.
ALTER TABLE release_trackers ADD COLUMN reconcile_drift boolean DEFAULT false NOT NULL;
