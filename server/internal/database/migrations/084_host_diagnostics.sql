-- Cached list of config-vs-reality diagnostic issues found by the agent's
-- own CheckConfig on its last report (e.g. collect_restic: true but
-- resticconf missing) — mirrors restic_profiles/restic_groups's cache
-- pattern, surfaced as a warning banner on the host detail page.
ALTER TABLE hosts ADD COLUMN diagnostics jsonb DEFAULT '[]'::jsonb NOT NULL;
