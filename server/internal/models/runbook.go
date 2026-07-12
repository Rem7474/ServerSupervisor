package models

import "time"

// ========== Runbooks ==========
// A runbook is a named, reusable sequence of dispatch steps (the same
// module/action/target/payload vocabulary already used by ScheduledTask and
// AlertActions.CommandTrigger), run one step at a time through the existing
// dispatcher. See internal/services/runbook.

type Runbook struct {
	ID          string        `json:"id" db:"id"`
	Name        string        `json:"name" db:"name"`
	Description string        `json:"description" db:"description"`
	CreatedAt   time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at" db:"updated_at"`
	Steps       []RunbookStep `json:"steps" db:"-"`
}

type RunbookStep struct {
	ID                string `json:"id" db:"id"`
	RunbookID         string `json:"runbook_id" db:"runbook_id"`
	Position          int    `json:"position" db:"position"`
	HostID            string `json:"host_id" db:"host_id"`
	Module            string `json:"module" db:"module"`
	Action            string `json:"action" db:"action"`
	Target            string `json:"target" db:"target"`
	Payload           string `json:"payload" db:"payload"`
	ContinueOnFailure bool   `json:"continue_on_failure" db:"continue_on_failure"`
}

// RunbookExecution is one run of a runbook, advancing one step at a time as
// each step's remote_commands row reaches a terminal state.
type RunbookExecution struct {
	ID                  string                 `json:"id" db:"id"`
	RunbookID           string                 `json:"runbook_id" db:"runbook_id"`
	Status              string                 `json:"status" db:"status"` // running | completed | failed
	CurrentStepPosition int                    `json:"current_step_position" db:"current_step_position"`
	TriggeredBy         string                 `json:"triggered_by" db:"triggered_by"`
	StartedAt           time.Time              `json:"started_at" db:"started_at"`
	CompletedAt         *time.Time             `json:"completed_at,omitempty" db:"completed_at"`
	RunbookName         string                 `json:"runbook_name,omitempty" db:"-"`
	Steps               []RunbookExecutionStep `json:"steps,omitempty" db:"-"`
}

// RunbookExecutionStep is one step's definition joined with the outcome of
// the remote_commands row it dispatched (empty Status/CommandID if this step
// hasn't been reached yet).
type RunbookExecutionStep struct {
	Position  int     `json:"position"`
	HostID    string  `json:"host_id"`
	Module    string  `json:"module"`
	Action    string  `json:"action"`
	Target    string  `json:"target"`
	CommandID *string `json:"command_id,omitempty"`
	Status    string  `json:"status,omitempty"`
	Output    string  `json:"output,omitempty"`
}

// ===== requests =====

type RunbookStepCreate struct {
	HostID            string `json:"host_id" binding:"required"`
	Module            string `json:"module" binding:"required"`
	Action            string `json:"action" binding:"required"`
	Target            string `json:"target"`
	Payload           string `json:"payload"`
	ContinueOnFailure bool   `json:"continue_on_failure"`
}

type RunbookCreate struct {
	Name        string              `json:"name" binding:"required"`
	Description string              `json:"description"`
	Steps       []RunbookStepCreate `json:"steps" binding:"required,min=1,dive"`
}

type RunbookUpdate struct {
	Name        *string              `json:"name"`
	Description *string              `json:"description"`
	Steps       *[]RunbookStepCreate `json:"steps"`
}
