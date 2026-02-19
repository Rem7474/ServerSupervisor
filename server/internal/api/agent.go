package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type AgentHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewAgentHandler(db *database.DB, cfg *config.Config) *AgentHandler {
	return &AgentHandler{db: db, cfg: cfg}
}

// ReceiveReport processes a full agent report (metrics + docker + apt)
func (h *AgentHandler) ReceiveReport(c *gin.Context) {
	hostID := c.GetString("host_id")
	if hostID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "host not identified"})
		return
	}

	var report models.AgentReport
	if err := c.ShouldBindJSON(&report); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update host status
	h.db.UpdateHostStatus(hostID, "online")

	// Update host info from agent report
	update := models.HostUpdate{
		Hostname: stringPtrIfNotEmpty(report.Metrics.Hostname),
		OS:       stringPtrIfNotEmpty(report.Metrics.OS),
	}
	if update.Hostname != nil || update.OS != nil {
		_ = h.db.UpdateHost(hostID, &update)
	}

	// Store metrics
	report.Metrics.HostID = hostID
	report.Metrics.Timestamp = time.Now()
	if _, err := h.db.InsertMetrics(&report.Metrics); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store metrics"})
		return
	}

	// Store docker containers
	if len(report.Docker.Containers) > 0 {
		for i := range report.Docker.Containers {
			report.Docker.Containers[i].HostID = hostID
		}
		h.db.UpsertDockerContainers(hostID, report.Docker.Containers)
	}

	// Store apt status
	report.AptStatus.HostID = hostID
	h.db.UpsertAptStatus(&report.AptStatus)

	// Return pending commands for this host
	commands, _ := h.db.GetPendingCommands(hostID)
	if commands == nil {
		commands = []models.PendingCommand{}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"commands": commands,
	})
}

// ReportCommandResult receives command execution results from agents
func (h *AgentHandler) ReportCommandResult(c *gin.Context) {
	hostID := c.GetString("host_id")
	if hostID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "host not identified"})
		return
	}

	var result models.CommandResult
	if err := c.ShouldBindJSON(&result); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.UpdateCommandStatus(result.CommandID, result.Status, result.Output); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update command"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetMetricsHistory returns historical metrics for charts
func (h *AgentHandler) GetMetricsHistory(c *gin.Context) {
	hostID := c.Param("id")
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	if hours > 168 { // max 7 days
		hours = 168
	}

	metrics, err := h.db.GetMetricsHistory(hostID, hours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch metrics"})
		return
	}
	if metrics == nil {
		metrics = []models.SystemMetrics{}
	}
	c.JSON(http.StatusOK, metrics)
}

func stringPtrIfNotEmpty(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}
