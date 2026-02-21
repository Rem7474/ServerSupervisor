package api

import (
	"log"
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
	db        *database.DB
	cfg       *config.Config
	streamHub *AptStreamHub
}

func NewAgentHandler(db *database.DB, cfg *config.Config, streamHub *AptStreamHub) *AgentHandler {
	return &AgentHandler{
		db:        db,
		cfg:       cfg,
		streamHub: streamHub,
	}
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

	// Update host info from agent report (only if metrics are provided)
	if report.Metrics != nil {
		update := models.HostUpdate{
			Hostname:     stringPtrIfNotEmpty(report.Metrics.Hostname),
			OS:           stringPtrIfNotEmpty(report.Metrics.OS),
			AgentVersion: stringPtrIfNotEmpty(report.AgentVersion),
		}
		if update.Hostname != nil || update.OS != nil || update.AgentVersion != nil {
			_ = h.db.UpdateHost(hostID, &update)
		}

		// Store metrics
		report.Metrics.HostID = hostID
		report.Metrics.Timestamp = time.Now()
		if _, err := h.db.InsertMetrics(report.Metrics); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store metrics"})
			return
		}
	} else {
		// If no metrics, still update agent version
		if report.AgentVersion != "" {
			update := models.HostUpdate{
				AgentVersion: stringPtrIfNotEmpty(report.AgentVersion),
			}
			_ = h.db.UpdateHost(hostID, &update)
		}
	}

	// Store docker containers
	if report.Docker != nil && len(report.Docker.Containers) > 0 {
		for i := range report.Docker.Containers {
			report.Docker.Containers[i].HostID = hostID
		}
		if err := h.db.UpsertDockerContainers(hostID, report.Docker.Containers); err != nil {
			// Log error but don't fail the entire request
			c.Header("X-Docker-Error", err.Error())
		}
	}

	// Store apt status
	if report.AptStatus != nil {
		report.AptStatus.HostID = hostID
		if err := h.db.UpsertAptStatus(report.AptStatus); err != nil {
			// Log error but don't fail the entire request
			c.Header("X-APT-Error", err.Error())
		}
	}

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

	// Broadcast status update to WebSocket clients
	commandIDStr := strconv.FormatInt(result.CommandID, 10)
	h.streamHub.BroadcastStatus(commandIDStr, result.Status)

	if result.Status == "completed" {
		cmd, err := h.db.GetAptCommandByID(result.CommandID)
		if err == nil {
			_ = h.db.TouchAptLastAction(cmd.HostID, cmd.Command)

			// Update full APT status if provided with command result
			if result.AptStatus != nil {
				result.AptStatus.HostID = cmd.HostID
				err := h.db.UpsertAptStatus(result.AptStatus)
				if err != nil {
					log.Printf("Failed to update APT status: %v", err)
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// StreamCommandOutput receives streaming output chunks from agents
func (h *AgentHandler) StreamCommandOutput(c *gin.Context) {
	hostID := c.GetString("host_id")
	if hostID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "host not identified"})
		return
	}

	var chunk struct {
		CommandID string `json:"command_id" binding:"required"`
		Chunk     string `json:"chunk" binding:"required"`
	}
	if err := c.ShouldBindJSON(&chunk); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Broadcast chunk to all connected WebSocket clients
	h.streamHub.Broadcast(chunk.CommandID, chunk.Chunk)

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetMetricsHistory returns historical metrics for charts
func (h *AgentHandler) GetMetricsHistory(c *gin.Context) {
	hostID := c.Param("id")
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))

	// Validate hours parameter
	if hours <= 0 {
		hours = 24
	}
	if hours > 8760 { // max 1 year
		hours = 8760
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

// GetMetricsSummary returns global metrics summary for dashboard charts
func (h *AgentHandler) GetMetricsSummary(c *gin.Context) {
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	bucketMinutes, _ := strconv.Atoi(c.DefaultQuery("bucket_minutes", "5"))

	if hours <= 0 {
		hours = 24
	}
	if hours > 8760 {
		hours = 8760
	}
	if bucketMinutes <= 0 {
		bucketMinutes = 5
	}

	summary, err := h.db.GetMetricsSummary(hours, bucketMinutes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch metrics summary"})
		return
	}
	if summary == nil {
		summary = []models.SystemMetricsSummary{}
	}
	c.JSON(http.StatusOK, summary)
}

func stringPtrIfNotEmpty(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}
