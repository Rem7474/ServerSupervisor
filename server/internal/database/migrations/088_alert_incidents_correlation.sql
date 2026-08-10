-- Alert incident correlation (ROADMAP.md item #5): when a host goes down, a
-- cascade of independent child alerts (one per Docker container that stops
-- reporting, etc.) used to each open their own incident and send their own
-- notification. correlated_with links a child incident to the host's own
-- open status_offline/heartbeat_timeout incident (see
-- internal/alerts/engine.go's maybeCorrelateWithHostDown) — the child
-- incident is still recorded (nothing hidden from the data model, still
-- visible grouped in the UI), it just skips the loud notification channels
-- and command_trigger, since the real cause is "host is down," not the
-- child rule's own condition. Computed once at incident-creation time, not
-- re-evaluated live if the parent later resolves.
ALTER TABLE alert_incidents
    ADD COLUMN correlated_with bigint REFERENCES alert_incidents(id) ON DELETE SET NULL;

CREATE INDEX idx_alert_incidents_correlated_with ON alert_incidents(correlated_with);
