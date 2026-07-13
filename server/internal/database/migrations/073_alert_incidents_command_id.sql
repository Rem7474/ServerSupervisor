-- Links an alert incident to the remote_commands row its command_trigger
-- dispatched (if any), so the incident's remediation outcome (pending/
-- running/completed/failed) can be surfaced without a separate lookup.
ALTER TABLE alert_incidents ADD COLUMN command_id character varying(36) REFERENCES remote_commands(id) ON DELETE SET NULL;
