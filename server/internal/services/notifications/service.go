// Package notifications is the application/service layer for the browser
// notification history. It owns the active-incident enrichment (operator, clear
// threshold, live value) behind a Repository port. The live-value lookup needs
// the concrete *database.DB (alerts.CurrentIncidentValue), so it is injected as a
// func to keep the service decoupled.
package notifications

import (
	"context"
	"time"

	"github.com/serversupervisor/server/internal/alerts"
	"github.com/serversupervisor/server/internal/models"
)

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	GetRecentNotifications(ctx context.Context, limit int) ([]models.NotificationItem, error)
	GetNotificationReadAt(ctx context.Context, username string) (*time.Time, error)
	UpsertNotificationReadAt(ctx context.Context, username string, readAt time.Time) error
	GetAlertRules(ctx context.Context) ([]models.AlertRule, error)
}

// IncidentValueFunc resolves the current metric value for an active incident.
// Wired to alerts.CurrentIncidentValue(ctx, db, rule, hostID).
type IncidentValueFunc func(ctx context.Context, rule models.AlertRule, hostID string) (float64, bool)

// Service holds the notification use-cases.
type Service struct {
	repo          Repository
	incidentValue IncidentValueFunc
}

func NewService(repo Repository, incidentValue IncidentValueFunc) *Service {
	return &Service{repo: repo, incidentValue: incidentValue}
}

// Recent returns the latest notifications (enriched with live incident data) plus
// the caller's server-side read_at marker for cross-device unread sync.
func (s *Service) Recent(ctx context.Context, username string, limit int) ([]models.NotificationItem, *time.Time, error) {
	items, err := s.repo.GetRecentNotifications(ctx, limit)
	if err != nil {
		return nil, nil, err
	}
	if items == nil {
		items = []models.NotificationItem{}
	}
	s.enrichActiveIncidents(ctx, items)

	var readAt *time.Time
	if username != "" {
		readAt, _ = s.repo.GetNotificationReadAt(ctx, username)
	}
	return items, readAt, nil
}

// MarkRead persists the current UTC timestamp as the user's "read up to" marker.
func (s *Service) MarkRead(ctx context.Context, username string) (time.Time, error) {
	readAt := time.Now().UTC()
	if err := s.repo.UpsertNotificationReadAt(ctx, username, readAt); err != nil {
		return time.Time{}, err
	}
	return readAt, nil
}

// enrichActiveIncidents fills Operator / ClearThreshold / CurrentValue on active
// alert incidents so the UI can show the live value and the resolve threshold.
// Rules are fetched once and indexed by ID; it no-ops when nothing is active.
func (s *Service) enrichActiveIncidents(ctx context.Context, items []models.NotificationItem) {
	hasActive := false
	for i := range items {
		if items[i].Type == "alert_incident" && items[i].ResolvedAt == nil && items[i].RuleID != nil {
			hasActive = true
			break
		}
	}
	if !hasActive {
		return
	}

	rules, err := s.repo.GetAlertRules(ctx)
	if err != nil {
		return
	}
	ruleByID := make(map[int64]models.AlertRule, len(rules))
	for _, r := range rules {
		ruleByID[r.ID] = r
	}

	for i := range items {
		item := &items[i]
		if item.Type != "alert_incident" || item.ResolvedAt != nil || item.RuleID == nil {
			continue
		}
		rule, ok := ruleByID[*item.RuleID]
		if !ok {
			continue
		}
		item.Operator = rule.Operator
		if clear := alerts.ResolveThresholdForSeverity(rule, alerts.AlertSeverity(item.Severity)); clear != nil {
			item.ClearThreshold = clear
		}
		if value, ok := s.incidentValue(ctx, rule, item.HostID); ok {
			item.CurrentValue = &value
		}
	}
}
