-- Reusable alert rule templates (ROADMAP.md item #9): a metric/threshold/
-- actions "recipe" with no host, applied to N hosts at once
-- (POST /alert-rule-templates/:id/apply) to stamp out N independent
-- alert_rules rows — same "definition + per-target instances" shape as
-- runbooks/runbook_steps, not a live link: editing or deleting a template
-- never touches rules already created from it (each is a normal,
-- independently editable alert_rules row afterward). Agent metrics only —
-- Docker scope requires a host_id per rule and Proxmox scope is
-- cluster-level already, so neither fits "apply the same recipe to N hosts"
-- (enforced in internal/services/alertrule, not by a DB constraint here).
CREATE TABLE alert_rule_templates (
    id SERIAL PRIMARY KEY,
    name character varying(255) NOT NULL,
    metric character varying(100) NOT NULL,
    operator character varying(10) NOT NULL,
    threshold_warn double precision NOT NULL,
    threshold_crit double precision NOT NULL,
    threshold_clear_warn double precision,
    threshold_clear_crit double precision,
    duration_seconds integer DEFAULT 0 NOT NULL,
    actions jsonb DEFAULT '{}' NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);
