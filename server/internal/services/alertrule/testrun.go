package alertrule

import (
	"context"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

// EngineFuncs are the alert-engine entry points the preview ("test run") needs.
// They are injected as funcs so the service stays free of the alerts/database
// imports (each closure binds the concrete *database.DB at wiring time).
type EngineFuncs struct {
	MetricValue        func(ctx context.Context, host models.Host, rule models.AlertRule) (float64, bool)
	MatchRule          func(rule models.AlertRule, host models.Host, value float64) bool
	BuildDockerTargets func(ctx context.Context, rule models.AlertRule) []models.Host
	FetchProxmoxLogs   func(ctx context.Context, rule models.AlertRule) ([]string, time.Time)
}

// TestRunInput is the payload for the preview endpoints (also reused for the
// proxmox auth-failure log export, which ignores the docker-only fields).
type TestRunInput struct {
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

// TestRunResult is a single host/target preview outcome.
type TestRunResult struct {
	HostID       string  `json:"host_id"`
	HostName     string  `json:"host_name"`
	CurrentValue float64 `json:"current_value"`
	WouldFire    bool    `json:"would_fire"`
	HasData      bool    `json:"has_data"`
}

func (in TestRunInput) toRule() models.AlertRule {
	return models.AlertRule{
		SourceType:         in.SourceType,
		HostID:             in.HostID,
		ProxmoxScope:       in.ProxmoxScope,
		DockerScope:        in.DockerScope,
		Metric:             in.Metric,
		Operator:           in.Operator,
		ThresholdWarn:      &in.ThresholdWarn,
		ThresholdCrit:      &in.ThresholdCrit,
		ThresholdClearWarn: in.ThresholdClearWarn,
		ThresholdClearCrit: in.ThresholdClearCrit,
		DurationSeconds:    in.Duration,
		Actions:            in.Actions,
		Enabled:            true,
	}
}

// TestRun evaluates a rule against current metrics without saving it, returning
// per-host/target outcomes and whether any would fire.
func (s *Service) TestRun(ctx context.Context, in TestRunInput) ([]TestRunResult, bool, error) {
	if err := s.ValidateMetricOperator(in.Metric, in.Operator); err != nil {
		return nil, false, err
	}
	if in.SourceType == "" {
		in.SourceType = models.InferAlertSourceType(in.Metric)
	}
	if err := s.ValidateActions(&in.Actions); err != nil {
		return nil, false, err
	}

	rule := in.toRule()

	// Test endpoint supports agent-wide preview when host_id is omitted.
	validationRule := rule
	if validationRule.SourceType == models.AlertSourceAgent && validationRule.HostID == nil {
		placeholderHostID := "__test_all_hosts__"
		validationRule.HostID = &placeholderHostID
	}
	if err := validationRule.Validate(); err != nil {
		return nil, false, apperr.Validation(err.Error())
	}

	switch rule.SourceType {
	case models.AlertSourceProxmox:
		if err := s.ValidateProxmoxScope(ctx, rule.ProxmoxScope); err != nil {
			return nil, false, err
		}
	case models.AlertSourceDocker:
		if err := s.ValidateDockerScope(ctx, rule.DockerScope); err != nil {
			return nil, false, err
		}
	}

	// Staleness only applies to the auth-failures metric; everything else is
	// evaluated against the latest value regardless of duration.
	ruleNoStaleness := rule
	if rule.Metric != "proxmox_auth_failures_recent" {
		ruleNoStaleness.DurationSeconds = 0
	}

	var results []TestRunResult
	anyFires := false
	eval := func(target models.Host) {
		value, ok := s.engine.MetricValue(ctx, target, ruleNoStaleness)
		_, freshOk := s.engine.MetricValue(ctx, target, rule)
		wouldFire := ok && freshOk && s.engine.MatchRule(rule, target, value)
		if wouldFire {
			anyFires = true
		}
		results = append(results, TestRunResult{
			HostID:       target.ID,
			HostName:     target.Name,
			CurrentValue: value,
			WouldFire:    wouldFire,
			HasData:      ok,
		})
	}

	switch rule.SourceType {
	case models.AlertSourceProxmox:
		targetID, targetLabel := s.ProxmoxScopeTestTarget(ctx, rule.ProxmoxScope)
		eval(models.Host{ID: targetID, Name: targetLabel, Status: "online", LastSeen: time.Now()})
	case models.AlertSourceDocker:
		for _, target := range s.engine.BuildDockerTargets(ctx, rule) {
			eval(target)
		}
	default:
		hosts, err := s.repo.GetAllHosts(ctx)
		if err != nil {
			return nil, false, apperr.Failed("failed to fetch hosts")
		}
		for _, host := range hosts {
			if rule.HostID != nil && *rule.HostID != host.ID {
				continue
			}
			eval(host)
		}
	}

	if results == nil {
		results = []TestRunResult{}
	}
	return results, anyFires, nil
}

// TestRunLogs returns the log lines used to evaluate proxmox_auth_failures_recent
// (and the lookback start), for export.
func (s *Service) TestRunLogs(ctx context.Context, in TestRunInput) ([]string, time.Time, error) {
	if in.SourceType == "" {
		in.SourceType = models.InferAlertSourceType(in.Metric)
	}
	if in.Metric != "proxmox_auth_failures_recent" {
		return nil, time.Time{}, apperr.Validation("Metrique non supportee pour les logs.")
	}

	rule := in.toRule()
	rule.DockerScope = nil // logs preview is proxmox-only

	if rule.SourceType != models.AlertSourceProxmox {
		return nil, time.Time{}, apperr.Validation("La metrique requiert une source Proxmox.")
	}
	if err := s.ValidateProxmoxScope(ctx, rule.ProxmoxScope); err != nil {
		return nil, time.Time{}, err
	}

	lines, since := s.engine.FetchProxmoxLogs(ctx, rule)
	return lines, since, nil
}
