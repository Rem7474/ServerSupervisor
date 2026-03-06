// Package scheduler manages cron-based scheduled task execution.
// It maintains an in-memory map of cron.EntryID keyed by scheduled_task UUID,
// allowing tasks to be added, updated, and removed without restarting the server.
package scheduler

import (
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/serversupervisor/server/internal/models"
)

// DB is the subset of database.DB methods needed by the scheduler.
type DB interface {
	GetAllScheduledTasks() ([]models.ScheduledTask, error)
	CreateRemoteCommand(hostID, module, action, target, payload, triggeredBy string, auditLogID *int64) (*models.RemoteCommand, error)
	UpdateScheduledTaskRun(id, status string, lastRunAt, nextRunAt time.Time) error
	LinkCommandToScheduledTask(commandID, taskID string) error
}

// TaskScheduler runs scheduled tasks using robfig/cron.
type TaskScheduler struct {
	c    *cron.Cron
	db   DB
	jobs map[string]cron.EntryID // scheduled_task.id → cron entry
	mu   sync.Mutex
}

// New creates a TaskScheduler. Call Start() to begin scheduling.
func New(db DB) *TaskScheduler {
	return &TaskScheduler{
		c:    cron.New(), // standard 5-field cron expressions
		db:   db,
		jobs: make(map[string]cron.EntryID),
	}
}

// Start loads all enabled tasks from DB and registers them with the cron runner.
func (s *TaskScheduler) Start() {
	tasks, err := s.db.GetAllScheduledTasks()
	if err != nil {
		log.Printf("[scheduler] failed to load tasks: %v", err)
	} else {
		for _, t := range tasks {
			if err := s.add(t); err != nil {
				log.Printf("[scheduler] failed to register task %s (%s): %v", t.ID, t.Name, err)
			}
		}
		log.Printf("[scheduler] started with %d task(s)", len(tasks))
	}
	s.c.Start()
}

// Stop gracefully shuts down the cron runner.
func (s *TaskScheduler) Stop() {
	s.c.Stop()
}

// Add registers a new scheduled task. Safe to call concurrently.
func (s *TaskScheduler) Add(t models.ScheduledTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.add(t)
}

// Remove unregisters a scheduled task by its UUID.
func (s *TaskScheduler) Remove(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entryID, ok := s.jobs[taskID]; ok {
		s.c.Remove(entryID)
		delete(s.jobs, taskID)
	}
}

// Update replaces an existing cron entry (remove + re-add).
func (s *TaskScheduler) Update(t models.ScheduledTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entryID, ok := s.jobs[t.ID]; ok {
		s.c.Remove(entryID)
		delete(s.jobs, t.ID)
	}
	if !t.Enabled {
		return nil
	}
	return s.add(t)
}

// NextRun returns the next scheduled time for a task, or zero if not registered.
func (s *TaskScheduler) NextRun(taskID string) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	entryID, ok := s.jobs[taskID]
	if !ok {
		return time.Time{}
	}
	return s.c.Entry(entryID).Next
}

// add registers one task; caller must hold s.mu.
func (s *TaskScheduler) add(t models.ScheduledTask) error {
	entryID, err := s.c.AddFunc(t.CronExpression, s.makeJob(t))
	if err != nil {
		return err
	}
	s.jobs[t.ID] = entryID
	return nil
}

// makeJob returns the cron.FuncJob for a task.
func (s *TaskScheduler) makeJob(t models.ScheduledTask) func() {
	return func() {
		payload := t.Payload
		if payload == "" {
			payload = "{}"
		}
		cmd, err := s.db.CreateRemoteCommand(t.HostID, t.Module, t.Action, t.Target, payload, "scheduler", nil)
		if err != nil {
			log.Printf("[scheduler] task %s (%s): failed to create command: %v", t.ID, t.Name, err)
			now := time.Now()
			next := s.NextRun(t.ID)
			_ = s.db.UpdateScheduledTaskRun(t.ID, "failed", now, next)
			return
		}
		if err := s.db.LinkCommandToScheduledTask(cmd.ID, t.ID); err != nil {
			log.Printf("[scheduler] task %s: failed to link command: %v", t.ID, err)
		}
		now := time.Now()
		next := s.NextRun(t.ID)
		if err := s.db.UpdateScheduledTaskRun(t.ID, "pending", now, next); err != nil {
			log.Printf("[scheduler] task %s: failed to update run metadata: %v", t.ID, err)
		}
		log.Printf("[scheduler] task %s (%s): queued command %s on host %s", t.ID, t.Name, cmd.ID, t.HostID)
	}
}
