-- Adds an activity heartbeat to remote_commands so the stalled-command reaper can
-- distinguish a still-alive long-running command from a genuinely dead agent.
--
-- Previously the cleanup keyed off created_at with a fixed 10-minute window, which
-- falsely failed legitimately long commands (e.g. a first apt update on a fresh host,
-- including CVE enrichment) while the agent was still working on them — the agent's
-- own absolute cap is 45 minutes. The reaper now keys off the most recent activity
-- (COALESCE(last_activity_at, started_at, created_at)); the server bumps
-- last_activity_at for a host's running commands on every report, so a command stays
-- alive as long as its agent keeps reporting.
--
-- Nullable, no backfill: existing rows fall through the COALESCE to started_at/created_at.
ALTER TABLE remote_commands
    ADD COLUMN last_activity_at TIMESTAMPTZ;
