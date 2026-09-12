package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryFindParam(t *testing.T) {
	// Canonical key
	p, ok := FindParam("SERVER_PORT")
	if !ok || p.Key != "SERVER_PORT" {
		t.Fatalf("expected to find SERVER_PORT")
	}

	// Lowercase setting key
	p, ok = FindParam("server_port")
	if !ok || p.Key != "SERVER_PORT" {
		t.Fatalf("expected to find SERVER_PORT via lowercase setting key")
	}

	// Custom setting key lookup
	p, ok = FindParam("ntfy_url")
	if !ok || p.Key != "NOTIFY_URL" {
		t.Fatalf("expected to find NOTIFY_URL via ntfy_url")
	}

	// Unknown key
	_, ok = FindParam("UNKNOWN_PARAM_XYZ")
	if ok {
		t.Fatalf("expected unknown key to return false")
	}
}

// TestEveryParamHasAFrontendTranslation mirrors
// apperr/catalog_test.go's TestEveryCodeHasAFrontendTranslation: AllParams's
// Label/Description are plain French strings with no i18n awareness of
// their own (unlike apperr's catalog, which is bilingual in Go) — the SPA
// translates them through config.json's params.<KEY>.label/.description
// (SettingsAdvancedConfigCard.vue's paramLabel/paramDescription), falling
// back to the raw Go text only for a key that predates its translation.
// Without this test, that fallback silently masks a forgotten translation
// instead of failing a build.
func TestEveryParamHasAFrontendTranslation(t *testing.T) {
	for _, lang := range []string{"en", "fr"} {
		path := filepath.Join("..", "..", "..", "frontend", "src", "locales", lang, "config.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("frontend locales not available (%v)", err)
		}
		var doc struct {
			Params map[string]struct {
				Label       string `json:"label"`
				Description string `json:"description"`
			} `json:"params"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, p := range AllParams {
			entry, ok := doc.Params[p.Key]
			if !ok {
				t.Errorf("key %q has no entry in frontend/src/locales/%s/config.json's params", p.Key, lang)
				continue
			}
			if entry.Label == "" {
				t.Errorf("key %q has an empty label in frontend/src/locales/%s/config.json", p.Key, lang)
			}
			if entry.Description == "" {
				t.Errorf("key %q has an empty description in frontend/src/locales/%s/config.json", p.Key, lang)
			}
		}
	}
}

func TestValidators(t *testing.T) {
	// validatePort
	if err := validatePort("8080"); err != nil {
		t.Errorf("validatePort(8080) failed: %v", err)
	}
	if err := validatePort("0"); err == nil {
		t.Errorf("expected error for port 0")
	}
	if err := validatePort("70000"); err == nil {
		t.Errorf("expected error for port 70000")
	}
	if err := validatePort("abc"); err == nil {
		t.Errorf("expected error for invalid port abc")
	}

	// validateURL
	if err := validateURL(""); err != nil {
		t.Errorf("empty URL should be valid: %v", err)
	}
	if err := validateURL("https://example.com:8080/path"); err != nil {
		t.Errorf("valid URL failed: %v", err)
	}
	if err := validateURL("not-a-url"); err == nil {
		t.Errorf("expected error for not-a-url")
	}

	// validateBool
	if err := validateBool("true"); err != nil {
		t.Errorf("validateBool(true) failed: %v", err)
	}
	if err := validateBool("false"); err != nil {
		t.Errorf("validateBool(false) failed: %v", err)
	}
	if err := validateBool("1"); err != nil {
		t.Errorf("validateBool(1) failed: %v", err)
	}
	if err := validateBool("0"); err != nil {
		t.Errorf("validateBool(0) failed: %v", err)
	}
	if err := validateBool("invalid"); err == nil {
		t.Errorf("expected error for invalid bool")
	}

	// validatePositiveInt
	if err := validatePositiveInt("42"); err != nil {
		t.Errorf("validatePositiveInt(42) failed: %v", err)
	}
	if err := validatePositiveInt("0"); err == nil {
		t.Errorf("expected error for 0")
	}
	if err := validatePositiveInt("-5"); err == nil {
		t.Errorf("expected error for -5")
	}
	if err := validatePositiveInt("notanumber"); err == nil {
		t.Errorf("expected error for non number")
	}

	// validateDuration
	if err := validateDuration("15m"); err != nil {
		t.Errorf("validateDuration(15m) failed: %v", err)
	}
	if err := validateDuration("24h"); err != nil {
		t.Errorf("validateDuration(24h) failed: %v", err)
	}
	if err := validateDuration("invalid"); err == nil {
		t.Errorf("expected error for invalid duration")
	}

	// validateOptions
	optVal := validateOptions([]string{"debug", "info", "warn"})
	if err := optVal("info"); err != nil {
		t.Errorf("valid option failed: %v", err)
	}
	if err := optVal("INFO"); err != nil {
		t.Errorf("case-insensitive option failed: %v", err)
	}
	if err := optVal("critical"); err == nil {
		t.Errorf("expected error for invalid option")
	}

	// validatePositiveFloat
	if err := validatePositiveFloat("3.14"); err != nil {
		t.Errorf("validatePositiveFloat failed: %v", err)
	}
	if err := validatePositiveFloat("0"); err == nil {
		t.Errorf("expected error for 0")
	}
	if err := validatePositiveFloat("-1.5"); err == nil {
		t.Errorf("expected error for negative float")
	}
	if err := validatePositiveFloat("abc"); err == nil {
		t.Errorf("expected error for abc")
	}

	// validateNonNegativeFloat
	if err := validateNonNegativeFloat("0"); err != nil {
		t.Errorf("validateNonNegativeFloat(0) failed: %v", err)
	}
	if err := validateNonNegativeFloat("5.5"); err != nil {
		t.Errorf("validateNonNegativeFloat(5.5) failed: %v", err)
	}
	if err := validateNonNegativeFloat("-1"); err == nil {
		t.Errorf("expected error for -1")
	}
	if err := validateNonNegativeFloat("invalid"); err == nil {
		t.Errorf("expected error for invalid")
	}

	// ValidateIPOrCIDR
	if err := ValidateIPOrCIDR(""); err != nil {
		t.Errorf("empty CIDR failed: %v", err)
	}
	if err := ValidateIPOrCIDR("192.168.1.1, 10.0.0.0/24"); err != nil {
		t.Errorf("valid IPs/CIDRs failed: %v", err)
	}
	if err := ValidateIPOrCIDR("999.999.999.999"); err == nil {
		t.Errorf("expected error for invalid IP")
	}
}
