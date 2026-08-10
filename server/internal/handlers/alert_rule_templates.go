package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

func parseAlertRuleTemplateID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, apperr.Validation("Identifiant de template invalide."))
		return 0, false
	}
	return id, true
}

// ListAlertRuleTemplates returns every reusable rule template.
func (h *AlertRulesHandler) ListAlertRuleTemplates(c *gin.Context) {
	templates, err := h.svc.ListTemplates(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, templates)
}

// GetAlertRuleTemplate returns one template by id.
func (h *AlertRulesHandler) GetAlertRuleTemplate(c *gin.Context) {
	id, ok := parseAlertRuleTemplateID(c)
	if !ok {
		return
	}
	t, err := h.svc.GetTemplate(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, t)
}

// CreateAlertRuleTemplate creates a new reusable rule template (admin only —
// same posture as CreateAlertRule).
func (h *AlertRulesHandler) CreateAlertRuleTemplate(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}
	var req models.AlertRuleTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(humanizeValidationError(err)))
		return
	}
	t, err := h.svc.CreateTemplate(c.Request.Context(), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, t)
}

// UpdateAlertRuleTemplate updates an existing template. Rules already
// created from it are untouched (see models.AlertRuleTemplate's doc comment).
func (h *AlertRulesHandler) UpdateAlertRuleTemplate(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}
	id, ok := parseAlertRuleTemplateID(c)
	if !ok {
		return
	}
	var req models.AlertRuleTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(humanizeValidationError(err)))
		return
	}
	t, err := h.svc.UpdateTemplate(c.Request.Context(), id, req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, t)
}

// DeleteAlertRuleTemplate removes a template.
func (h *AlertRulesHandler) DeleteAlertRuleTemplate(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}
	id, ok := parseAlertRuleTemplateID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteTemplate(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "template deleted"})
}

// ApplyAlertRuleTemplate creates one independent alert rule per requested
// host from the template — not all-or-nothing, see
// models.ApplyAlertRuleTemplateResult.
func (h *AlertRulesHandler) ApplyAlertRuleTemplate(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}
	id, ok := parseAlertRuleTemplateID(c)
	if !ok {
		return
	}
	var req models.ApplyAlertRuleTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(humanizeValidationError(err)))
		return
	}
	result, err := h.svc.ApplyTemplate(c.Request.Context(), id, req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
