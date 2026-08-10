package alertrule

import (
	"context"
	"database/sql"
	"errors"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

// isTemplatableMetric rejects Docker/Proxmox/synthetic metrics: Docker scope
// requires a host_id per rule, Proxmox scope is cluster-level already (no
// per-host axis), and the two synthetic metrics (uptime_down_count,
// ssl_min_days_remaining) evaluate globally, once per rule, not per host
// (see internal/alerts/engine.go's isSyntheticMetric) — none of the three
// fit "apply the same recipe to N hosts."
func isTemplatableMetric(metric string) bool {
	if models.IsDockerMetric(metric) || models.IsProxmoxMetric(metric) {
		return false
	}
	return metric != "uptime_down_count" && metric != "ssl_min_days_remaining"
}

func validateTemplateRequest(req *models.AlertRuleTemplateRequest) error {
	if err := validateAlertRuleMetricOperator(req.Metric, req.Operator); err != nil {
		return err
	}
	if !isTemplatableMetric(req.Metric) {
		return apperr.Validation("Les métriques Docker, Proxmox ou synthétiques ne peuvent pas être utilisées dans un template — elles ne s'appliquent pas hôte par hôte.")
	}
	return validateAlertActions(&req.Actions)
}

// CreateTemplate validates and persists a new reusable rule template.
func (s *Service) CreateTemplate(ctx context.Context, req models.AlertRuleTemplateRequest) (*models.AlertRuleTemplate, error) {
	if err := validateTemplateRequest(&req); err != nil {
		return nil, err
	}
	if req.Actions.Channels == nil {
		req.Actions.Channels = []string{}
	}
	t := &models.AlertRuleTemplate{
		Name: req.Name, Metric: req.Metric, Operator: req.Operator,
		ThresholdWarn: req.ThresholdWarn, ThresholdCrit: req.ThresholdCrit,
		ThresholdClearWarn: req.ThresholdClearWarn, ThresholdClearCrit: req.ThresholdClearCrit,
		DurationSeconds: req.Duration, Actions: req.Actions,
	}
	if err := s.repo.CreateAlertRuleTemplate(ctx, t); err != nil {
		return nil, apperr.Failed("failed to create template: " + err.Error())
	}
	return t, nil
}

// ListTemplates returns every template (never nil).
func (s *Service) ListTemplates(ctx context.Context) ([]models.AlertRuleTemplate, error) {
	templates, err := s.repo.GetAlertRuleTemplates(ctx)
	if err != nil {
		return nil, err
	}
	if templates == nil {
		templates = []models.AlertRuleTemplate{}
	}
	return templates, nil
}

// GetTemplate returns a template by id, or apperr.NotFound when absent.
func (s *Service) GetTemplate(ctx context.Context, id int64) (*models.AlertRuleTemplate, error) {
	t, err := s.repo.GetAlertRuleTemplateByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("template not found")
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

// UpdateTemplate validates and applies changes to an existing template. Rules
// already created from it are untouched (see AlertRuleTemplate's doc comment).
func (s *Service) UpdateTemplate(ctx context.Context, id int64, req models.AlertRuleTemplateRequest) (*models.AlertRuleTemplate, error) {
	if err := validateTemplateRequest(&req); err != nil {
		return nil, err
	}
	if _, err := s.GetTemplate(ctx, id); err != nil {
		return nil, err
	}
	if req.Actions.Channels == nil {
		req.Actions.Channels = []string{}
	}
	t := &models.AlertRuleTemplate{
		ID: id, Name: req.Name, Metric: req.Metric, Operator: req.Operator,
		ThresholdWarn: req.ThresholdWarn, ThresholdCrit: req.ThresholdCrit,
		ThresholdClearWarn: req.ThresholdClearWarn, ThresholdClearCrit: req.ThresholdClearCrit,
		DurationSeconds: req.Duration, Actions: req.Actions,
	}
	if err := s.repo.UpdateAlertRuleTemplate(ctx, t); err != nil {
		return nil, apperr.Failed("failed to update template: " + err.Error())
	}
	return t, nil
}

// DeleteTemplate removes a template. Rules already created from it are
// untouched — there is no live link between a template and its spawned rules.
func (s *Service) DeleteTemplate(ctx context.Context, id int64) error {
	if _, err := s.GetTemplate(ctx, id); err != nil {
		return err
	}
	return s.repo.DeleteAlertRuleTemplate(ctx, id)
}

// ApplyTemplate stamps out one independent alert rule per requested host,
// reusing Create's exact validation/persistence path — a template row is
// just a saved AlertRuleCreate minus HostID. Not all-or-nothing: one host's
// failure is recorded in the result and doesn't stop the rest.
func (s *Service) ApplyTemplate(ctx context.Context, id int64, req models.ApplyAlertRuleTemplateRequest) (*models.ApplyAlertRuleTemplateResult, error) {
	t, err := s.GetTemplate(ctx, id)
	if err != nil {
		return nil, err
	}
	result := &models.ApplyAlertRuleTemplateResult{Errors: map[string]string{}}
	for _, hostID := range req.HostIDs {
		hostID := hostID
		rule, err := s.Create(ctx, models.AlertRuleCreate{
			Name: t.Name, Enabled: req.Enabled, SourceType: models.AlertSourceAgent, HostID: &hostID,
			Metric: t.Metric, Operator: t.Operator,
			ThresholdWarn: t.ThresholdWarn, ThresholdCrit: t.ThresholdCrit,
			ThresholdClearWarn: t.ThresholdClearWarn, ThresholdClearCrit: t.ThresholdClearCrit,
			Duration: t.DurationSeconds, Actions: t.Actions,
		})
		if err != nil {
			result.Errors[hostID] = err.Error()
			continue
		}
		result.CreatedRuleIDs = append(result.CreatedRuleIDs, rule.ID)
	}
	return result, nil
}
