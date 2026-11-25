-- Dynamic host groups and tag/group targeting for agent alert rules.
--
-- A host group stores a matching rule, never a list of hosts: membership is
-- computed on every read and every alert evaluation from hosts.tags (and an
-- optional OS substring), so a newly tagged host joins its groups without any
-- edit to the group or to the rules targeting it.
CREATE TABLE IF NOT EXISTS host_groups (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    match_mode  VARCHAR(8) NOT NULL DEFAULT 'all' CHECK (match_mode IN ('all', 'any')),
    tags        JSONB NOT NULL DEFAULT '[]'::jsonb,
    os_contains VARCHAR(100) NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- target_type selects how an agent rule picks its hosts:
--   host  : the single host in host_id
--   all   : every host
--   group : every current member of host_group_id
--   tag   : every host carrying at least one of target_tags
-- Empty for Docker/Proxmox rules, which carry their own scope.
-- A group referenced by a rule cannot be deleted (ON DELETE RESTRICT): a
-- silently emptied target would stop alerting without anyone noticing.
ALTER TABLE alert_rules
    ADD COLUMN IF NOT EXISTS target_type VARCHAR(10) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS host_group_id UUID REFERENCES host_groups(id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS target_tags JSONB NOT NULL DEFAULT '[]'::jsonb;

-- Existing agent rules keep their exact coverage: a rule with a host_id
-- targets that host, a rule without one targets every host.
UPDATE alert_rules
SET target_type = CASE WHEN COALESCE(host_id, '') <> '' THEN 'host' ELSE 'all' END
WHERE target_type = '' AND source_type = 'agent';

CREATE INDEX IF NOT EXISTS idx_alert_rules_host_group_id ON alert_rules (host_group_id) WHERE host_group_id IS NOT NULL;
