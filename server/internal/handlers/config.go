package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
	configsvc "github.com/serversupervisor/server/internal/services/config"
)

// ConfigHandler exposes endpoints for managing system & Docker configuration.
type ConfigHandler struct {
	svc *configsvc.Service
}

func NewConfigHandler(svc *configsvc.Service) *ConfigHandler {
	return &ConfigHandler{svc: svc}
}

// GetConfig returns the full list of system parameters with effective values and origins.
func (h *ConfigHandler) GetConfig(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}

	reveal := c.Query("reveal") == "true"
	summary, err := h.svc.GetEffectiveConfig(c.Request.Context(), reveal)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, summary)
}

type updateConfigKeyRequest struct {
	Value string `json:"value"`
}

// UpdateConfigKey updates a single configuration parameter by key.
func (h *ConfigHandler) UpdateConfigKey(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}

	key := c.Param("key")
	var req updateConfigKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}

	entry, warning, err := h.svc.UpdateParam(c.Request.Context(), key, req.Value, c.GetString("username"), c.ClientIP())
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"entry":   entry,
		"warning": warning,
		"message": "Paramètre mis à jour",
	})
}

// ResetConfigKey removes any UI override for the parameter, returning to ENV or default.
func (h *ConfigHandler) ResetConfigKey(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}

	key := c.Param("key")
	entry, warning, err := h.svc.ResetParam(c.Request.Context(), key, c.GetString("username"), c.ClientIP())
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"entry":   entry,
		"warning": warning,
		"message": "Paramètre réinitialisé",
	})
}

type updateBulkRequest struct {
	Settings map[string]string `json:"settings"`
}

// UpdateConfigBulk updates multiple parameters simultaneously.
func (h *ConfigHandler) UpdateConfigBulk(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}

	var req updateBulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}

	summary, warnings, err := h.svc.UpdateBulk(c.Request.Context(), req.Settings, c.GetString("username"), c.ClientIP())
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"summary":  summary,
		"warnings": warnings,
		"message":  "Configuration mise à jour",
	})
}
