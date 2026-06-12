package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
	settingssvc "github.com/serversupervisor/server/internal/services/settings"
)

// SettingsHandler translates HTTP to the settings service. Admin authz stays
// here; the snapshot/persist/diagnostics/cleanup logic lives in
// internal/services/settings.
type SettingsHandler struct {
	svc *settingssvc.Service
}

func NewSettingsHandler(svc *settingssvc.Service) *SettingsHandler {
	return &SettingsHandler{svc: svc}
}

// GetSettings returns system configuration and database status.
func (h *SettingsHandler) GetSettings(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.Snapshot(c.Request.Context()))
}

// UpdateSettings persists configuration changes and applies them in memory.
func (h *SettingsHandler) UpdateSettings(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}
	var req models.SettingsUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	h.svc.Update(c.Request.Context(), req, c.GetString("username"), c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Paramètres mis à jour"})
}

// TestSmtp tests SMTP connectivity with full TLS/auth/envelope validation.
func (h *SettingsHandler) TestSmtp(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}
	message, err := h.svc.TestSMTP(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": message})
}

// TestNtfy sends a test notification to ntfy.sh.
func (h *SettingsHandler) TestNtfy(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}
	message, err := h.svc.TestNtfy(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": message})
}

// CleanupMetrics triggers manual cleanup of old metrics.
func (h *SettingsHandler) CleanupMetrics(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}
	user := c.GetString("username")
	slog.InfoContext(c.Request.Context(), "manual metrics cleanup triggered", slog.String("user", user))
	deletedDigests, message, err := h.svc.CleanupMetrics(c.Request.Context(), user, c.ClientIP())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": message, "deleted_digests": deletedDigests})
}

// CleanupAuditLogs triggers manual cleanup of old audit logs.
func (h *SettingsHandler) CleanupAuditLogs(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}
	user := c.GetString("username")
	slog.InfoContext(c.Request.Context(), "manual audit logs cleanup triggered", slog.String("user", user))
	deleted, message, err := h.svc.CleanupAuditLogs(c.Request.Context(), user, c.ClientIP())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": message, "deleted": deleted})
}
