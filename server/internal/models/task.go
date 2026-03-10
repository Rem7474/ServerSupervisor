package models

import "time"

// ========== Scheduled Tasks ==========

type ScheduledTask struct {
	ID             string     `json:"id"`
	HostID         string     `json:"host_id"`
	Name           string     `json:"name"`
	Module         string     `json:"module"`  // apt | docker | systemd | journal | processes | custom
	Action         string     `json:"action"`
	Target         string     `json:"target"`
	Payload        string     `json:"payload"` // JSON extra args
	CronExpression string     `json:"cron_expression"`
	Enabled        bool       `json:"enabled"`
	LastRunAt      *time.Time `json:"last_run_at"`
	NextRunAt      *time.Time `json:"next_run_at"`
	LastRunStatus  *string    `json:"last_run_status"` // completed | failed | nil
	LastCommandID  *string    `json:"last_command_id,omitempty"`
	CreatedBy      string     `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
}

// ScheduledTaskWithHost embeds ScheduledTask and adds host display name.
type ScheduledTaskWithHost struct {
	ScheduledTask
	HostName string `json:"host_name"`
}

type ScheduledTaskRequest struct {
	Name           string `json:"name" binding:"required"`
	Module         string `json:"module" binding:"required"`
	Action         string `json:"action" binding:"required"`
	Target         string `json:"target"`
	Payload        string `json:"payload"`
	CronExpression string `json:"cron_expression" binding:"required"`
	Enabled        bool   `json:"enabled"`
}

// ========== Custom Tasks ==========

// CustomTaskSummary is the lightweight representation of a custom task sent in
// agent reports so the server can display available tasks in the UI.
type CustomTaskSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
