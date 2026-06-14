package notifications

import (
	"context"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	items      []models.NotificationItem
	rules      []models.AlertRule
	readAt     *time.Time
	upsertedAt *time.Time
}

func (f *fakeRepo) GetRecentNotifications(context.Context, int) ([]models.NotificationItem, error) {
	return f.items, nil
}
func (f *fakeRepo) GetNotificationReadAt(context.Context, string) (*time.Time, error) {
	return f.readAt, nil
}
func (f *fakeRepo) UpsertNotificationReadAt(_ context.Context, _ string, readAt time.Time) error {
	f.upsertedAt = &readAt
	return nil
}
func (f *fakeRepo) GetAlertRules(context.Context) ([]models.AlertRule, error) {
	return f.rules, nil
}

func ruleID(id int64) *int64 { return &id }

func TestRecent_EnrichesActiveIncident(t *testing.T) {
	repo := &fakeRepo{
		items: []models.NotificationItem{
			{Type: "alert_incident", RuleID: ruleID(7), Severity: "crit", HostID: "h1"},
		},
		rules: []models.AlertRule{{ID: 7, Operator: ">"}},
	}
	called := false
	svc := NewService(repo, func(context.Context, models.AlertRule, string) (float64, bool) {
		called = true
		return 42, true
	})
	items, _, err := svc.Recent(context.Background(), "alice", 30)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if !called {
		t.Fatal("incident value resolver should be called for an active incident")
	}
	if items[0].Operator != ">" {
		t.Errorf("Operator = %q, want >", items[0].Operator)
	}
	if items[0].CurrentValue == nil || *items[0].CurrentValue != 42 {
		t.Errorf("CurrentValue = %v, want 42", items[0].CurrentValue)
	}
}

func TestRecent_NoActiveIncidentSkipsResolver(t *testing.T) {
	resolved := time.Now()
	repo := &fakeRepo{
		items: []models.NotificationItem{
			{Type: "alert_incident", RuleID: ruleID(7), ResolvedAt: &resolved},
			{Type: "release_tracker_detected"},
		},
	}
	called := false
	svc := NewService(repo, func(context.Context, models.AlertRule, string) (float64, bool) {
		called = true
		return 0, false
	})
	if _, _, err := svc.Recent(context.Background(), "alice", 30); err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if called {
		t.Error("resolver must not be called when no incident is active")
	}
}

func TestMarkRead(t *testing.T) {
	repo := &fakeRepo{}
	readAt, err := NewService(repo, nil).MarkRead(context.Background(), "alice")
	if err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if repo.upsertedAt == nil || !repo.upsertedAt.Equal(readAt) {
		t.Errorf("persisted read_at %v != returned %v", repo.upsertedAt, readAt)
	}
}
