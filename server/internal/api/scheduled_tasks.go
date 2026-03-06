package api

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/scheduler"
)

type ScheduledTaskHandler struct {
	db        *database.DB
	cfg       *config.Config
	scheduler *scheduler.TaskScheduler
}

func NewScheduledTaskHandler(db *database.DB, cfg *config.Config, sched *scheduler.TaskScheduler) *ScheduledTaskHandler {
	return &ScheduledTaskHandler{db: db, cfg: cfg, scheduler: sched}
}

var validTaskModules = map[string]bool{
	"apt": true, "docker": true, "systemd": true,
	"journal": true, "processes": true, "custom": true,
}

// ListScheduledTasks returns all scheduled tasks for a host.
func (h *ScheduledTaskHandler) ListScheduledTasks(c *gin.Context) {
	hostID := c.Param("id")
	tasks, err := h.db.GetScheduledTasks(hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if tasks == nil {
		tasks = []models.ScheduledTask{}
	}
	c.JSON(http.StatusOK, tasks)
}

// CreateScheduledTask creates a new scheduled task for a host.
func (h *ScheduledTaskHandler) CreateScheduledTask(c *gin.Context) {
	hostID := c.Param("id")
	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}

	var req models.ScheduledTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validTaskModules[req.Module] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid module: " + req.Module})
		return
	}
	if req.Payload == "" {
		req.Payload = "{}"
	}

	task := models.ScheduledTask{
		HostID:         hostID,
		Name:           req.Name,
		Module:         req.Module,
		Action:         req.Action,
		Target:         req.Target,
		Payload:        req.Payload,
		CronExpression: req.CronExpression,
		Enabled:        req.Enabled,
		CreatedBy:      username,
	}

	created, err := h.db.CreateScheduledTask(task)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if created.Enabled {
		if err := h.scheduler.Add(*created); err != nil {
			c.JSON(http.StatusCreated, gin.H{"task": created, "warning": "invalid cron expression: " + err.Error()})
			return
		}
		if next := h.scheduler.NextRun(created.ID); !next.IsZero() {
			_ = h.db.SetScheduledTaskNextRun(created.ID, next)
			created.NextRunAt = &next
		}
	}

	c.JSON(http.StatusCreated, created)
}

// UpdateScheduledTask modifies an existing scheduled task.
func (h *ScheduledTaskHandler) UpdateScheduledTask(c *gin.Context) {
	taskID := c.Param("id")

	var req models.ScheduledTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validTaskModules[req.Module] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid module: " + req.Module})
		return
	}
	if req.Payload == "" {
		req.Payload = "{}"
	}

	existing, err := h.db.GetScheduledTask(taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	updated := models.ScheduledTask{
		ID:             taskID,
		HostID:         existing.HostID,
		Name:           req.Name,
		Module:         req.Module,
		Action:         req.Action,
		Target:         req.Target,
		Payload:        req.Payload,
		CronExpression: req.CronExpression,
		Enabled:        req.Enabled,
	}

	if err := h.db.UpdateScheduledTask(taskID, updated); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.scheduler.Update(updated); err != nil {
		c.JSON(http.StatusOK, gin.H{"task": updated, "warning": "invalid cron expression: " + err.Error()})
		return
	}

	if req.Enabled {
		if next := h.scheduler.NextRun(taskID); !next.IsZero() {
			_ = h.db.SetScheduledTaskNextRun(taskID, next)
		}
	}

	if t, err := h.db.GetScheduledTask(taskID); err == nil {
		c.JSON(http.StatusOK, t)
	} else {
		c.JSON(http.StatusOK, updated)
	}
}

// DeleteScheduledTask removes a scheduled task.
func (h *ScheduledTaskHandler) DeleteScheduledTask(c *gin.Context) {
	taskID := c.Param("id")
	if _, err := h.db.GetScheduledTask(taskID); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	h.scheduler.Remove(taskID)
	if err := h.db.DeleteScheduledTask(taskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// GetCustomTasks returns the list of available custom tasks for a host,
// as reported by the agent from its local tasks.yaml.
func (h *ScheduledTaskHandler) GetCustomTasks(c *gin.Context) {
	hostID := c.Param("id")
	tasksJSON, err := h.db.GetHostCustomTasks(hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(tasksJSON))
}

// RunScheduledTask triggers a scheduled task immediately (manual execution).
func (h *ScheduledTaskHandler) RunScheduledTask(c *gin.Context) {
	taskID := c.Param("id")
	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}

	task, err := h.db.GetScheduledTask(taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	payload := task.Payload
	if payload == "" {
		payload = "{}"
	}
	cmd, err := h.db.CreateRemoteCommand(task.HostID, task.Module, task.Action, task.Target, payload, username, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.LinkCommandToScheduledTask(cmd.ID, task.ID); err != nil {
		log.Printf("Failed to link command %s to scheduled task %s: %v", cmd.ID, task.ID, err)
	}
	_ = h.db.UpdateScheduledTaskStatus(task.ID, "pending")

	c.JSON(http.StatusOK, gin.H{"command_id": cmd.ID, "status": "pending"})
}
