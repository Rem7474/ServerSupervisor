package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/alerts"
	"github.com/serversupervisor/server/internal/models"
)

// TestAlertRule evaluates a rule against current metrics without saving it.
func (h *AlertRulesHandler) TestAlertRule(c *gin.Context) {
	var req struct {
		SourceType         models.AlertSourceType     `json:"source_type"`
		HostID             *string                    `json:"host_id"`
		ProxmoxScope       *models.ProxmoxMetricScope `json:"proxmox_scope"`
		DockerScope        *models.DockerMetricScope  `json:"docker_scope"`
		Metric             string                     `json:"metric" binding:"required"`
		Operator           string                     `json:"operator" binding:"required"`
		ThresholdWarn      float64                    `json:"threshold_warn" binding:"required"`
		ThresholdCrit      float64                    `json:"threshold_crit" binding:"required"`
		ThresholdClearWarn *float64                   `json:"threshold_clear_warn"`
		ThresholdClearCrit *float64                   `json:"threshold_clear_crit"`
		Duration           int                        `json:"duration"`
		Actions            models.AlertActions        `json:"actions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": humanizeValidationError(err)})
		return
	}
	if err := validateAlertRuleMetricOperator(req.Metric, req.Operator); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.SourceType == "" {
		req.SourceType = models.InferAlertSourceType(req.Metric)
	}

	if err := validateAlertActions(&req.Actions); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule := models.AlertRule{
		SourceType:         req.SourceType,
		HostID:             req.HostID,
		ProxmoxScope:       req.ProxmoxScope,
		DockerScope:        req.DockerScope,
		Metric:             req.Metric,
		Operator:           req.Operator,
		ThresholdWarn:      &req.ThresholdWarn,
		ThresholdCrit:      &req.ThresholdCrit,
		ThresholdClearWarn: req.ThresholdClearWarn,
		ThresholdClearCrit: req.ThresholdClearCrit,
		DurationSeconds:    req.Duration,
		Actions:            req.Actions,
		Enabled:            true,
	}
	// Test endpoint supports agent-wide preview when host_id is omitted.
	validationRule := rule
	if validationRule.SourceType == models.AlertSourceAgent && validationRule.HostID == nil {
		placeholderHostID := "__test_all_hosts__"
		validationRule.HostID = &placeholderHostID
	}

	if err := validationRule.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if rule.SourceType == models.AlertSourceProxmox {
		if err := validateProxmoxScopeExists(c.Request.Context(), h.db, rule.ProxmoxScope); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	if rule.SourceType == models.AlertSourceDocker {
		if err := validateDockerScopeExists(c.Request.Context(), h.db, rule.DockerScope); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	type TestResult struct {
		HostID       string  `json:"host_id"`
		HostName     string  `json:"host_name"`
		CurrentValue float64 `json:"current_value"`
		WouldFire    bool    `json:"would_fire"`
		HasData      bool    `json:"has_data"`
	}

	var results []TestResult
	anyFires := false

	ruleNoStaleness := rule
	if rule.Metric != "proxmox_auth_failures_recent" {
		ruleNoStaleness.DurationSeconds = 0
	}

	if rule.SourceType == models.AlertSourceProxmox {
		targetID, targetLabel := h.proxmoxScopeTestTarget(c.Request.Context(), rule.ProxmoxScope)
		target := models.Host{ID: targetID, Name: targetLabel, Status: "online", LastSeen: time.Now()}
		value, ok := alerts.GetMetricValue(c.Request.Context(), h.db, target, ruleNoStaleness)
		_, freshOk := alerts.GetMetricValue(c.Request.Context(), h.db, target, rule)
		wouldFire := ok && freshOk && alerts.MatchRule(rule, target, value)
		if wouldFire {
			anyFires = true
		}
		results = append(results, TestResult{
			HostID:       target.ID,
			HostName:     target.Name,
			CurrentValue: value,
			WouldFire:    wouldFire,
			HasData:      ok,
		})
	} else if rule.SourceType == models.AlertSourceDocker {
		targets := alerts.BuildDockerTestTargets(c.Request.Context(), h.db, rule)
		for _, target := range targets {
			value, ok := alerts.GetMetricValue(c.Request.Context(), h.db, target, ruleNoStaleness)
			_, freshOk := alerts.GetMetricValue(c.Request.Context(), h.db, target, rule)
			wouldFire := ok && freshOk && alerts.MatchRule(rule, target, value)
			if wouldFire {
				anyFires = true
			}
			results = append(results, TestResult{
				HostID:       target.ID,
				HostName:     target.Name,
				CurrentValue: value,
				WouldFire:    wouldFire,
				HasData:      ok,
			})
		}
	} else {
		hosts, err := h.db.GetAllHosts(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch hosts"})
			return
		}

		for _, host := range hosts {
			if rule.HostID != nil && *rule.HostID != host.ID {
				continue
			}
			value, ok := alerts.GetMetricValue(c.Request.Context(), h.db, host, ruleNoStaleness)
			_, freshOk := alerts.GetMetricValue(c.Request.Context(), h.db, host, rule)
			wouldFire := ok && freshOk && alerts.MatchRule(rule, host, value)
			if wouldFire {
				anyFires = true
			}
			results = append(results, TestResult{
				HostID:       host.ID,
				HostName:     host.Name,
				CurrentValue: value,
				WouldFire:    wouldFire,
				HasData:      ok,
			})
		}
	}

	if results == nil {
		results = []TestResult{}
	}
	c.JSON(http.StatusOK, gin.H{
		"any_fires":    anyFires,
		"evaluated_at": time.Now(),
		"results":      results,
	})
}

// TestAlertRuleLogs returns the log lines used to evaluate proxmox_auth_failures_recent.
func (h *AlertRulesHandler) TestAlertRuleLogs(c *gin.Context) {
	var req struct {
		SourceType         models.AlertSourceType     `json:"source_type"`
		HostID             *string                    `json:"host_id"`
		ProxmoxScope       *models.ProxmoxMetricScope `json:"proxmox_scope"`
		Metric             string                     `json:"metric" binding:"required"`
		Operator           string                     `json:"operator" binding:"required"`
		ThresholdWarn      float64                    `json:"threshold_warn" binding:"required"`
		ThresholdCrit      float64                    `json:"threshold_crit" binding:"required"`
		ThresholdClearWarn *float64                   `json:"threshold_clear_warn"`
		ThresholdClearCrit *float64                   `json:"threshold_clear_crit"`
		Duration           int                        `json:"duration"`
		Actions            models.AlertActions        `json:"actions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": humanizeValidationError(err)})
		return
	}
	if req.SourceType == "" {
		req.SourceType = models.InferAlertSourceType(req.Metric)
	}
	if req.Metric != "proxmox_auth_failures_recent" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Metrique non supportee pour les logs."})
		return
	}

	rule := models.AlertRule{
		SourceType:         req.SourceType,
		HostID:             req.HostID,
		ProxmoxScope:       req.ProxmoxScope,
		Metric:             req.Metric,
		Operator:           req.Operator,
		ThresholdWarn:      &req.ThresholdWarn,
		ThresholdCrit:      &req.ThresholdCrit,
		ThresholdClearWarn: req.ThresholdClearWarn,
		ThresholdClearCrit: req.ThresholdClearCrit,
		DurationSeconds:    req.Duration,
		Actions:            req.Actions,
		Enabled:            true,
	}

	if rule.SourceType != models.AlertSourceProxmox {
		c.JSON(http.StatusBadRequest, gin.H{"error": "La metrique requiert une source Proxmox."})
		return
	}
	if err := validateProxmoxScopeExists(c.Request.Context(), h.db, rule.ProxmoxScope); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	lines, since := alerts.FetchProxmoxAuthFailureLogs(c.Request.Context(), h.db, rule)
	content := strings.Join(lines, "\n")
	filename := fmt.Sprintf("proxmox-auth-failures-%s.log", time.Now().Format("20060102-150405"))

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("X-Log-Since", since.UTC().Format(time.RFC3339))
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(content))
}
