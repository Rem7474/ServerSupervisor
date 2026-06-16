package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	alertrulesvc "github.com/serversupervisor/server/internal/services/alertrule"
)

// TestAlertRule evaluates a rule against current metrics without saving it.
func (h *AlertRulesHandler) TestAlertRule(c *gin.Context) {
	var in alertrulesvc.TestRunInput
	if err := c.ShouldBindJSON(&in); err != nil {
		respondError(c, apperr.Validation(humanizeValidationError(err)))
		return
	}

	results, anyFires, err := h.svc.TestRun(c.Request.Context(), in)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"any_fires":    anyFires,
		"evaluated_at": time.Now(),
		"results":      results,
	})
}

// TestAlertRuleLogs returns the log lines used to evaluate proxmox_auth_failures_recent.
func (h *AlertRulesHandler) TestAlertRuleLogs(c *gin.Context) {
	var in alertrulesvc.TestRunInput
	if err := c.ShouldBindJSON(&in); err != nil {
		respondError(c, apperr.Validation(humanizeValidationError(err)))
		return
	}

	lines, since, err := h.svc.TestRunLogs(c.Request.Context(), in)
	if err != nil {
		respondError(c, err)
		return
	}

	content := strings.Join(lines, "\n")
	filename := fmt.Sprintf("proxmox-auth-failures-%s.log", time.Now().Format("20060102-150405"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("X-Log-Since", since.UTC().Format(time.RFC3339))
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(content))
}
