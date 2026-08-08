-- Maintenance windows: a time range during which the alert engine suppresses
-- notifications for a host (host_id set) or every host (host_id NULL, a
-- global window) — see internal/alerts/engine.go's per-target maintenance
-- check and internal/services/maintenance. Any already-open incident for a
-- target entering a window is resolved silently, exactly like the existing
-- disabled-rule branch: no "resolved" notification either, since the goal is
-- zero noise during a planned intervention, not just zero new alerts.
CREATE TABLE maintenance_windows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    host_id character varying(64) REFERENCES hosts(id) ON DELETE CASCADE,
    reason text DEFAULT '' NOT NULL,
    starts_at timestamp with time zone NOT NULL,
    ends_at timestamp with time zone NOT NULL,
    created_by character varying(255) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT maintenance_windows_ends_after_starts CHECK (ends_at > starts_at)
);

CREATE INDEX idx_maintenance_windows_host ON maintenance_windows(host_id);
-- Supports the alert engine's per-target "is there a window covering now()"
-- lookup (internal/database/db_maintenance.go's IsHostInMaintenance).
CREATE INDEX idx_maintenance_windows_active ON maintenance_windows(host_id, ends_at);
