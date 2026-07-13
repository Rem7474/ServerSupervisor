package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

// alertRuleTagMessage returns a human-readable message for a validator tag.
func alertRuleTagMessage(field, tag string) string {
	label, ok := alertRuleFieldLabel[field]
	if !ok {
		label = field
	}
	switch tag {
	case "required":
		return fmt.Sprintf("Le champ %s est obligatoire.", label)
	case "min":
		return fmt.Sprintf("Le champ %s est trop court.", label)
	case "max":
		return fmt.Sprintf("Le champ %s est trop long.", label)
	case "email":
		return fmt.Sprintf("Le champ %s doit etre une adresse e-mail valide.", label)
	default:
		return fmt.Sprintf("Le champ %s est invalide.", label)
	}
}

// humanizeValidationError converts a go-playground/validator error into a single
// readable string. Falls back to the raw error for non-validation errors.
func humanizeValidationError(err error) string {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return err.Error()
	}
	if len(ve) == 1 {
		return alertRuleTagMessage(ve[0].Field(), ve[0].Tag())
	}
	msg := "Plusieurs champs sont invalides :"
	for _, fe := range ve {
		msg += " " + alertRuleTagMessage(fe.Field(), fe.Tag()) + ";"
	}
	return msg
}

// parseAlertRuleID parses the :id path param into the int64 rule id.
func parseAlertRuleID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, apperr.Validation("Identifiant de règle invalide."))
		return 0, false
	}
	return id, true
}

// ListAlertRules returns alert rules. Admins and users with no host_permissions
// entries see all rules; hostperm-restricted users only see rules scoped to one
// of their granted hosts (Docker/agent rules — Proxmox scopes and agent rules
// with no host_id apply cluster/fleet-wide and have no single resolvable host,
// so they stay admin-only-visible in this MVP). Rule CRUD/test/capabilities
// endpoints are unaffected — still admin-only via the router group.
func (h *AlertRulesHandler) ListAlertRules(c *gin.Context) {
	scope, err := resolveAlertHostScope(c, h.db)
	if err != nil {
		respondError(c, apperr.Failed("failed to validate host permissions"))
		return
	}
	rules, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, filterAlertRulesByScope(rules, scope))
}

// filterAlertRulesByScope keeps only the rules an alertHostScope-restricted
// caller is allowed to see.
func filterAlertRulesByScope(rules []models.AlertRule, scope alertHostScope) []models.AlertRule {
	if scope.unrestricted {
		return rules
	}
	out := make([]models.AlertRule, 0, len(rules))
	for _, r := range rules {
		if scope.allowsHost(ruleHostID(r)) {
			out = append(out, r)
		}
	}
	return out
}

// ruleHostID returns the single host a rule is scoped to, or "" when the rule
// has no single resolvable host (Proxmox scope, or an agent rule with no
// host_id — i.e. it applies fleet-wide).
func ruleHostID(r models.AlertRule) string {
	switch r.SourceType {
	case models.AlertSourceDocker:
		if r.DockerScope != nil {
			return r.DockerScope.HostID
		}
	case models.AlertSourceAgent:
		if r.HostID != nil {
			return *r.HostID
		}
	}
	return ""
}

// GetAlertRule returns a single alert rule by ID.
func (h *AlertRulesHandler) GetAlertRule(c *gin.Context) {
	id, ok := parseAlertRuleID(c)
	if !ok {
		return
	}
	rule, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, rule)
}

// CreateAlertRule creates a new alert rule.
func (h *AlertRulesHandler) CreateAlertRule(c *gin.Context) {
	var req models.AlertRuleCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(humanizeValidationError(err)))
		return
	}
	rule, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, rule)
}

// UpdateAlertRule updates an existing alert rule.
func (h *AlertRulesHandler) UpdateAlertRule(c *gin.Context) {
	id, ok := parseAlertRuleID(c)
	if !ok {
		return
	}
	var req models.AlertRuleUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(humanizeValidationError(err)))
		return
	}
	if err := h.svc.Update(c.Request.Context(), id, req); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Alert rule updated"})
}

// DeleteAlertRule deletes an alert rule.
func (h *AlertRulesHandler) DeleteAlertRule(c *gin.Context) {
	id, ok := parseAlertRuleID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Alert rule deleted"})
}
