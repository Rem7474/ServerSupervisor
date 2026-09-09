package config

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/serversupervisor/server/internal/config"
)

type mockRepo struct {
	settings  map[string]string
	auditLogs []string
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		settings:  make(map[string]string),
		auditLogs: make([]string, 0),
	}
}

func (m *mockRepo) GetAllSettings(_ context.Context) (map[string]string, error) {
	res := make(map[string]string, len(m.settings))
	for k, v := range m.settings {
		res[k] = v
	}
	return res, nil
}

func (m *mockRepo) SetSetting(_ context.Context, key, value string) error {
	m.settings[key] = value
	return nil
}

func (m *mockRepo) DeleteSetting(_ context.Context, key string) error {
	delete(m.settings, key)
	return nil
}

func (m *mockRepo) CreateAuditLog(_ context.Context, username, action, _, _, details, _ string) (int64, error) {
	m.auditLogs = append(m.auditLogs, username+":"+action+":"+details)
	return int64(len(m.auditLogs)), nil
}

func TestConfigService_GetEffectiveConfig(t *testing.T) {
	repo := newMockRepo()
	cfg := config.Load()
	svc := NewService(repo, cfg)

	summary, err := svc.GetEffectiveConfig(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary.TotalParams == 0 {
		t.Errorf("expected params > 0")
	}
}

func TestConfigService_UpdateParam(t *testing.T) {
	repo := newMockRepo()
	cfg := config.Load()
	svc := NewService(repo, cfg)

	// Test validation error (invalid port)
	_, _, err := svc.UpdateParam(context.Background(), "SERVER_PORT", "999999", "admin", "127.0.0.1")
	if err == nil {
		t.Errorf("expected validation error for invalid port")
	}

	// Test non-editable param
	_, _, err = svc.UpdateParam(context.Background(), "DEMO_MODE", "true", "admin", "127.0.0.1")
	if err == nil {
		t.Errorf("expected error when trying to edit DEMO_MODE")
	}

	// Test valid update
	entry, warn, err := svc.UpdateParam(context.Background(), "BASE_URL", "https://supervisor.corp.internal", "admin", "127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry == nil {
		t.Fatalf("expected entry returned")
	}
	if repo.settings["base_url"] != "https://supervisor.corp.internal" {
		t.Errorf("expected setting stored in repo")
	}
	if len(repo.auditLogs) != 1 {
		t.Errorf("expected audit log created")
	}

	// Test warning if ENV is set
	_ = os.Setenv("BASE_URL", "http://env-url:8080")
	defer func() { _ = os.Unsetenv("BASE_URL") }()

	_, warn, err = svc.UpdateParam(context.Background(), "BASE_URL", "https://new-url.corp", "admin", "127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if warn == "" {
		t.Errorf("expected warning when updating param that has active ENV")
	}
}

func TestConfigService_SecretAuditMasking(t *testing.T) {
	repo := newMockRepo()
	cfg := config.Load()
	svc := NewService(repo, cfg)

	_, _, err := svc.UpdateParam(context.Background(), "SMTP_PASS", "SuperP@ssw0rd!", "admin", "127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.auditLogs) != 1 {
		t.Fatalf("expected audit log created")
	}
	if strings.Contains(repo.auditLogs[0], "SuperP@ssw0rd!") {
		t.Errorf("audit log exposed plaintext password: %s", repo.auditLogs[0])
	}
	if !strings.Contains(repo.auditLogs[0], "[REDACTED]") {
		t.Errorf("expected [REDACTED] in audit log: %s", repo.auditLogs[0])
	}
}

func TestConfigService_ResetParam(t *testing.T) {
	repo := newMockRepo()
	repo.settings["smtp_host"] = "mail.mycorp.com"
	cfg := config.Load()
	svc := NewService(repo, cfg)

	entry, err := svc.ResetParam(context.Background(), "SMTP_HOST", "admin", "127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry == nil {
		t.Fatalf("expected entry returned")
	}
	if _, exists := repo.settings["smtp_host"]; exists {
		t.Errorf("expected setting to be deleted from repo")
	}
}

func TestConfigService_UpdateBulk(t *testing.T) {
	repo := newMockRepo()
	cfg := config.Load()
	svc := NewService(repo, cfg)

	updates := map[string]string{
		"SMTP_HOST": "smtp.domain.tld",
		"SMTP_PORT": "587",
	}

	summary, _, err := svc.UpdateBulk(context.Background(), updates, "admin", "127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.TotalParams == 0 {
		t.Errorf("expected summary with entries")
	}
	if repo.settings["smtp_host"] != "smtp.domain.tld" {
		t.Errorf("expected smtp_host updated")
	}
	if repo.settings["smtp_port"] != "587" {
		t.Errorf("expected smtp_port updated")
	}
}
