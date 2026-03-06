-- Migration 015: Link remote commands back to their originating scheduled task
-- Allows ReportCommandResult to propagate final status (completed/failed) to the task.

ALTER TABLE remote_commands
    ADD COLUMN IF NOT EXISTS scheduled_task_id UUID REFERENCES scheduled_tasks(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_remote_commands_scheduled_task
    ON remote_commands(scheduled_task_id) WHERE scheduled_task_id IS NOT NULL;

-- Cache the list of custom tasks declared in the agent's tasks.yaml
ALTER TABLE hosts
    ADD COLUMN IF NOT EXISTS custom_tasks JSONB NOT NULL DEFAULT '[]';

-- Clean up any stale scheduled tasks left in "pending" from before this fix.
-- A task stuck pending for more than 10 minutes is considered failed.
UPDATE scheduled_tasks
    SET last_run_status = 'failed', last_run_at = NOW()
    WHERE last_run_status = 'pending'
      AND last_run_at < NOW() - INTERVAL '10 minutes';
