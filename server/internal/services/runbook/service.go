// Package runbook is the application/service layer for runbooks: named,
// reusable sequences of dispatch steps (the same module/action/target
// vocabulary already validated for alert rules' command_trigger and
// scheduled tasks), run one step at a time through the existing dispatcher.
// A step only ever names an already-whitelisted agent-side action — nothing
// here can hand an agent a new capability, matching the boundary documented
// in agent/CLAUDE.md ("the server can trigger a task by ID only").
package runbook

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
)

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	CreateRunbook(ctx context.Context, name, description string, steps []models.RunbookStepCreate) (*models.Runbook, error)
	GetRunbook(ctx context.Context, id string) (*models.Runbook, error)
	ListRunbooks(ctx context.Context) ([]models.Runbook, error)
	UpdateRunbook(ctx context.Context, id string, name, description *string, steps *[]models.RunbookStepCreate) error
	DeleteRunbook(ctx context.Context, id string) error
	HostExists(ctx context.Context, id string) (bool, error)

	CreateRunbookExecution(ctx context.Context, runbookID, triggeredBy string) (*models.RunbookExecution, error)
	GetRunbookExecution(ctx context.Context, id string) (*models.RunbookExecution, error)
	ListRunbookExecutions(ctx context.Context, runbookID string, limit int) ([]models.RunbookExecution, error)
	ListRunbookExecutionSteps(ctx context.Context, executionID string) ([]models.RunbookExecutionStep, error)
	GetRunbookStepByPosition(ctx context.Context, runbookID string, position int) (*models.RunbookStep, error)
	AdvanceRunbookExecution(ctx context.Context, executionID string, position int) error
	FinishRunbookExecution(ctx context.Context, executionID, status string) error
	LinkCommandToRunbookExecution(ctx context.Context, commandID, executionID string) error

	GetRemoteCommandByID(ctx context.Context, id string) (*models.RemoteCommand, error)
}

// Dispatcher is the agent-command port. *dispatch.Dispatcher satisfies it.
type Dispatcher interface {
	Create(ctx context.Context, req dispatch.Request) (*dispatch.Result, error)
}

type Service struct {
	repo       Repository
	dispatcher Dispatcher
}

func NewService(repo Repository, dispatcher Dispatcher) *Service {
	return &Service{repo: repo, dispatcher: dispatcher}
}

// ===== definitions =====

func (s *Service) List(ctx context.Context) ([]models.Runbook, error) {
	return s.repo.ListRunbooks(ctx)
}

func (s *Service) Get(ctx context.Context, id string) (*models.Runbook, error) {
	rb, err := s.repo.GetRunbook(ctx, id)
	if err == sql.ErrNoRows {
		return nil, apperr.NotFound("Runbook introuvable.")
	}
	if err != nil {
		return nil, err
	}
	return rb, nil
}

func (s *Service) Create(ctx context.Context, req models.RunbookCreate) (*models.Runbook, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperr.Validation("Le nom du runbook est requis.")
	}
	if err := s.validateSteps(ctx, req.Steps); err != nil {
		return nil, err
	}
	rb, err := s.repo.CreateRunbook(ctx, req.Name, req.Description, req.Steps)
	if err != nil {
		return nil, apperr.Failed("Erreur lors de la création du runbook.")
	}
	return rb, nil
}

