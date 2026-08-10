package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	auditsvc "github.com/serversupervisor/server/internal/services/audit"
)

// AuditHandler translates HTTP to the audit service. Role authorization, query
// parsing / limit clamping and response envelopes stay here (HTTP concerns); the
// read orchestration lives in internal/services/audit.
type AuditHandler struct {
	svc *auditsvc.Service
}

func NewAuditHandler(svc *auditsvc.Service) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// clampQueryInt reads a positive query int, applying a default and a max cap.
func clampQueryInt(c *gin.Context, key string, def, max int) int {
	v := def
	if raw := c.Query(key); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= max {
			v = parsed
		}
	}
	return v
}

// parseAuditLogFilter reads the optional category/from/to query params shared
// by GetAuditLogs and ExportAuditLogs. from/to accept RFC3339 or a bare
// YYYY-MM-DD date (the latter treated as that day's start, respectively end
// of day, in UTC — a plain HTML date input sends this shape).
func parseAuditLogFilter(c *gin.Context) database.AuditLogFilter {
	f := database.AuditLogFilter{Category: c.Query("category")}
	if raw := c.Query("from"); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			f.From = &t
		} else if t, err := time.Parse("2006-01-02", raw); err == nil {
			f.From = &t
		}
	}
	if raw := c.Query("to"); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			f.To = &t
		} else if t, err := time.Parse("2006-01-02", raw); err == nil {
			endOfDay := t.Add(24*time.Hour - time.Nanosecond)
			f.To = &endOfDay
		}
	}
	return f
}

// GetAuditLogs returns a filtered, paginated page of audit logs (admin only).
func (h *AuditHandler) GetAuditLogs(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}
	page := clampQueryInt(c, "page", 1, 1<<31-1)
	limit := clampQueryInt(c, "limit", 50, 100)
	logs, err := h.svc.Logs(c.Request.Context(), limit, (page-1)*limit, parseAuditLogFilter(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs, "page": page, "limit": limit})
}

// ExportAuditLogs streams the same filtered set as GetAuditLogs (unpaginated,
// capped at auditsvc's maxExportRows) as a CSV download.
func (h *AuditHandler) ExportAuditLogs(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}
	logs, err := h.svc.Export(c.Request.Context(), parseAuditLogFilter(c))
	if err != nil {
		respondError(c, err)
		return
	}

	filename := fmt.Sprintf("audit-logs-%s.csv", time.Now().Format("20060102-150405"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Header("Content-Type", "text/csv; charset=utf-8")

	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"id", "created_at", "username", "category", "action", "host_name", "ip_address", "status", "details"})
	for _, l := range logs {
		_ = w.Write([]string{
			strconv.FormatInt(l.ID, 10),
			l.CreatedAt.UTC().Format(time.RFC3339),
			l.Username,
			l.Category,
			l.Action,
			l.HostName,
			l.IPAddress,
			l.Status,
			l.Details,
		})
	}
	w.Flush()
}

// GetAuditLogsByHost returns audit logs for a specific host.
func (h *AuditHandler) GetAuditLogsByHost(c *gin.Context) {
	hostID := c.Param("host_id")
	if hostID == "" {
		respondError(c, apperr.Validation("host_id required"))
		return
	}
	logs, err := h.svc.LogsByHost(c.Request.Context(), hostID, clampQueryInt(c, "limit", 100, 500))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, logs)
}

// GetMyAuditLogs returns the current user's own audit logs.
func (h *AuditHandler) GetMyAuditLogs(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		respondError(c, apperr.Unauthorized("unauthorized"))
		return
	}
	logs, err := h.svc.LogsByUser(c.Request.Context(), username, clampQueryInt(c, "limit", 10, 100))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": username, "logs": logs})
}

// GetCommandsHistory returns paginated remote commands for all hosts (admin and operator).
func (h *AuditHandler) GetCommandsHistory(c *gin.Context) {
	if role := c.GetString("role"); role != models.RoleAdmin && role != models.RoleOperator {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}
	page := clampQueryInt(c, "page", 1, 1<<31-1)
	limit := 50
	f := database.CommandFilter{
		Search: c.Query("search"),
		Module: c.Query("module"),
		Status: c.Query("status"),
	}
	cmds, total, err := h.svc.Commands(c.Request.Context(), limit, (page-1)*limit, f)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"commands": cmds, "total": total, "page": page, "limit": limit})
}

// GetCommandByID returns the status and output of a single remote command by UUID.
// Requires admin or operator role to prevent cross-host information disclosure.
func (h *AuditHandler) GetCommandByID(c *gin.Context) {
	if role := c.GetString("role"); role != models.RoleAdmin && role != models.RoleOperator {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}
	id := c.Param("id")
	if id == "" {
		respondError(c, apperr.Validation("id required"))
		return
	}
	cmd, err := h.svc.Command(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, cmd)
}

// CancelCommand cancels a pending or running command by UUID.
func (h *AuditHandler) CancelCommand(c *gin.Context) {
	if role := c.GetString("role"); role != models.RoleAdmin && role != models.RoleOperator {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}
	id := c.Param("id")
	if id == "" {
		respondError(c, apperr.Validation("id required"))
		return
	}
	if err := h.svc.Cancel(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

// GetHostTimeline returns a merged chronological activity feed for a host.
func (h *AuditHandler) GetHostTimeline(c *gin.Context) {
	hostID := c.Param("id")
	if hostID == "" {
		respondError(c, apperr.Validation("id required"))
		return
	}
	limit := clampQueryInt(c, "limit", 50, 200)
	events, err := h.svc.HostTimeline(c.Request.Context(), hostID, limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": events})
}

// GetAuditLogsByUser returns audit logs for a specific user (admin only).
func (h *AuditHandler) GetAuditLogsByUser(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}
	username := c.Param("username")
	if username == "" {
		respondError(c, apperr.Validation("username required"))
		return
	}
	logs, err := h.svc.LogsByUser(c.Request.Context(), username, clampQueryInt(c, "limit", 100, 500))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": username, "logs": logs})
}
