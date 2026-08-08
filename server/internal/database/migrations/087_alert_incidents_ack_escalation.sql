-- Acknowledgment + escalation for alert incidents (ROADMAP.md items #3/#4).
-- Ack is orthogonal to open/resolved (resolved_at IS NULL stays the single
-- "is it open" predicate every existing query already relies on) rather than
-- a replacement status enum: an incident can be open+acknowledged,
-- open+unacknowledged, or resolved (ack no longer matters once resolved).
-- last_escalated_at tracks the last time the engine re-sent a notification
-- for an already-open, unacknowledged incident (internal/alerts/engine.go),
-- so it can space re-notifications by AlertActions.EscalateAfterMinutes
-- instead of re-firing on every evaluation tick.
ALTER TABLE alert_incidents
    ADD COLUMN acknowledged_at timestamp with time zone,
    ADD COLUMN acknowledged_by character varying(255),
    ADD COLUMN last_escalated_at timestamp with time zone;
