package api

import (
	"encoding/json"
	"fmt"
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

	// Cleanup any stalled commands for this host (in case agent restarted)
	if err := h.db.CleanupHostStalledCommands(hostID, 60); err != nil {
		log.Printf("Warning: failed to cleanup stalled commands for host %s: %v", hostID, err)
	}

	// Update host info from agent report (only if metrics are provided)
	if report.Metrics != nil {
		update := models.HostUpdate{
			Hostname:     stringPtrIfNotEmpty(report.Metrics.Hostname),
			OS:           stringPtrIfNotEmpty(report.Metrics.OS),
			AgentVersion: stringPtrIfNotEmpty(report.AgentVersion),
		}
		if update.Hostname != nil || update.OS != nil || update.AgentVersion != nil {
			if err := h.db.UpdateHost(hostID, &update); err != nil {
				log.Printf("Warning: failed to update host %s: %v", hostID, err)
			}
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
			if err := h.db.UpdateHost(hostID, &update); err != nil {
				log.Printf("Warning: failed to update host %s: %v", hostID, err)
			}
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

	// Store Docker networks
	if len(report.DockerNetworks) > 0 {
		dbNetworks := make([]models.DockerNetwork, 0, len(report.DockerNetworks))
		for _, n := range report.DockerNetworks {
			dbNetworks = append(dbNetworks, models.DockerNetwork{
				ID:           fmt.Sprintf("%s-%s", hostID, n.NetworkID),
				HostID:       hostID,
				NetworkID:    n.NetworkID,
				Name:         n.Name,
				Driver:       n.Driver,
				Scope:        n.Scope,
				ContainerIDs: n.ContainerIDs,
				UpdatedAt:    time.Now(),
			})
		}
		if err := h.db.UpsertDockerNetworks(hostID, dbNetworks); err != nil {
			log.Printf("Warning: failed to store docker networks for host %s: %v", hostID, err)
		}
	}

	// Store container env vars (for topology inference)
	if len(report.ContainerEnvs) > 0 {
		if err := h.db.UpsertContainerEnvs(hostID, report.ContainerEnvs); err != nil {
			log.Printf("Warning: failed to store container envs for host %s: %v", hostID, err)
		}
	}

	// Store docker-compose projects
	if len(report.ComposeProjects) > 0 {
		if err := h.db.UpsertComposeProjects(hostID, report.ComposeProjects); err != nil {
			log.Printf("Warning: failed to store compose projects for host %s: %v", hostID, err)
		}
	}

	// Store disk metrics
	if len(report.DiskMetrics) > 0 {
		for i := range report.DiskMetrics {
			report.DiskMetrics[i].HostID = hostID
			report.DiskMetrics[i].Timestamp = time.Now()
		}
		if err := h.db.InsertDiskMetrics(report.DiskMetrics); err != nil {
			log.Printf("Warning: failed to store disk metrics for host %s: %v", hostID, err)
		}
	}

	// Store disk health
	if len(report.DiskHealth) > 0 {
		for i := range report.DiskHealth {
			report.DiskHealth[i].HostID = hostID
			report.DiskHealth[i].CollectedAt = time.Now()
		}
		if err := h.db.InsertDiskHealth(report.DiskHealth); err != nil {
			log.Printf("Warning: failed to store disk health for host %s: %v", hostID, err)
		}
	}

	// Return pending commands for this host (APT + Docker)
	commands, _ := h.db.GetPendingCommands(hostID)
	if commands == nil {
		commands = []models.PendingCommand{}
	}
	dockerCmds, _ := h.db.GetPendingDockerCommands(hostID)
	if len(dockerCmds) > 0 {
		commands = append(commands, dockerCmds...)
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

	// Route docker AND systemd commands to the docker_commands table
	if result.Type == "docker" || result.Type == "systemd" {
		dockerCmd, cmdErr := h.db.GetDockerCommandByID(result.CommandID)
		if cmdErr != nil || dockerCmd.HostID != hostID {
			c.JSON(http.StatusForbidden, gin.H{"error": "command does not belong to host"})
			return
		}
		// Si c'est une commande systemd status, tente de parser le JSON pour l'affichage frontend
		if dockerCmd.Action == "systemd_status" && result.Status == "completed" && result.Output != "" {
			type systemdStatusJSON struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				LoadState   string `json:"load_state"`
				ActiveState string `json:"active_state"`
				SubState    string `json:"sub_state"`
			}
			var parsed systemdStatusJSON
			// systemctl --output=json renvoie un tableau JSON
			var arr []systemdStatusJSON
			if err := json.Unmarshal([]byte(result.Output), &arr); err == nil && len(arr) > 0 {
				parsed = arr[0]
				// On remplace l'output par un résumé lisible
				result.Output = fmt.Sprintf("%s: %s (%s/%s) - %s", parsed.Name, parsed.Description, parsed.ActiveState, parsed.SubState, parsed.LoadState)
			}
		}
		if err := h.db.UpdateDockerCommandStatus(result.CommandID, result.Status, result.Output); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update command"})
			return
		}

		if dockerCmd.AuditLogID != nil {
			details := ""
			if result.Status == "failed" {
				details = truncateOutput(result.Output, 2000)
			}
			_ = h.db.UpdateAuditLogStatus(*dockerCmd.AuditLogID, result.Status, details)
		}
		commandIDStr := strconv.FormatInt(result.CommandID, 10)
		h.streamHub.BroadcastStatus(commandIDStr, result.Status)
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}

	// APT command path
	cmd, cmdErr := h.db.GetAptCommandByID(result.CommandID)
	if cmdErr != nil || cmd.HostID != hostID {
		c.JSON(http.StatusForbidden, gin.H{"error": "command does not belong to host"})
		return
	}

	if err := h.db.UpdateCommandStatus(result.CommandID, result.Status, result.Output); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update command"})
		return
	}

	// Update related audit log status (if linked)
	if cmd != nil && cmd.AuditLogID != nil {
		details := ""
		if result.Status == "failed" {
			details = truncateOutput(result.Output, 2000)
		}
		_ = h.db.UpdateAuditLogStatus(*cmd.AuditLogID, result.Status, details)
	}

	// Broadcast status update to WebSocket clients
	commandIDStr := strconv.FormatInt(result.CommandID, 10)
	h.streamHub.BroadcastStatus(commandIDStr, result.Status)

	if result.Status == "completed" && cmd != nil {
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

	cmdID, err := strconv.ParseInt(chunk.CommandID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid command_id"})
		return
	}

	// Check ownership: try apt_commands first, then docker_commands
	aptCmd, aptErr := h.db.GetAptCommandByID(cmdID)
	if aptErr != nil {
		// Not an APT command — try docker
		dockerCmd, dockerErr := h.db.GetDockerCommandByID(cmdID)
		if dockerErr != nil || dockerCmd.HostID != hostID {
			c.JSON(http.StatusForbidden, gin.H{"error": "command does not belong to host"})
			return
		}
	} else if aptCmd.HostID != hostID {
		c.JSON(http.StatusForbidden, gin.H{"error": "command does not belong to host"})
		return
	}

	// Broadcast chunk to all connected WebSocket clients
	h.streamHub.Broadcast(chunk.CommandID, chunk.Chunk)

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func truncateOutput(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "..."
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

// GetMetricsAggregated returns metrics with intelligent aggregation based on time range
// - 0-24h: raw 5min metrics
// - 24-720h (30d): hourly aggregates
// - 720h+ (>30d): daily aggregates
func (h *AgentHandler) GetMetricsAggregated(c *gin.Context) {
	hostID := c.Param("id")
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))

	// Validate hours parameter
	if hours <= 0 {
		hours = 24
	}
	if hours > 8760 { // max 1 year
		hours = 8760
	}

	var metrics interface{}
	var err error
	var aggregationType string

	// Determine which aggregation to use based on time range
	if hours <= 24 {
		// Raw metrics (5-minute intervals)
		metrics, err = h.db.GetMetricsHistory(hostID, hours)
		aggregationType = "raw"
	} else if hours <= 720 { // 30 days
		// Hourly aggregates
		metrics, err = h.db.GetMetricsAggregatesByType(hostID, hours, "hour")
		aggregationType = "hour"
	} else {
		// Daily aggregates
		metrics, err = h.db.GetMetricsAggregatesByType(hostID, hours, "day")
		aggregationType = "day"
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch aggregated metrics"})
		return
	}
	if metrics == nil {
		metrics = []interface{}{}
	}

	c.JSON(http.StatusOK, gin.H{
		"aggregation_type": aggregationType,
		"hours":            hours,
		"metrics":          metrics,
	})
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

// LogAuditAction records an audit log entry from the agent (e.g., startup apt update)
func (h *AgentHandler) LogAuditAction(c *gin.Context) {
	hostID := c.GetString("host_id")
	if hostID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "host not identified"})
		return
	}

	var audit struct {
		Action  string `json:"action" binding:"required"`
		Status  string `json:"status" binding:"required"`
		Details string `json:"details"`
	}

	if err := c.ShouldBindJSON(&audit); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create audit log entry with "agent" as the username
	_, err := h.db.CreateAuditLog("agent", audit.Action, hostID, c.ClientIP(), audit.Details, audit.Status)
	if err != nil {
		log.Printf("Failed to log audit action: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record audit log"})
		return
	}

	// If this is an apt update action, also update the last_update timestamp
	if audit.Action == "update" && audit.Status == "completed" {
		_ = h.db.TouchAptLastAction(hostID, "update")
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Audit log recorded"})
}

func stringPtrIfNotEmpty(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}
