-- Allow Docker trackers to work in "monitor-only" mode without a linked host/task.
-- host_id becomes nullable and custom_task_id defaults to empty string.
ALTER TABLE release_trackers
    ALTER COLUMN host_id       DROP NOT NULL,
    ALTER COLUMN custom_task_id SET DEFAULT '';

-- Drop the FK constraint on host_id so it can be NULL
ALTER TABLE release_trackers
    DROP CONSTRAINT IF EXISTS release_trackers_host_id_fkey;

-- Re-add the FK as nullable (ON DELETE SET NULL)
ALTER TABLE release_trackers
    ADD CONSTRAINT release_trackers_host_id_fkey
        FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE SET NULL;
