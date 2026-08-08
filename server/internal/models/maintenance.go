package models

import "time"

// MaintenanceWindow suppresses alert notifications for a host (HostID set)
// or every host (HostID nil, a global window) between StartsAt and EndsAt.
// The alert engine (internal/alerts) checks for an active window before
// evaluating an evaluation target and silently resolves any already-open
// incident without notifying, exactly like a disabled rule — see
// internal/alerts/engine.go and internal/database/db_maintenance.go's
// IsHostInMaintenance.
type MaintenanceWindow struct {
	ID        string    `json:"id"`
	HostID    *string   `json:"host_id"` // nil = applies to every host
	HostName  *string   `json:"host_name,omitempty"`
	Reason    string    `json:"reason"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

type MaintenanceWindowRequest struct {
	Reason   string    `json:"reason" binding:"required"`
	StartsAt time.Time `json:"starts_at" binding:"required"`
	EndsAt   time.Time `json:"ends_at" binding:"required"`
}
