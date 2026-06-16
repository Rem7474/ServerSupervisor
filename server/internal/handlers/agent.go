package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/events"
	"github.com/serversupervisor/server/internal/models"
	agentsvc "github.com/serversupervisor/server/internal/services/agent"
	"github.com/serversupervisor/server/internal/ws"
)

// AgentHandler is the thin HTTP layer over the agent protocol service: it extracts
// request data, binds payloads and renders responses. All ingest/result/stream/
// audit logic and the host metric reads live in internal/services/agent.
type AgentHandler struct {
	svc *agentsvc.Service
}

func NewAgentHandler(db *database.DB, cfg *config.Config, streamHub *ws.CommandStreamHub, notifPusher agentsvc.NotificationPusher, bus *events.Bus) *AgentHandler {
	return &AgentHandler{svc: agentsvc.NewService(db, cfg, streamHub, notifPusher, bus)}
}

// AddCompletionListener registers a listener notified on terminal command states.
func (h *AgentHandler) AddCompletionListener(listener agentsvc.CommandCompletionListener) {
	h.svc.AddCompletionListener(listener)
}

// ReceiveReport processes a full agent report (metrics + docker + apt).
func (h *AgentHandler) ReceiveReport(c *gin.Context) {
	hostID := c.GetString("host_id")
	if hostID == "" {
		respondError(c, apperr.Unauthorized("host not identified"))
		return
	}
	// Sanitize hostID for safe logging (prevent log forging via newlines).
	safeHostID := strings.ReplaceAll(strings.ReplaceAll(hostID, "\n", ""), "\r", "")

	const maxReportSize = 5 * 1024 * 1024 // 5 MB — prevent oversized payloads from a rogue agent
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxReportSize)

	var report models.AgentReport
	if err := c.ShouldBindJSON(&report); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}

	res, err := h.svc.ReceiveReport(c.Request.Context(), hostID, safeHostID, &report)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       "ok",
		"commands":     res.Commands,
		"skip_metrics": res.SkipMetrics,
	})
}

// ReportCommandResult receives command execution results from agents.
func (h *AgentHandler) ReportCommandResult(c *gin.Context) {
	hostID := c.GetString("host_id")
	if hostID == "" {
		respondError(c, apperr.Unauthorized("host not identified"))
		return
	}

	var result models.CommandResult
	if err := c.ShouldBindJSON(&result); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}

	if err := h.svc.ReportCommandResult(c.Request.Context(), hostID, result); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// StreamCommandOutput receives streaming output chunks from agents.
func (h *AgentHandler) StreamCommandOutput(c *gin.Context) {
	hostID := c.GetString("host_id")
	if hostID == "" {
		respondError(c, apperr.Unauthorized("host not identified"))
		return
	}

	var chunk struct {
		CommandID string `json:"command_id" binding:"required"`
		Chunk     string `json:"chunk" binding:"required"`
	}
	if err := c.ShouldBindJSON(&chunk); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}

	if err := h.svc.StreamCommandOutput(c.Request.Context(), hostID, chunk.CommandID, chunk.Chunk); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetHostCommandHistory returns all recent commands for a host across all modules.
func (h *AgentHandler) GetHostCommandHistory(c *gin.Context) {
	hostID := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	cmds, err := h.svc.HostCommandHistory(c.Request.Context(), hostID, limit)
	if err != nil {
		respondError(c, apperr.Failed("failed to fetch command history"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"commands": cmds})
}

// GetMetricsHistory returns historical metrics for charts.
func (h *AgentHandler) GetMetricsHistory(c *gin.Context) {
	hostID := c.Param("id")
	hours := clampHours(c.DefaultQuery("hours", "24"))

	metrics, err := h.svc.MetricsHistory(c.Request.Context(), hostID, hours)
	if err != nil {
		respondError(c, apperr.Failed("failed to fetch metrics"))
		return
	}
	c.JSON(http.StatusOK, metrics)
}

// GetMetricsAggregated returns metrics with intelligent aggregation based on time
// range: 0-24h raw 5min, 24-720h hourly, 720h+ daily.
func (h *AgentHandler) GetMetricsAggregated(c *gin.Context) {
	hostID := c.Param("id")
	hours := clampHours(c.DefaultQuery("hours", "24"))

	metrics, aggregationType, err := h.svc.MetricsAggregated(c.Request.Context(), hostID, hours)
	if err != nil {
		respondError(c, apperr.Failed("failed to fetch aggregated metrics"))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"aggregation_type": aggregationType,
		"hours":            hours,
		"metrics":          metrics,
	})
}

// GetMetricsSummary returns the global metrics summary for dashboard charts.
func (h *AgentHandler) GetMetricsSummary(c *gin.Context) {
	hours := clampHours(c.DefaultQuery("hours", "24"))
	bucketMinutes, _ := strconv.Atoi(c.DefaultQuery("bucket_minutes", "5"))
	if bucketMinutes <= 0 {
		bucketMinutes = 5
	}

	summary, err := h.svc.MetricsSummary(c.Request.Context(), hours, bucketMinutes)
	if err != nil {
		respondError(c, apperr.Failed("failed to fetch metrics summary"))
		return
	}
	c.JSON(http.StatusOK, summary)
}

// LogAuditAction records an audit log entry from the agent (e.g. startup apt
// update). When "module" is provided, a completed remote_command is also created
// so the action appears in the unified commands history.
func (h *AgentHandler) LogAuditAction(c *gin.Context) {
	hostID := c.GetString("host_id")
	if hostID == "" {
		respondError(c, apperr.Unauthorized("host not identified"))
		return
	}

	var audit struct {
		Module  string `json:"module"` // optional — e.g. "apt"; creates a remote_command when set
		Action  string `json:"action" binding:"required"`
		Status  string `json:"status" binding:"required"`
		Details string `json:"details"`
	}
	if err := c.ShouldBindJSON(&audit); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}

	if err := h.svc.LogAuditAction(c.Request.Context(), hostID, audit.Module, audit.Action, audit.Status, audit.Details, c.ClientIP()); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Audit log recorded"})
}

// clampHours parses an hours query value and clamps it to [1, 8760] (max 1 year).
func clampHours(raw string) int {
	hours, _ := strconv.Atoi(raw)
	if hours <= 0 {
		hours = 24
	}
	if hours > 8760 {
		hours = 8760
	}
	return hours
}