func (s *Service) Update(ctx context.Context, id string, req models.RunbookUpdate) error {
	if _, err := s.repo.GetRunbook(ctx, id); err == sql.ErrNoRows {
		return apperr.NotFound("Runbook introuvable.")
	} else if err != nil {
		return err
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		return apperr.Validation("Le nom du runbook est requis.")
	}
	if req.Steps != nil {
		if err := s.validateSteps(ctx, *req.Steps); err != nil {
			return err
		}
	}
	if err := s.repo.UpdateRunbook(ctx, id, req.Name, req.Description, req.Steps); err != nil {
		return apperr.Failed("Erreur lors de la mise à jour du runbook.")
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.GetRunbook(ctx, id); err == sql.ErrNoRows {
		return apperr.NotFound("Runbook introuvable.")
	} else if err != nil {
		return err
	}
	return s.repo.DeleteRunbook(ctx, id)
}

// ===== execution =====

// Run creates an execution and dispatches its first step. Host authorization
// for each step's host is the caller's (HTTP) responsibility, same as
// scheduled tasks and alert command_trigger.
func (s *Service) Run(ctx context.Context, runbookID, triggeredBy string) (*models.RunbookExecution, error) {
	rb, err := s.repo.GetRunbook(ctx, runbookID)
	if err == sql.ErrNoRows {
		return nil, apperr.NotFound("Runbook introuvable.")
	}
	if err != nil {
		return nil, err
	}
	if len(rb.Steps) == 0 {
		return nil, apperr.Validation("Ce runbook n'a aucune étape.")
	}

	exec, err := s.repo.CreateRunbookExecution(ctx, runbookID, triggeredBy)
	if err != nil {
		return nil, apperr.Failed("Erreur lors du lancement du runbook.")
	}

	s.dispatchStep(ctx, exec.ID, rb.Steps[0])
	return exec, nil
}

func (s *Service) ListExecutions(ctx context.Context, runbookID string, limit int) ([]models.RunbookExecution, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.repo.ListRunbookExecutions(ctx, runbookID, limit)
}

// GetExecution returns an execution with its per-step outcomes.
func (s *Service) GetExecution(ctx context.Context, id string) (*models.RunbookExecution, error) {
	exec, err := s.repo.GetRunbookExecution(ctx, id)
	if err == sql.ErrNoRows {
		return nil, apperr.NotFound("Exécution introuvable.")
	}
	if err != nil {
		return nil, err
	}
	steps, err := s.repo.ListRunbookExecutionSteps(ctx, id)
	if err != nil {
		return nil, err
	}
	exec.Steps = steps
	return exec, nil
}

// NotifyComplete implements agentsvc.CommandCompletionListener: it advances
// or terminates a runbook execution when one of its steps' commands reaches
// a terminal state. commandID is checked against every completed command in
// the system — most won't be runbook steps at all, which is the normal case
// and returns silently.
func (s *Service) NotifyComplete(ctx context.Context, commandID, status string) {
	if status != "completed" && status != "failed" {
		return
	}

	cmd, err := s.repo.GetRemoteCommandByID(ctx, commandID)
	if err != nil || cmd.RunbookExecutionID == nil {
		return
	}
	executionID := *cmd.RunbookExecutionID

	exec, err := s.repo.GetRunbookExecution(ctx, executionID)
	if err != nil {
		slog.WarnContext(ctx, "runbook: could not load execution for completed step", "execution_id", executionID, "err", err)
		return
	}
	if exec.Status != "running" {
		return // already terminal — avoid acting twice on a duplicate/late completion event
	}

	step, err := s.repo.GetRunbookStepByPosition(ctx, exec.RunbookID, exec.CurrentStepPosition)
	if err != nil {
		_ = s.repo.FinishRunbookExecution(ctx, executionID, "failed")
		return
	}

	if status == "failed" && !step.ContinueOnFailure {
		_ = s.repo.FinishRunbookExecution(ctx, executionID, "failed")
		return
	}

	nextPosition := exec.CurrentStepPosition + 1
	nextStep, err := s.repo.GetRunbookStepByPosition(ctx, exec.RunbookID, nextPosition)
	if err == sql.ErrNoRows {
		_ = s.repo.FinishRunbookExecution(ctx, executionID, "completed")
		return
	}
	if err != nil {
		_ = s.repo.FinishRunbookExecution(ctx, executionID, "failed")
		return
	}

	if err := s.repo.AdvanceRunbookExecution(ctx, executionID, nextPosition); err != nil {
		slog.WarnContext(ctx, "runbook: failed to advance execution", "execution_id", executionID, "err", err)
		return
	}
	s.dispatchStep(ctx, executionID, *nextStep)
}

func (s *Service) dispatchStep(ctx context.Context, executionID string, step models.RunbookStep) {
	result, err := s.dispatcher.Create(ctx, dispatch.Request{
		HostID:      step.HostID,
		Module:      step.Module,
		Action:      step.Action,
		Target:      step.Target,
		Payload:     step.Payload,
		TriggeredBy: "runbook",
	})
	if err != nil {
		slog.ErrorContext(ctx, "runbook: failed to dispatch step", "execution_id", executionID, "position", step.Position, "err", err)
		_ = s.repo.FinishRunbookExecution(ctx, executionID, "failed")
		return
	}
	if err := s.repo.LinkCommandToRunbookExecution(ctx, result.Command.ID, executionID); err != nil {
		slog.WarnContext(ctx, "runbook: failed to link command to execution", "command_id", result.Command.ID, "execution_id", executionID, "err", err)
	}
}

// ===== validation =====

// commandModuleActions mirrors alertrule's whitelist for the same reason:
// a step only ever names an action already valid for that module — steps
// carry no shell content, no arbitrary payload beyond what dispatch.Request
// already accepts for any other trigger source (manual, scheduled, alert).
var commandModuleActions = map[string][]string{
	"docker":    {"logs", "restart", "start", "stop", "compose_up", "compose_down", "compose_pull", "compose_logs", "compose_restart"},
	"journal":   {"read"},
	"apt":       {"update", "upgrade", "full-upgrade", "autoremove"},
	"systemd":   {"status", "start", "stop", "restart", "list"},
	"processes": {"list"},
	"custom":    {"run"},
}

var commandModuleRequiresTarget = map[string]bool{
	"journal": true,
	"systemd": true,
	"custom":  true,
}

func (s *Service) validateSteps(ctx context.Context, steps []models.RunbookStepCreate) error {
	if len(steps) == 0 {
		return apperr.Validation("Le runbook doit avoir au moins une étape.")
	}
	for i, step := range steps {
		n := i + 1
		if strings.TrimSpace(step.HostID) == "" {
			return apperr.Validation(fmt.Sprintf("Étape %d : l'hôte est requis.", n))
		}
		if ok, err := s.repo.HostExists(ctx, step.HostID); err != nil {
			return apperr.Failed("Erreur lors de la validation des étapes.")
		} else if !ok {
			return apperr.Validation(fmt.Sprintf("Étape %d : hôte introuvable.", n))
		}
		allowedActions, ok := commandModuleActions[step.Module]
		if !ok {
			return apperr.Validation(fmt.Sprintf("Étape %d : module invalide (%s).", n, step.Module))
		}
		if !containsString(allowedActions, step.Action) {
			return apperr.Validation(fmt.Sprintf("Étape %d : action invalide pour le module %s (%s).", n, step.Module, step.Action))
		}
		if commandModuleRequiresTarget[step.Module] && strings.TrimSpace(step.Target) == "" {
			return apperr.Validation(fmt.Sprintf("Étape %d : le module %s requiert une cible.", n, step.Module))
		}
	}
	return nil
}

func containsString(values []string, expected string) bool {
	for _, v := range values {
		if v == expected {
			return true
		}
	}
	return false
}
