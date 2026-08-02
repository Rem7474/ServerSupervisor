-- Restic backup supervision: history of runs dispatched via
-- module=restic action=run_backup (manual trigger from the UI or a
-- scheduled task, see scheduledtask.Service's validModules), plus the
-- latest passive status snapshot reported periodically by the agent
-- (restic_status, same shape/pattern as apt_status). Never stores
-- restic/Swift/SMTP credentials — only run outcomes and repo facts.
CREATE TABLE backup_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    host_id character varying(64) NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    profile character varying(255) DEFAULT '' NOT NULL,
    command_id character varying(36) REFERENCES remote_commands(id) ON DELETE SET NULL,
    triggered_by character varying(255) DEFAULT '' NOT NULL,
    status character varying(20) DEFAULT 'running' NOT NULL, -- running | ok | warning | error
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    finished_at timestamp with time zone,
    duration_sec integer,
    progress_percent double precision,
    files_done bigint,
    files_total bigint,
    bytes_done bigint,
    bytes_total bigint,
    snapshot_id character varying(64) DEFAULT '' NOT NULL,
    snapshot_time timestamp with time zone,
    repo_size_bytes bigint,
    error_message text DEFAULT '' NOT NULL,
    raw_summary text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE UNIQUE INDEX idx_backup_runs_command_id ON backup_runs(command_id) WHERE command_id IS NOT NULL;
CREATE INDEX idx_backup_runs_host_started ON backup_runs(host_id, started_at DESC);

-- Passive, periodic snapshot of Restic's state (status-file or restic
-- snapshots/stats fallback), reported by the agent's regular collector cycle
-- and refreshed out-of-band right after a manual run_backup completes.
-- Falls back for hosts whose backups aren't (yet) dispatched by
-- ServerSupervisor at all.
CREATE TABLE restic_status (
    host_id character varying(64) PRIMARY KEY REFERENCES hosts(id) ON DELETE CASCADE,
    installed boolean DEFAULT false NOT NULL,
    last_run_at timestamp with time zone,
    last_status character varying(20) DEFAULT '' NOT NULL,
    snapshot_id character varying(64) DEFAULT '' NOT NULL,
    repo_size_bytes bigint,
    error_message text DEFAULT '' NOT NULL,
    source character varying(20) DEFAULT '' NOT NULL, -- status_file | restic_commands | unavailable
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);
