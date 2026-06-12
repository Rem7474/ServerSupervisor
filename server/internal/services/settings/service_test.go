package settings

import (
	"context"
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
)

type fakeRepo struct {
	deletedAudit int64
	auditActions []string
}

func (fakeRepo) GetAllSettings(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}
func (fakeRepo) SetSetting(context.Context, string, string) error        { return nil }
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

func isValidation(err error) bool {
	var ae *apperr.Error
	return errors.As(err, &ae) && ae.HTTPStatus == 400
}
