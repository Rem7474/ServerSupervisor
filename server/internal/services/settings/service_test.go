package settings

import (
	"context"
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	deletedAudit int64
	auditActions []string
	setCalls     map[string]string
}

func (fakeRepo) GetAllSettings(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}
func (f *fakeRepo) SetSetting(_ context.Context, key, value string) error {
	if f.setCalls == nil {
		f.setCalls = map[string]string{}
	}
	f.setCalls[key] = value
	return nil
}
func (fakeRepo) UpdateMetricsRetentionPolicy(context.Context, int) error { return nil }
func (fakeRepo) CleanupTrackerTagDigests(context.Context, int) (int64, error) {
	return 3, nil
}
func (f *fakeRepo) CleanOldAuditLogs(context.Context, int) (int64, error) {
	return f.deletedAudit, nil
}
func (f *fakeRepo) CreateAuditLog(_ context.Context, _, action, _, _, _, _ string) (int64, error) {
	f.auditActions = append(f.auditActions, action)
	return 1, nil
}
func (fakeRepo) CountAuditLogs(context.Context) (int64, error) { return 10, nil }
func (fakeRepo) CountMetrics(context.Context) (int64, error)   { return 20, nil }
func (fakeRepo) CountHosts(context.Context) (int64, error)     { return 2, nil }
func (fakeRepo) Ping() error                                   { return nil }

func newSvc(repo Repository, cfg *config.Config) *Service {
	return NewService(repo, cfg, func() string { return "v9.9" })
}

func TestSnapshot_IncludesVersionAndDBStatus(t *testing.T) {
	snap := newSvc(&fakeRepo{}, &config.Config{SMTPHost: "mail.example.com"}).Snapshot(context.Background())
	settings := snap["settings"].(map[string]any)
	if settings["latestAgentVersion"] != "v9.9" {
		t.Errorf("latestAgentVersion = %v, want v9.9", settings["latestAgentVersion"])
	}
	if settings["smtpConfigured"] != true {
		t.Errorf("smtpConfigured = %v, want true", settings["smtpConfigured"])
	}
	db := snap["dbStatus"].(map[string]any)
	if db["connected"] != true || db["hostsCount"] != int64(2) {
		t.Errorf("unexpected dbStatus: %v", db)
	}
}

func TestTestSMTP_NotConfigured(t *testing.T) {
	_, err := newSvc(&fakeRepo{}, &config.Config{}).TestSMTP(context.Background())
	if !isValidation(err) {
		t.Errorf("unconfigured SMTP should be apperr 400, got %v", err)
	}
}

func TestTestNtfy_NotConfigured(t *testing.T) {
	_, err := newSvc(&fakeRepo{}, &config.Config{}).TestNtfy(context.Background())
	if !isValidation(err) {
		t.Errorf("unconfigured ntfy should be apperr 400, got %v", err)
	}
}

func TestCleanupAuditLogs_ReturnsCountAndAudits(t *testing.T) {
	repo := &fakeRepo{deletedAudit: 5}
	deleted, message, err := newSvc(repo, &config.Config{AuditRetentionDays: 30}).CleanupAuditLogs(context.Background(), "alice", "1.2.3.4")
	if err != nil {
		t.Fatalf("CleanupAuditLogs: %v", err)
	}
	if deleted != 5 || message == "" {
		t.Errorf("deleted=%d message=%q", deleted, message)
	}
	if len(repo.auditActions) != 1 || repo.auditActions[0] != "cleanup_audit_logs" {
		t.Errorf("expected a cleanup_audit_logs audit entry, got %v", repo.auditActions)
	}
}

// Regression test: saving one settings tab (e.g. Rétention, which only sends
// metrics_retention_days/audit_retention_days) must not blank SMTP/notification
// config a different tab had previously saved, just because this request's
// SMTP/notification fields decode to their Go zero value.
func TestUpdate_PartialRequestDoesNotBlankSMTPOrNotificationFields(t *testing.T) {
	repo := &fakeRepo{}
	svc := newSvc(repo, &config.Config{})

	svc.Update(context.Background(), models.SettingsUpdateRequest{MetricsRetentionDays: 30}, "alice", "1.2.3.4")

	for _, key := range []string{"smtp_host", "smtp_user", "smtp_pass", "smtp_from", "smtp_to", "ntfy_url", "github_token"} {
		if _, ok := repo.setCalls[key]; ok {
			t.Errorf("Update wrote %q from a request that never provided it — would blank a previously saved value", key)
		}
	}
	if repo.setCalls["metrics_retention_days"] != "30" {
		t.Errorf("metrics_retention_days = %q, want 30", repo.setCalls["metrics_retention_days"])
	}
}

func TestUpdate_SavesProvidedSMTPFields(t *testing.T) {
	repo := &fakeRepo{}
	svc := newSvc(repo, &config.Config{})

	svc.Update(context.Background(), models.SettingsUpdateRequest{
		SMTPHost: "smtp.example.com",
		SMTPUser: "alice",
		SMTPPass: "secret",
	}, "alice", "1.2.3.4")

	if repo.setCalls["smtp_host"] != "smtp.example.com" {
		t.Errorf("smtp_host = %q, want smtp.example.com", repo.setCalls["smtp_host"])
	}
	if repo.setCalls["smtp_user"] != "alice" {
		t.Errorf("smtp_user = %q, want alice", repo.setCalls["smtp_user"])
	}
	if repo.setCalls["smtp_pass"] != "secret" {
		t.Errorf("smtp_pass = %q, want secret", repo.setCalls["smtp_pass"])
	}
}

func isValidation(err error) bool {
	var ae *apperr.Error
	return errors.As(err, &ae) && ae.HTTPStatus == 400
}
