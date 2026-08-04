-- Cached list of resticprofile.yaml profile names reported by the agent
-- (mirrors custom_tasks: agent-local file, read locally, only names sent up)
-- so the backup/scheduled-task UIs can offer a picker instead of free text.
ALTER TABLE hosts ADD COLUMN restic_profiles jsonb DEFAULT '[]'::jsonb NOT NULL;
