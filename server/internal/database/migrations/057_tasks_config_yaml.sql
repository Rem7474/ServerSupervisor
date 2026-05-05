-- Cache the raw tasks.yaml content reported by the agent
ALTER TABLE hosts
    ADD COLUMN IF NOT EXISTS tasks_config_yaml TEXT NOT NULL DEFAULT '';
