-- Runbooks: named, reusable sequences of dispatch actions (the same
-- module/action/target/payload vocabulary already used by scheduled_tasks
-- and alert_rules' command_trigger), run one step at a time via the
-- existing dispatcher — see internal/services/runbook.
CREATE TABLE runbooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name character varying(255) NOT NULL,
    description text DEFAULT '' NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE runbook_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    runbook_id UUID NOT NULL REFERENCES runbooks(id) ON DELETE CASCADE,
    position integer NOT NULL,
    host_id character varying(64) NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    module character varying(50) NOT NULL,
    action character varying(100) NOT NULL,
    target character varying(255) DEFAULT '' NOT NULL,
    payload text DEFAULT '{}' NOT NULL,
    continue_on_failure boolean DEFAULT false NOT NULL,
    UNIQUE (runbook_id, position)
);

-- One row per run, advancing one step at a time as each step's remote_commands
-- row reaches a terminal state (see runbook.Service.NotifyComplete, hooked into
-- agentsvc.CommandCompletionListener like release trackers and git webhooks).
CREATE TABLE runbook_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    runbook_id UUID NOT NULL REFERENCES runbooks(id) ON DELETE CASCADE,
    status character varying(20) DEFAULT 'running' NOT NULL, -- running | completed | failed
    current_step_position integer DEFAULT 0 NOT NULL,
    triggered_by character varying(255) DEFAULT 'system' NOT NULL,
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone
);

-- Mirrors remote_commands.scheduled_task_id's existing shape exactly (same
-- nullable UUID FK pattern) so a runbook execution's step history is just
-- `SELECT * FROM remote_commands WHERE runbook_execution_id = $1 ORDER BY created_at`.
ALTER TABLE remote_commands ADD COLUMN runbook_execution_id UUID REFERENCES runbook_executions(id) ON DELETE SET NULL;

CREATE INDEX idx_runbook_steps_runbook ON runbook_steps(runbook_id, position);
CREATE INDEX idx_runbook_executions_runbook ON runbook_executions(runbook_id, started_at DESC);
CREATE INDEX idx_remote_commands_runbook_execution ON remote_commands(runbook_execution_id);
