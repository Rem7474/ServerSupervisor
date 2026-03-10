package models

import "time"

// ========== Remote Commands (unified: docker | apt | systemd | journal) ==========

// RemoteCommand represents any task dispatched to a remote agent.
// module ∈ "docker" | "apt" | "systemd" | "journal"
type RemoteCommand struct {
	ID              string     `json:"id" db:"id"` // UUID v4
	HostID          string     `json:"host_id" db:"host_id"`
	Module          string     `json:"module" db:"module"`   // docker | apt | systemd | journal
	Action          string     `json:"action" db:"action"`   // start, stop, upgrade, logs, list, …
	Target          string     `json:"target" db:"target"`   // container / service name; empty for apt
	Payload         string     `json:"payload" db:"payload"` // JSON extra args (e.g. {"working_dir": "…"})
	Status          string     `json:"status" db:"status"`   // pending | running | completed | failed
	Output          string     `json:"output" db:"output"`
	TriggeredBy     string     `json:"triggered_by" db:"triggered_by"`
	AuditLogID      *int64     `json:"audit_log_id,omitempty" db:"audit_log_id"`
	ScheduledTaskID *string    `json:"scheduled_task_id,omitempty" db:"scheduled_task_id"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	StartedAt       *time.Time `json:"started_at" db:"started_at"`
	EndedAt         *time.Time `json:"ended_at" db:"ended_at"`
}

type DockerCommandRequest struct {
	HostID        string `json:"host_id" binding:"required"`
	ContainerName string `json:"container_name" binding:"required"`
	Action        string `json:"action" binding:"required,oneof=start stop restart logs compose_up compose_down compose_restart compose_logs"`
	WorkingDir    string `json:"working_dir"` // required for compose_* actions
}

// ========== Commands (server → agent) ==========

type PendingCommand struct {
	ID      string `json:"id"`      // UUID
	Module  string `json:"module"`  // docker | apt | systemd | journal
	Action  string `json:"action"`  // start, stop, upgrade, logs, list, …
	Target  string `json:"target"`  // container / service name; empty for apt
	Payload string `json:"payload"` // JSON extra args
}

type CommandResult struct {
	CommandID string     `json:"command_id"` // UUID
	Status    string     `json:"status"`     // running | completed | failed
	Output    string     `json:"output"`
	AptStatus *AptStatus `json:"apt_status,omitempty"` // Full APT status after update/upgrade
}

// ========== Audit Log (APT & Admin Actions) ==========

type AuditLog struct {
	ID        int64     `json:"id" db:"id"`
	Username  string    `json:"username" db:"username"`     // Who
	Action    string    `json:"action" db:"action"`         // What (apt_update, apt_upgrade, user_created, etc.)
	HostID    string    `json:"host_id" db:"host_id"`       // On which host (nullable)
	HostName  string    `json:"host_name" db:"host_name"`   // Display name (if available)
	IPAddress string    `json:"ip_address" db:"ip_address"` // Client IP
	Details   string    `json:"details" db:"details"`       // JSON payload (command output, new privileges, etc.)
	Status    string    `json:"status" db:"status"`         // pending, completed, failed
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
