-- Drop legacy tracked_repos table.
-- Replaced by release_trackers + release_tracker_executions (migration 017+).
-- The owning code (releasetracker.Tracker poller, DockerHandler CRUD handlers,
-- db CreateTrackedRepo/GetTrackedRepos/UpdateTrackedRepo/DeleteTrackedRepo) has
-- been removed; this migration drops the now-unused table.

DROP TABLE IF EXISTS tracked_repos;
