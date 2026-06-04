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

func TestOverrideFromDB_LoaderErrorIsNoop(t *testing.T) {
	c := &Config{SMTPHost: "env-host"}
	c.OverrideFromDB(fakeSettingsLoader{err: context.DeadlineExceeded})
	if c.SMTPHost != "env-host" {
		t.Errorf("SMTPHost = %q, want env-host (loader error must leave config untouched)", c.SMTPHost)
	}
}
