package config

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestValidateStrict_AllowsDevDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	c := &Config{
		JWTSecret:     DefaultJWTSecret,
		AdminPassword: "admin",
		DBPassword:    "supervisor",
	}
	if err := c.ValidateStrict(); err != nil {
		t.Fatalf("dev env should bypass strict validation, got %v", err)
	}
}

func TestValidateStrict_RejectsDefaultJWTSecretInProd(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	c := &Config{
		JWTSecret:     DefaultJWTSecret,
		AdminPassword: "strong",
		DBPassword:    "strong",
	}
	err := c.ValidateStrict()
	if err == nil {
		t.Fatal("expected ValidateStrict to fail with default JWT secret")
	}
	if !errors.Is(err, ErrInsecureConfig) {
		t.Fatalf("expected ErrInsecureConfig, got %v", err)
	}
	if !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("expected JWT_SECRET in error, got %v", err)
	}
}

func TestValidateStrict_RejectsShortSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	c := &Config{
		JWTSecret:     "tooshort",
		AdminPassword: "strong",
		DBPassword:    "strong",
	}
	if err := c.ValidateStrict(); err == nil {
		t.Fatal("expected ValidateStrict to fail on short secret")
	}
}

func TestValidateStrict_RejectsDefaultAdminAndDBPassword(t *testing.T) {
	t.Setenv("APP_ENV", "")
	long := strings.Repeat("a", 64)

	cases := map[string]Config{
		"default admin password": {JWTSecret: long, AdminPassword: "admin", DBPassword: "strong"},
		"short admin password":   {JWTSecret: long, AdminPassword: "short", DBPassword: "strong"},
		"default DB password":    {JWTSecret: long, AdminPassword: "strong", DBPassword: "supervisor"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if err := c.ValidateStrict(); err == nil {
				t.Fatal("expected ValidateStrict to fail")
			}
		})
	}
}

func TestValidateStrict_AcceptsEmptyAdminAndJWTSecretForFirstRun(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	c := &Config{
		JWTSecret:     "",
		AdminPassword: "",
		DBPassword:    "strong-db-pass",
	}
	if err := c.ValidateStrict(); err != nil {
		t.Fatalf("expected no error for empty admin/jwt (auto-generated at boot), got %v", err)
	}
}

