// Package scheduledtask is the application/service layer for host scheduled tasks.
// It owns the task business logic (validation, cron scheduling coordination, manual
// run dispatch) behind three ports — a Repository, the cron Scheduler and the
// command Dispatcher — so the orchestration is unit-testable with fakes and the
// HTTP handler is reduced to request/response translation + authorization.
package scheduledtask

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
)

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	GetGlobalScheduledTasks(ctx context.Context) ([]models.ScheduledTaskWithHost, error)
	GetScheduledTasks(ctx context.Context, hostID string) ([]models.ScheduledTask, error)
	GetScheduledTask(ctx context.Context, id string) (*models.ScheduledTask, error)
	CreateScheduledTask(ctx context.Context, t models.ScheduledTask) (*models.ScheduledTask, error)
	UpdateScheduledTask(ctx context.Context, id string, t models.ScheduledTask) error
	DeleteScheduledTask(ctx context.Context, id string) error
	SetScheduledTaskNextRun(ctx context.Context, id string, next time.Time) error
	GetHostCustomTasks(ctx context.Context, hostID string) (string, error)
	GetHostTasksConfigYAML(ctx context.Context, hostID string) (string, error)
	GetScheduledTaskExecutions(ctx context.Context, id string, limit int) ([]models.RemoteCommand, error)
	LinkCommandToScheduledTask(ctx context.Context, commandID, taskID string) error
	UpdateScheduledTaskStatus(ctx context.Context, id, status string) error
}

// Scheduler is the cron-scheduling port. *scheduler.TaskScheduler satisfies it.
type Scheduler interface {
	Add(t models.ScheduledTask) error
	Update(t models.ScheduledTask) error
	Remove(id string)
	NextRun(id string) time.Time
}

// Dispatcher is the agent-command port. *dispatch.Dispatcher satisfies it.
type Dispatcher interface {
	Create(ctx context.Context, req dispatch.Request) (*dispatch.Result, error)
}

// Service holds the scheduled-task use-cases.
type Service struct {
	repo       Repository
	sched      Scheduler
	dispatcher Dispatcher
}

func NewService(repo Repository, sched Scheduler, dispatcher Dispatcher) *Service {
	return &Service{repo: repo, sched: sched, dispatcher: dispatcher}
}

var validModules = map[string]bool{
	"apt": true, "docker": true, "systemd": true,
	"journal": true, "processes": true, "custom": true,
}

// validate checks the request and normalizes its payload. Returns apperr.Validation
// on a bad module or cron expression.
func validate(req *models.ScheduledTaskRequest) error {
	if !validModules[req.Module] {
		return apperr.Validation("invalid module: " + req.Module)
	}
	if req.CronExpression != "" {
		if _, err := cron.ParseStandard(req.CronExpression); err != nil {
			return apperr.Validation("expression cron invalide : " + err.Error())
		}
	}
	if req.Payload == "" {
		req.Payload = "{}"
	}
	return nil
}

// ListAll returns every scheduled task across hosts (never nil).
func (s *Service) ListAll(ctx context.Context) ([]models.ScheduledTaskWithHost, error) {
	tasks, err := s.repo.GetGlobalScheduledTasks(ctx)
	if err != nil {
		return nil, err
	}
	if tasks == nil {
		tasks = []models.ScheduledTaskWithHost{}
	}
	return tasks, nil
}

// ListForHost returns a host's scheduled tasks (never nil).
func (s *Service) ListForHost(ctx context.Context, hostID string) ([]models.ScheduledTask, error) {
	tasks, err := s.repo.GetScheduledTasks(ctx, hostID)
	if err != nil {
		return nil, err
	}
	if tasks == nil {
		tasks = []models.ScheduledTask{}
	}
	return tasks, nil
}

