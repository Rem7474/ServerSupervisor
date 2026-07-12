package config

import (
	"context"
	"testing"
)

// fakeSettingsLoader implements DBSettingsLoader from an in-memory map.
type fakeSettingsLoader struct {
	settings map[string]string
	err      error
}

func (f fakeSettingsLoader) GetAllSettings(_ context.Context) (map[string]string, error) {
	return f.settings, f.err
}

func TestOverrideFromDB_AppliesPersistedSettings(t *testing.T) {
	c := &Config{
		SMTPHost:             "env-host",
		SMTPPort:             25,
		NotifyURL:            "env-ntfy",
		GitHubToken:          "env-token",
		MetricsRetentionDays: 30,
		AuditRetentionDays:   90,
		WebLogsRetentionDays: 30,
	}

	c.OverrideFromDB(fakeSettingsLoader{settings: map[string]string{
		"smtp_host":               "db-host",
		"smtp_port":               "587",
		"smtp_tls":                "true",
		"ntfy_url":                "db-ntfy",
		"github_token":            "db-token",
		"metrics_retention_days":  "60",
		"audit_retention_days":    "120",
		"web_logs_retention_days": "45",
	}})

	if c.SMTPHost != "db-host" {
		t.Errorf("SMTPHost = %q, want db-host", c.SMTPHost)
	}
	if c.SMTPPort != 587 {
		t.Errorf("SMTPPort = %d, want 587", c.SMTPPort)
	}
	if !c.SMTPTLS {
		t.Error("SMTPTLS = false, want true")
	}
	if c.NotifyURL != "db-ntfy" {
		t.Errorf("NotifyURL = %q, want db-ntfy", c.NotifyURL)
	}
	if c.GitHubToken != "db-token" {
		t.Errorf("GitHubToken = %q, want db-token", c.GitHubToken)
	}
	if c.MetricsRetentionDays != 60 || c.AuditRetentionDays != 120 || c.WebLogsRetentionDays != 45 {
		t.Errorf("retention = (%d,%d,%d), want (60,120,45)", c.MetricsRetentionDays, c.AuditRetentionDays, c.WebLogsRetentionDays)
	}
}

func TestOverrideFromDB_KeepsEnvWhenAbsentOrInvalid(t *testing.T) {
	c := &Config{
		SMTPHost:             "env-host",
		SMTPPort:             25,
		MetricsRetentionDays: 30,
	}

	c.OverrideFromDB(fakeSettingsLoader{settings: map[string]string{
		"smtp_host":              "",   // empty -> ignored, env kept
		"smtp_port":              "xx", // invalid int -> ignored
		"metrics_retention_days": "",   // empty -> ignored
		// absent keys keep env values
	}})

	if c.SMTPHost != "env-host" {
		t.Errorf("SMTPHost = %q, want env-host (empty DB value must not override)", c.SMTPHost)
	}
	if c.SMTPPort != 25 {
		t.Errorf("SMTPPort = %d, want 25 (invalid DB value must not override)", c.SMTPPort)
	}
	if c.MetricsRetentionDays != 30 {
		t.Errorf("MetricsRetentionDays = %d, want 30", c.MetricsRetentionDays)
	}
}

// TestOverrideFromDB_KeepsSecretsWhenBlank guards against a regression where an
// admin saving the Settings form without re-entering a secret field (a common
// UX pattern for password inputs) silently wipes a working env-var-configured
// secret. smtp_user/smtp_pass/ntfy_url/github_token must behave exactly like
// smtp_host: an empty persisted value must never override a non-empty one.
func TestOverrideFromDB_KeepsSecretsWhenBlank(t *testing.T) {
	c := &Config{
		SMTPUser:    "env-user",
		SMTPPass:    "env-pass",
		NotifyURL:   "env-ntfy",
		GitHubToken: "env-token",
	}

	c.OverrideFromDB(fakeSettingsLoader{settings: map[string]string{
		"smtp_user":    "",
		"smtp_pass":    "",
		"ntfy_url":     "",
		"github_token": "",
	}})

	if c.SMTPUser != "env-user" {
		t.Errorf("SMTPUser = %q, want env-user (empty DB value must not override)", c.SMTPUser)
	}
	if c.SMTPPass != "env-pass" {
		t.Errorf("SMTPPass = %q, want env-pass (empty DB value must not override)", c.SMTPPass)
	}
	if c.NotifyURL != "env-ntfy" {
		t.Errorf("NotifyURL = %q, want env-ntfy (empty DB value must not override)", c.NotifyURL)
	}
	if c.GitHubToken != "env-token" {
		t.Errorf("GitHubToken = %q, want env-token (empty DB value must not override)", c.GitHubToken)
	}
}

func TestOverrideFromDB_LoaderErrorIsNoop(t *testing.T) {
	c := &Config{SMTPHost: "env-host"}
	c.OverrideFromDB(fakeSettingsLoader{err: context.DeadlineExceeded})
	if c.SMTPHost != "env-host" {
		t.Errorf("SMTPHost = %q, want env-host (loader error must leave config untouched)", c.SMTPHost)
	}
}
