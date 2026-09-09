package config

import (
	"os"
	"testing"
)

func TestEffectiveConfigPriorities(t *testing.T) {
	// Clean env for tests
	origSMTPHost, hadSMTPHost := os.LookupEnv("SMTP_HOST")
	defer func() {
		if hadSMTPHost {
			_ = os.Setenv("SMTP_HOST", origSMTPHost)
		} else {
			_ = os.Unsetenv("SMTP_HOST")
		}
	}()

	_ = os.Unsetenv("SMTP_HOST")

	// 1. Default fallback
	summary := ResolveEffectiveConfig(map[string]string{}, false)
	var smtpHostEntry *ConfigEntry
	for i := range summary.Entries {
		if summary.Entries[i].Key == "SMTP_HOST" {
			smtpHostEntry = &summary.Entries[i]
			break
		}
	}
	if smtpHostEntry == nil {
		t.Fatalf("SMTP_HOST parameter not found")
	}
	if smtpHostEntry.Source != "default" {
		t.Errorf("expected source default, got %s", smtpHostEntry.Source)
	}
	if smtpHostEntry.HasEnvOverride {
		t.Errorf("expected HasEnvOverride=false")
	}
	if smtpHostEntry.HasConflict {
		t.Errorf("expected HasConflict=false")
	}

	// 2. UI override
	dbSettings := map[string]string{
		"smtp_host": "mail.example.com",
	}
	summary = ResolveEffectiveConfig(dbSettings, false)
	for i := range summary.Entries {
		if summary.Entries[i].Key == "SMTP_HOST" {
			smtpHostEntry = &summary.Entries[i]
			break
		}
	}
	if smtpHostEntry.Source != "ui" {
		t.Errorf("expected source ui, got %s", smtpHostEntry.Source)
	}
	if smtpHostEntry.EffectiveValue != "mail.example.com" {
		t.Errorf("expected EffectiveValue 'mail.example.com', got '%s'", smtpHostEntry.EffectiveValue)
	}
	if smtpHostEntry.HasEnvOverride {
		t.Errorf("expected HasEnvOverride=false")
	}
	if smtpHostEntry.HasConflict {
		t.Errorf("expected HasConflict=false")
	}

	// 3. ENV override (Priority 1)
	_ = os.Setenv("SMTP_HOST", "env.smtp.corp")
	summary = ResolveEffectiveConfig(dbSettings, false)
	for i := range summary.Entries {
		if summary.Entries[i].Key == "SMTP_HOST" {
			smtpHostEntry = &summary.Entries[i]
			break
		}
	}
	if smtpHostEntry.Source != "env" {
		t.Errorf("expected source env, got %s", smtpHostEntry.Source)
	}
	if smtpHostEntry.EffectiveValue != "env.smtp.corp" {
		t.Errorf("expected EffectiveValue 'env.smtp.corp', got '%s'", smtpHostEntry.EffectiveValue)
	}
	if !smtpHostEntry.HasEnvOverride {
		t.Errorf("expected HasEnvOverride=true")
	}
	// Conflict since UI is mail.example.com and ENV is env.smtp.corp
	if !smtpHostEntry.HasConflict {
		t.Errorf("expected HasConflict=true")
	}
	if summary.ConflictCount != 1 {
		t.Errorf("expected ConflictCount=1, got %d", summary.ConflictCount)
	}

	// 4. Same value in ENV and UI -> no conflict
	dbSettings["smtp_host"] = "env.smtp.corp"
	summary = ResolveEffectiveConfig(dbSettings, false)
	for i := range summary.Entries {
		if summary.Entries[i].Key == "SMTP_HOST" {
			smtpHostEntry = &summary.Entries[i]
			break
		}
	}
	if smtpHostEntry.HasConflict {
		t.Errorf("expected HasConflict=false when env and ui match")
	}
}

func TestSecretMasking(t *testing.T) {
	dbSettings := map[string]string{
		"smtp_pass": "SuperSecretPass123!",
	}

	// Masked
	summary := ResolveEffectiveConfig(dbSettings, false)
	var passEntry *ConfigEntry
	for i := range summary.Entries {
		if summary.Entries[i].Key == "SMTP_PASS" {
			passEntry = &summary.Entries[i]
			break
		}
	}
	if passEntry == nil {
		t.Fatalf("SMTP_PASS not found")
	}
	if passEntry.EffectiveValue != MaskedSecretSentinel {
		t.Errorf("expected masked value '%s', got '%s'", MaskedSecretSentinel, passEntry.EffectiveValue)
	}

	// Revealed
	summaryRevealed := ResolveEffectiveConfig(dbSettings, true)
	for i := range summaryRevealed.Entries {
		if summaryRevealed.Entries[i].Key == "SMTP_PASS" {
			passEntry = &summaryRevealed.Entries[i]
			break
		}
	}
	if passEntry.EffectiveValue != "SuperSecretPass123!" {
		t.Errorf("expected revealed secret 'SuperSecretPass123!', got '%s'", passEntry.EffectiveValue)
	}
}

func TestParamValidation(t *testing.T) {
	// Port
	if err := validatePort("8080"); err != nil {
		t.Errorf("valid port failed: %v", err)
	}
	if err := validatePort("70000"); err == nil {
		t.Errorf("expected error for port > 65535")
	}
	if err := validatePort("invalid"); err == nil {
		t.Errorf("expected error for non-integer port")
	}

	// URL
	if err := validateURL("https://example.com:8080"); err != nil {
		t.Errorf("valid URL failed: %v", err)
	}
	if err := validateURL(""); err != nil {
		t.Errorf("empty optional URL should pass: %v", err)
	}

	// Options
	optValidator := validateOptions([]string{"debug", "info", "warn", "error"})
	if err := optValidator("INFO"); err != nil {
		t.Errorf("valid case-insensitive option failed: %v", err)
	}
	if err := optValidator("verbose"); err == nil {
		t.Errorf("expected error for invalid option")
	}

	// Duration
	if err := validateDuration("15m"); err != nil {
		t.Errorf("valid duration failed: %v", err)
	}
	if err := validateDuration("invalid"); err == nil {
		t.Errorf("expected error for invalid duration")
	}
}

func TestFindParam(t *testing.T) {
	p, ok := FindParam("SERVER_PORT")
	if !ok || p.Key != "SERVER_PORT" {
		t.Errorf("FindParam by Key failed")
	}

	p, ok = FindParam("server_port")
	if !ok || p.Key != "SERVER_PORT" {
		t.Errorf("FindParam by SettingKey failed")
	}

	_, ok = FindParam("NON_EXISTENT_KEY")
	if ok {
		t.Errorf("expected false for unknown key")
	}
}
