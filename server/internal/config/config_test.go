package config

import (
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