// Get returns a task by id, or apperr.NotFound when it is absent.
func (s *Service) Get(ctx context.Context, id string) (*models.ScheduledTask, error) {
	t, err := s.repo.GetScheduledTask(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("task not found")
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

// Create validates the request, persists the task and, when enabled, registers it
// with the scheduler and stores its next run.
func (s *Service) Create(ctx context.Context, hostID, username string, req models.ScheduledTaskRequest) (*models.ScheduledTask, error) {
	if err := validate(&req); err != nil {
		return nil, err
	}
	created, err := s.repo.CreateScheduledTask(ctx, models.ScheduledTask{
		HostID:         hostID,
		Name:           req.Name,
		Module:         req.Module,
		Action:         req.Action,
		Target:         req.Target,
		Payload:        req.Payload,
		CronExpression: req.CronExpression,
		Enabled:        req.Enabled,
		CreatedBy:      username,
	})
	if err != nil {
		return nil, err
	}
	if created.Enabled {
		s.schedule(ctx, *created)
		if next := s.sched.NextRun(created.ID); !next.IsZero() {
			created.NextRunAt = &next
		}
	}
	return created, nil
}

// Update validates the request, applies it to the task and re-registers it with
// the scheduler, returning the stored result.
func (s *Service) Update(ctx context.Context, id string, req models.ScheduledTaskRequest) (*models.ScheduledTask, error) {
	if err := validate(&req); err != nil {
		return nil, err
	}
	existing, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	updated := models.ScheduledTask{
		ID:             id,
		HostID:         existing.HostID,
		Name:           req.Name,
		Module:         req.Module,
		Action:         req.Action,
		Target:         req.Target,
		Payload:        req.Payload,
		CronExpression: req.CronExpression,
		Enabled:        req.Enabled,
	}
	if err := s.repo.UpdateScheduledTask(ctx, id, updated); err != nil {
		return nil, err
	}
	if err := s.sched.Update(updated); err != nil {
		slog.WarnContext(ctx, "scheduler update failed", slog.String("task_id", id), slog.Any("err", err))
	} else if req.Enabled {
		if next := s.sched.NextRun(id); !next.IsZero() {
			_ = s.repo.SetScheduledTaskNextRun(ctx, id, next)
		}
	}
	if fresh, err := s.repo.GetScheduledTask(ctx, id); err == nil {
		return fresh, nil
	}
	return &updated, nil
}

// Delete unregisters the task from the scheduler and removes it.
func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	s.sched.Remove(id)
	return s.repo.DeleteScheduledTask(ctx, id)
}

// CustomTasks returns the host's agent-reported custom tasks (raw JSON).
func (s *Service) CustomTasks(ctx context.Context, hostID string) (string, error) {
	return s.repo.GetHostCustomTasks(ctx, hostID)
}

// TasksYAML returns the host's cached tasks.yaml content.
func (s *Service) TasksYAML(ctx context.Context, hostID string) (string, error) {
	return s.repo.GetHostTasksConfigYAML(ctx, hostID)
}

// Executions returns the last N executions for a task (never nil).
func (s *Service) Executions(ctx context.Context, id string, limit int) ([]models.RemoteCommand, error) {
	cmds, err := s.repo.GetScheduledTaskExecutions(ctx, id, limit)
	if err != nil {
		return nil, err
	}
	if cmds == nil {
		cmds = []models.RemoteCommand{}
	}
	return cmds, nil
}

// Run dispatches the task to its host immediately, links the resulting command to
// the task and marks it pending. Returns the command id. Host authorization is the
// caller's (HTTP) responsibility.
func (s *Service) Run(ctx context.Context, task models.ScheduledTask, username string) (string, error) {
	payload := task.Payload
	if payload == "" {
		payload = "{}"
	}
	result, err := s.dispatcher.Create(ctx, dispatch.Request{
		HostID:      task.HostID,
		Module:      task.Module,
		Action:      task.Action,
		Target:      task.Target,
		Payload:     payload,
		TriggeredBy: username,
	})
	if err != nil {
		return "", err
	}
	if err := s.repo.LinkCommandToScheduledTask(ctx, result.Command.ID, task.ID); err != nil {
		slog.ErrorContext(ctx, "failed to link command to scheduled task",
			slog.String("command_id", result.Command.ID), slog.String("task_id", task.ID), slog.Any("err", err))
	}
	_ = s.repo.UpdateScheduledTaskStatus(ctx, task.ID, "pending")
	return result.Command.ID, nil
}

// schedule registers an enabled task and stores its next run; cron is pre-validated
// so an Add error is unexpected and only logged (the task is already persisted).
func (s *Service) schedule(ctx context.Context, t models.ScheduledTask) {
	if err := s.sched.Add(t); err != nil {
		slog.WarnContext(ctx, "scheduler add failed", slog.String("task_id", t.ID), slog.Any("err", err))
		return
	}
	if next := s.sched.NextRun(t.ID); !next.IsZero() {
		_ = s.repo.SetScheduledTaskNextRun(ctx, t.ID, next)
	}
}