func TestValidateStrict_AcceptsStrongConfig(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	c := &Config{
		JWTSecret:     strings.Repeat("a", 64),
		AdminPassword: "a-strong-password",
		DBPassword:    "a-strong-db-password",
	}
	if err := c.ValidateStrict(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestGenerateRandomPassword(t *testing.T) {
	p1 := GenerateRandomPassword(16)
	p2 := GenerateRandomPassword(16)
	if len(p1) != 16 || len(p2) != 16 {
		t.Fatalf("expected password length 16, got %d and %d", len(p1), len(p2))
	}
	if p1 == p2 {
		t.Fatal("consecutive random passwords should not match")
	}
	// Edge cases
	if pZero := GenerateRandomPassword(0); pZero != "" {
		t.Fatalf("expected empty string for length 0, got %q", pZero)
	}
	if pLong := GenerateRandomPassword(128); len(pLong) != 128 {
		t.Fatalf("expected length 128, got %d", len(pLong))
	}
}

func TestLoad_GeneratesRandomSecretInDev(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("JWT_SECRET", "")

	c1 := Load()
	c2 := Load()

	if c1.JWTSecret == DefaultJWTSecret {
		t.Fatal("dev mode should auto-generate, not fall back to DefaultJWTSecret")
	}
	if len(c1.JWTSecret) < 32 {
		t.Fatalf("generated secret too short: %d", len(c1.JWTSecret))
	}
	if c1.JWTSecret == c2.JWTSecret {
		t.Fatal("two Load() calls in dev mode must produce different ephemeral secrets")
	}
}

func TestLoad_KeepsExplicitSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	explicit := strings.Repeat("z", 64)
	t.Setenv("JWT_SECRET", explicit)

	c := Load()
	if c.JWTSecret != explicit {
		t.Fatalf("expected explicit secret to be kept, got %q", c.JWTSecret)
	}
}

func TestIsDevEnv(t *testing.T) {
	cases := map[string]bool{
		"dev":         true,
		"development": true,
		"DEV":         true,
		"production":  false,
		"":            false,
		"staging":     false,
	}
	for v, want := range cases {
		t.Run(v, func(t *testing.T) {
			t.Setenv("APP_ENV", v)
			if got := IsDevEnv(); got != want {
				t.Fatalf("IsDevEnv()=%v want %v", got, want)
			}
		})
	}
}

func TestLoad_RateLimitRPSFromEnv(t *testing.T) {
	t.Setenv("RATE_LIMIT_RPS", "150")
	c := Load()
	if c.RateLimitRPS != 150 {
		t.Fatalf("expected RateLimitRPS=150, got %d", c.RateLimitRPS)
	}
}

func TestOIDCConfig_LoadAndValidate(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_DISPLAY_NAME", "Authentik SSO")
	t.Setenv("OIDC_ISSUER_URL", "https://auth.example.com/application/o/ss/")
	t.Setenv("OIDC_CLIENT_ID", "ss-client-id")
	t.Setenv("OIDC_CLIENT_SECRET", "ss-secret")
	t.Setenv("OIDC_REDIRECT_URL", "https://ss.example.com/api/auth/oidc/callback")
	t.Setenv("OIDC_SCOPES", "openid,profile,email,groups")
	t.Setenv("OIDC_ADMIN_GROUP", "admins")
	t.Setenv("OIDC_OPERATOR_GROUP", "operators")
	t.Setenv("OIDC_VIEWER_GROUP", "viewers")
	t.Setenv("OIDC_DEFAULT_ROLE", "viewer")
	t.Setenv("OIDC_AUTO_CREATE_USER", "true")
	t.Setenv("OIDC_ALLOW_LOCAL_LOGIN", "false")
	t.Setenv("OIDC_INSECURE_SKIP_VERIFY", "false")

	c := Load()
	if !c.OIDCEnabled || c.OIDCDisplayName != "Authentik SSO" || c.OIDCIssuerURL != "https://auth.example.com/application/o/ss/" {
		t.Fatalf("unexpected OIDC config loaded: %+v", c)
	}
	if len(c.OIDCScopes) != 4 || c.OIDCScopes[3] != "groups" {
		t.Fatalf("unexpected OIDC scopes: %v", c.OIDCScopes)
	}
	if c.OIDCAllowLocalLogin != false || c.OIDCAutoCreateUser != true {
		t.Fatalf("unexpected flags: allow_local=%v auto_create=%v", c.OIDCAllowLocalLogin, c.OIDCAutoCreateUser)
	}

	c.JWTSecret = strings.Repeat("x", 64)
	c.AdminPassword = "strong-admin-pass"
	c.DBPassword = "strong-db-pass"
	if err := c.ValidateStrict(); err != nil {
		t.Fatalf("valid OIDC config should pass ValidateStrict, got %v", err)
	}

	// Test missing OIDC_ISSUER_URL
	cMissingIssuer := *c
	cMissingIssuer.OIDCIssuerURL = ""
	if err := cMissingIssuer.ValidateStrict(); err == nil {
		t.Fatal("expected error on missing OIDC_ISSUER_URL")
	}

	// Test missing OIDC_CLIENT_ID
	cMissingClient := *c
	cMissingClient.OIDCClientID = ""
	if err := cMissingClient.ValidateStrict(); err == nil {
		t.Fatal("expected error on missing OIDC_CLIENT_ID")
	}

	// Test missing OIDC_REDIRECT_URL
	cMissingRedir := *c
	cMissingRedir.OIDCRedirectURL = ""
	if err := cMissingRedir.ValidateStrict(); err == nil {
		t.Fatal("expected error on missing OIDC_REDIRECT_URL")
	}
}

type mockSettingsLoader map[string]string

func (m mockSettingsLoader) GetAllSettings(ctx context.Context) (map[string]string, error) {
	return m, nil
}

func TestOIDCConfig_OverrideFromDB(t *testing.T) {
	c := &Config{
		OIDCEnabled:     false,
		OIDCDisplayName: "Default",
	}

	dbSettings := mockSettingsLoader{
		"oidc_enabled":              "true",
		"oidc_display_name":         "Custom DB SSO",
		"oidc_issuer_url":           "https://custom-auth.example.com",
		"oidc_client_id":            "custom-client",
		"oidc_client_secret":        "custom-secret",
		"oidc_redirect_url":         "https://custom.example.com/callback",
		"oidc_scopes":               "openid,email",
		"oidc_username_claim":       "preferred_username",
		"oidc_email_claim":          "email",
		"oidc_groups_claim":         "groups",
		"oidc_admin_group":          "db-admins",
		"oidc_operator_group":       "db-ops",
		"oidc_viewer_group":         "db-viewers",
		"oidc_default_role":         "operator",
		"oidc_auto_create_user":     "false",
		"oidc_allow_local_login":    "true",
		"oidc_insecure_skip_verify": "true",
	}

	c.OverrideFromDB(dbSettings)

	if !c.OIDCEnabled || c.OIDCDisplayName != "Custom DB SSO" || c.OIDCIssuerURL != "https://custom-auth.example.com" {
		t.Fatalf("unexpected overrides: %+v", c)
	}
	if len(c.OIDCScopes) != 2 || c.OIDCAdminGroup != "db-admins" || c.OIDCDefaultRole != "operator" {
		t.Fatalf("unexpected OIDC claims/groups: %+v", c)
	}
	if c.OIDCAutoCreateUser != false || c.OIDCInsecureSkipVerify != true {
		t.Fatalf("unexpected flags after override: %+v", c)
	}
}

func TestGetCSVEnvOrDefault(t *testing.T) {
	t.Setenv("TEST_CSV_KEY", "apple, banana , cherry")
	res := getCSVEnvOrDefault("TEST_CSV_KEY", []string{"default"})
	if len(res) != 3 || res[0] != "apple" || res[1] != "banana" || res[2] != "cherry" {
		t.Fatalf("unexpected csv parsed: %v", res)
	}

	t.Setenv("TEST_EMPTY_CSV", "")
	resDef := getCSVEnvOrDefault("TEST_EMPTY_CSV", []string{"foo", "bar"})
	if len(resDef) != 2 || resDef[0] != "foo" {
		t.Fatalf("unexpected default csv: %v", resDef)
	}
}
