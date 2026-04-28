-- unattended-upgrades status per host and upgrade run history
CREATE TABLE IF NOT EXISTS unattended_upgrades_status (
    host_id         VARCHAR(64) PRIMARY KEY REFERENCES hosts(id) ON DELETE CASCADE,
    installed       BOOLEAN     NOT NULL DEFAULT false,
    enabled         BOOLEAN     NOT NULL DEFAULT false,
    reboot_required BOOLEAN     NOT NULL DEFAULT false,
    last_run_at     TIMESTAMP WITH TIME ZONE,
    last_run_packages INTEGER   DEFAULT 0,
    config          JSONB       NOT NULL DEFAULT '{}',
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS unattended_upgrades_runs (
    id          BIGSERIAL   PRIMARY KEY,
    host_id     VARCHAR(64) NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    run_at      TIMESTAMP WITH TIME ZONE NOT NULL,
    packages    JSONB       NOT NULL DEFAULT '[]',
    had_error   BOOLEAN     NOT NULL DEFAULT false,
    log_snippet TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (host_id, run_at)
);

CREATE INDEX IF NOT EXISTS idx_uu_runs_host_run ON unattended_upgrades_runs(host_id, run_at DESC);
