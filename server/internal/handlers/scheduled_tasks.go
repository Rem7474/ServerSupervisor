package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	sttask "github.com/serversupervisor/server/internal/services/scheduledtask"
)

// ScheduledTaskHandler translates HTTP to the scheduled-task service. It keeps a
// db reference solely for requireHostAccess in RunScheduledTask (HTTP-level host
// authorization, which must write the 403 to the gin context); all task logic
// lives in internal/services/scheduledtask.
type ScheduledTaskHandler struct {
	svc *sttask.Service
	db  *database.DB
}

func NewScheduledTaskHandler(svc *sttask.Service, db *database.DB) *ScheduledTaskHandler {
	return &ScheduledTaskHandler{svc: svc, db: db}
}

// ListAllScheduledTasks returns all scheduled tasks across all hosts (global view).
func (h *ScheduledTaskHandler) ListAllScheduledTasks(c *gin.Context) {
	tasks, err := h.svc.ListAll(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, tasks)
}

// ListScheduledTasks returns all scheduled tasks for a host.
func (h *ScheduledTaskHandler) ListScheduledTasks(c *gin.Context) {
	tasks, err := h.svc.ListForHost(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, tasks)
}

// CreateScheduledTask creates a new scheduled task for a host.
func (h *ScheduledTaskHandler) CreateScheduledTask(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}
	var req models.ScheduledTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	created, err := h.svc.Create(c.Request.Context(), c.Param("id"), username, req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, created)
}

// UpdateScheduledTask modifies an existing scheduled task.
func (h *ScheduledTaskHandler) UpdateScheduledTask(c *gin.Context) {
	var req models.ScheduledTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	updated, err := h.svc.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

// DeleteScheduledTask removes a scheduled task.
func (h *ScheduledTaskHandler) DeleteScheduledTask(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// GetCustomTasks returns the list of available custom tasks for a host,
// as reported by the agent from its local tasks.yaml.
func (h *ScheduledTaskHandler) GetCustomTasks(c *gin.Context) {
	tasksJSON, err := h.svc.CustomTasks(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(tasksJSON))
}

// GetTasksConfigYAML returns the raw tasks.yaml content cached from the agent's last report.
func (h *ScheduledTaskHandler) GetTasksConfigYAML(c *gin.Context) {
	yaml, err := h.svc.TasksYAML(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"yaml": yaml})
}

// GetScheduledTaskExecutions returns the last N executions (remote_commands) for a task.
func (h *ScheduledTaskHandler) GetScheduledTaskExecutions(c *gin.Context) {
	limit := 20
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	cmds, err := h.svc.Executions(c.Request.Context(), c.Param("id"), limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, cmds)
}

// RunScheduledTask triggers a scheduled task immediately (manual execution).
func (h *ScheduledTaskHandler) RunScheduledTask(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}
	task, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	if !requireHostAccess(c, h.db, task.HostID, "operator") {
		return
	}
	commandID, err := h.svc.Run(c.Request.Context(), *task, username)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"command_id": commandID, "status": "pending"})
}
