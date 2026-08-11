package config

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

const DefaultJWTSecret = "change-me-in-production-please"

// ErrInsecureConfig is returned by ValidateStrict when the configuration
// contains values unsafe for production (default JWT secret, default admin
// password, default DB password) and APP_ENV is not "dev"/"development".
var ErrInsecureConfig = errors.New("insecure configuration detected")

type Config struct {
	// Server
	Port       string
	BaseURL    string
	TLSEnabled bool // Whether HTTPS is enabled
	// DemoMode disables every background job/poller that makes a real
	// outbound network call (Proxmox VE, Nginx Proxy Manager, uptime/SSL
	// probes, Git release tracking) — see main.go's job/poller wiring.
	// Orthogonal to APP_ENV=dev (which only relaxes secret validation):
	// a real local dev setup against a real Proxmox box must not have its
	// pollers silently disabled just because APP_ENV=dev is convenient for
	// the JWT secret auto-generation. Env-only by design — never read back
	// from OverrideFromDB, so it can't be flipped at runtime via the
	// Settings UI (that would defeat the "zero network calls" guarantee the
	// demo/screenshot pipeline relies on).
	DemoMode bool

	// Logging
	LogLevel  string // debug|info|warn|error
	LogFormat string // json|text

	// Proxies
	TrustedProxyCIDRs []string
	AllowedOrigins    []string // Extra allowed WebSocket origins (ALLOWED_ORIGINS env var)

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// Auth
	JWTSecret              string
	JWTExpiration          time.Duration
	RefreshTokenExpiration time.Duration
	APIKeyHeader           string
	AdminUser              string
	AdminPassword          string

	// Rate limiting
	RateLimitRPS        int
	RateLimitBurst      int
	AgentRateLimitRPS   int
	AgentRateLimitBurst int

	// GitHub
	GitHubToken        string
	GitHubPollInterval time.Duration

	// Alerts
	NotifyURL     string
	NtfyAuthToken string
	SMTPHost      string
	SMTPPort      int
	SMTPUser      string
	SMTPPass      string
	SMTPFrom      string
	SMTPTo        string
	SMTPTLS       bool

	// Metrics and Audit retention
	MetricsRetentionDays int
	AuditRetentionDays   int
	WebLogsRetentionDays int
	// NetworkFlowsRetentionDays governs network_flow_metrics ("top talkers"),
	// which stores remote IPs — a potentially identifying value, hence a
	// short applicative-job retention (internal/background/network_flows.go)
	// rather than a fixed TimescaleDB policy, same posture as WebLogsRetentionDays.
	NetworkFlowsRetentionDays int
	// AuditRetentionDaysByCategory overrides AuditRetentionDays per audit
	// log category (models.AuditCategories' keys) — settings-only (no env
	// var: a per-category map doesn't fit the flat KEY=value env shape the
	// rest of this config uses). A category absent from the map falls back
	// to AuditRetentionDays. See internal/background/audit.go.
	AuditRetentionDaysByCategory map[string]int

	// Threat detection (web logs) — admin-tunable coefficients behind the
	// BotView "IPs suspectes" score. See internal/threatdetect.Weights for
	// what each one does; defaults mirror threatdetect.DefaultWeights().
	ThreatWeightWordPress        float64
	ThreatWeightAdminPanel       float64
	ThreatWeightPathTraversal    float64
	ThreatWeightKnownScanner     float64
	ThreatWeightSuspiciousMethod float64
	ThreatWeightStatus2xx        float64
	ThreatWeightStatus3xx        float64
	ThreatWeightStatus404        float64
	ThreatWeightStatus4xxOther   float64
	ThreatWeightStatus5xx        float64
	ThreatWeightBreadth          float64
	ThreatWeightHits             float64
	ThreatThresholdMedium        float64
	ThreatThresholdHigh          float64
	ThreatThresholdCritical      float64
}

// AppEnv reports the current deployment environment ("dev"/"development"
// for local work, anything else — including unset — is treated as production).
func AppEnv() string {
	return strings.ToLower(strings.TrimSpace(getEnv("APP_ENV", "production")))
}

// IsDevEnv returns true when APP_ENV is "dev" or "development".
func IsDevEnv() bool {
	e := AppEnv()
	return e == "dev" || e == "development"
}

// generateRandomSecret returns a 64-character hex string (32 random bytes).
// Used as a dev-only fallback when JWT_SECRET is unset so local runs work
// out of the box without ever using the hardcoded default.
func generateRandomSecret() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Extremely unlikely; fall back to the well-known default so the
		// dev server can still boot. ValidateStrict() will refuse to start
		// in production anyway.
		return DefaultJWTSecret
	}
	return hex.EncodeToString(b[:])
}

// logFormatDefault honours LOG_FORMAT when set, otherwise defaults to
// human-friendly text in dev and machine-parseable json in production.
func logFormatDefault() string {
	if v := strings.TrimSpace(os.Getenv("LOG_FORMAT")); v != "" {
		return v
	}
	if IsDevEnv() {
		return "text"
	}
	return "json"
}

func Load() *Config {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		if IsDevEnv() {
			jwtSecret = generateRandomSecret()
		} else {
			jwtSecret = DefaultJWTSecret
		}
	}

	return &Config{
		Port:       getEnv("SERVER_PORT", "8080"),
		BaseURL:    getEnv("BASE_URL", "http://localhost:8080"),
		TLSEnabled: getBoolEnv("TLS_ENABLED", false),
		DemoMode:   getBoolEnv("DEMO_MODE", false),

		LogLevel:  getEnv("LOG_LEVEL", "info"),
		LogFormat: logFormatDefault(),

		TrustedProxyCIDRs: getCSVEnv("TRUSTED_PROXIES"),
		AllowedOrigins:    getCSVEnv("ALLOWED_ORIGINS"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "supervisor"),
		DBPassword: getEnv("DB_PASSWORD", "supervisor"),
		DBName:     getEnv("DB_NAME", "serversupervisor"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		JWTSecret:              jwtSecret,
		JWTExpiration:          getDurationEnv("JWT_EXPIRATION", 24*time.Hour),
		RefreshTokenExpiration: getDurationEnv("REFRESH_TOKEN_EXPIRATION", 7*24*time.Hour),
		APIKeyHeader:           "X-API-Key",
		AdminUser:              getEnv("ADMIN_USER", "admin"),
		AdminPassword:          getEnv("ADMIN_PASSWORD", "admin"),

		RateLimitRPS:        getIntEnv("RATE_LIMIT_RPS", 100),
		RateLimitBurst:      getIntEnv("RATE_LIMIT_BURST", 200),
		AgentRateLimitRPS:   getIntEnv("AGENT_RATE_LIMIT_RPS", 20),
		AgentRateLimitBurst: getIntEnv("AGENT_RATE_LIMIT_BURST", 40),

		GitHubToken:        getEnv("GITHUB_TOKEN", ""),
		GitHubPollInterval: getDurationEnv("GITHUB_POLL_INTERVAL", 15*time.Minute),

		NotifyURL:     getEnv("NOTIFY_URL", ""),
		NtfyAuthToken: getEnv("NTFY_AUTH_TOKEN", ""),
		SMTPHost:      getEnv("SMTP_HOST", ""),
		SMTPPort:      getIntEnv("SMTP_PORT", 587),
		SMTPUser:      getEnv("SMTP_USER", ""),
		SMTPPass:      getEnv("SMTP_PASS", ""),
		SMTPFrom:      getEnv("SMTP_FROM", ""),
		SMTPTo:        getEnv("SMTP_TO", ""),
		SMTPTLS:       getBoolEnv("SMTP_TLS", true),

		MetricsRetentionDays:      getIntEnv("METRICS_RETENTION_DAYS", 30),
		AuditRetentionDays:        getIntEnv("AUDIT_RETENTION_DAYS", 90),
		WebLogsRetentionDays:      getIntEnv("WEB_LOGS_RETENTION_DAYS", 30),
		NetworkFlowsRetentionDays: getIntEnv("NETWORK_FLOWS_RETENTION_DAYS", 14),

		// Defaults below must match internal/threatdetect.DefaultWeights() —
		// duplicated as literals rather than imported so this leaf config
		// package doesn't need to depend on the models/threatdetect chain.
		ThreatWeightWordPress:        getFloatEnv("THREAT_WEIGHT_WORDPRESS", 2),
		ThreatWeightAdminPanel:       getFloatEnv("THREAT_WEIGHT_ADMIN_PANEL", 3),
		ThreatWeightPathTraversal:    getFloatEnv("THREAT_WEIGHT_PATH_TRAVERSAL", 5),
		ThreatWeightKnownScanner:     getFloatEnv("THREAT_WEIGHT_KNOWN_SCANNER", 4),
		ThreatWeightSuspiciousMethod: getFloatEnv("THREAT_WEIGHT_SUSPICIOUS_METHOD", 2),
		ThreatWeightStatus2xx:        getFloatEnv("THREAT_WEIGHT_STATUS_2XX", 0.1),
		ThreatWeightStatus3xx:        getFloatEnv("THREAT_WEIGHT_STATUS_3XX", 1),
		ThreatWeightStatus404:        getFloatEnv("THREAT_WEIGHT_STATUS_404", 2),
		ThreatWeightStatus4xxOther:   getFloatEnv("THREAT_WEIGHT_STATUS_4XX", 1.5),
		ThreatWeightStatus5xx:        getFloatEnv("THREAT_WEIGHT_STATUS_5XX", 3),
		ThreatWeightBreadth:          getFloatEnv("THREAT_WEIGHT_BREADTH", 3),
		ThreatWeightHits:             getFloatEnv("THREAT_WEIGHT_HITS", 2),
		ThreatThresholdMedium:        getFloatEnv("THREAT_THRESHOLD_MEDIUM", 15),
		ThreatThresholdHigh:          getFloatEnv("THREAT_THRESHOLD_HIGH", 50),
		ThreatThresholdCritical:      getFloatEnv("THREAT_THRESHOLD_CRITICAL", 150),
	}
}

// DBSettingsLoader is a minimal interface to avoid import cycles.
type DBSettingsLoader interface {
	GetAllSettings(ctx context.Context) (map[string]string, error)
}

// OverrideFromDB applies DB-persisted settings on top of env vars.
// Call this after the database is connected.
func (c *Config) OverrideFromDB(db DBSettingsLoader) {
	settings, err := db.GetAllSettings(context.Background())
	if err != nil {
		return
	}
	if v, ok := settings["smtp_host"]; ok && v != "" {
		c.SMTPHost = v
	}
	if v, ok := settings["smtp_port"]; ok && v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.SMTPPort = i
		}
	}
	if v, ok := settings["smtp_user"]; ok && v != "" {
		c.SMTPUser = v
	}
	if v, ok := settings["smtp_pass"]; ok && v != "" {
		c.SMTPPass = v
	}
	if v, ok := settings["smtp_from"]; ok && v != "" {
		c.SMTPFrom = v
	}
	if v, ok := settings["smtp_to"]; ok && v != "" {
		c.SMTPTo = v
	}
	if v, ok := settings["smtp_tls"]; ok {
		c.SMTPTLS = v == "true" || v == "1"
	}
	if v, ok := settings["ntfy_url"]; ok && v != "" {
		c.NotifyURL = v
	}
	if v, ok := settings["github_token"]; ok && v != "" {
		c.GitHubToken = v
	}
	if v, ok := settings["metrics_retention_days"]; ok && v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.MetricsRetentionDays = i
		}
	}
	if v, ok := settings["audit_retention_days"]; ok && v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.AuditRetentionDays = i
		}
	}
	if v, ok := settings["audit_retention_days_by_category"]; ok && v != "" {
		var byCategory map[string]int
		if err := json.Unmarshal([]byte(v), &byCategory); err == nil {
			c.AuditRetentionDaysByCategory = byCategory
		}
	}
	if v, ok := settings["web_logs_retention_days"]; ok && v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.WebLogsRetentionDays = i
		}
	}
	if v, ok := settings["network_flows_retention_days"]; ok && v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.NetworkFlowsRetentionDays = i
		}
	}

	overrideFloat(settings, "threat_weight_wordpress", &c.ThreatWeightWordPress)
	overrideFloat(settings, "threat_weight_adminpanel", &c.ThreatWeightAdminPanel)
	overrideFloat(settings, "threat_weight_pathtraversal", &c.ThreatWeightPathTraversal)
	overrideFloat(settings, "threat_weight_knownscanner", &c.ThreatWeightKnownScanner)
	overrideFloat(settings, "threat_weight_suspiciousmethod", &c.ThreatWeightSuspiciousMethod)
	overrideFloat(settings, "threat_weight_status_2xx", &c.ThreatWeightStatus2xx)
	overrideFloat(settings, "threat_weight_status_3xx", &c.ThreatWeightStatus3xx)
	overrideFloat(settings, "threat_weight_status_404", &c.ThreatWeightStatus404)
	overrideFloat(settings, "threat_weight_status_4xx", &c.ThreatWeightStatus4xxOther)
	overrideFloat(settings, "threat_weight_status_5xx", &c.ThreatWeightStatus5xx)
	overrideFloat(settings, "threat_weight_breadth", &c.ThreatWeightBreadth)
	overrideFloat(settings, "threat_weight_hits", &c.ThreatWeightHits)
	overrideFloat(settings, "threat_threshold_medium", &c.ThreatThresholdMedium)
	overrideFloat(settings, "threat_threshold_high", &c.ThreatThresholdHigh)
	overrideFloat(settings, "threat_threshold_critical", &c.ThreatThresholdCritical)
}

// overrideFloat applies settings[key] to *dst when present and parseable,
// leaving the existing value (env var or hardcoded default) untouched
// otherwise. Used for the threat-detection weights, where the zero value
// (0) is a legitimate admin choice — unlike the int fields above, it can't
// double as a "not set" sentinel.
func overrideFloat(settings map[string]string, key string, dst *float64) {
	if v, ok := settings[key]; ok && v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			*dst = f
		}
	}
}

// Validate returns a list of human-readable warnings for insecure or invalid
// configuration values. The server can still start when warnings are present,
// but they should be addressed before deploying to production.
func (c *Config) Validate() []string {
	var warnings []string
	if c.JWTSecret == DefaultJWTSecret {
		warnings = append(warnings, "JWT_SECRET is using the default insecure value — set a unique secret in production")
	}
	if c.AdminPassword == "admin" {
		warnings = append(warnings, "ADMIN_PASSWORD is 'admin' — change it immediately")
	}
	if c.DBPassword == "supervisor" {
		warnings = append(warnings, "DB_PASSWORD is using the default value — change it in production")
	}
	if c.MetricsRetentionDays <= 0 {
		warnings = append(warnings, "METRICS_RETENTION_DAYS must be a positive integer")
	}
	if c.AuditRetentionDays <= 0 {
		warnings = append(warnings, "AUDIT_RETENTION_DAYS must be a positive integer")
	}
	if c.WebLogsRetentionDays <= 0 {
		warnings = append(warnings, "WEB_LOGS_RETENTION_DAYS must be a positive integer")
	}
	if c.NetworkFlowsRetentionDays <= 0 {
		warnings = append(warnings, "NETWORK_FLOWS_RETENTION_DAYS must be a positive integer")
	}
	return warnings
}

// ValidateStrict returns ErrInsecureConfig (wrapped with details) when any
// insecure default is detected and APP_ENV is not a development environment.
// Server startup should refuse to continue in that case.
func (c *Config) ValidateStrict() error {
	if IsDevEnv() {
		return nil
	}
	var problems []string
	if c.JWTSecret == DefaultJWTSecret || c.JWTSecret == "" {
		problems = append(problems, "JWT_SECRET must be set to a unique random value (>=32 chars)")
	}
	if len(c.JWTSecret) < 32 {
		problems = append(problems, "JWT_SECRET is too short (minimum 32 characters)")
	}
	if c.AdminPassword == "admin" {
		problems = append(problems, "ADMIN_PASSWORD must not be 'admin'")
	}
	if c.DBPassword == "supervisor" {
		problems = append(problems, "DB_PASSWORD must not be the default 'supervisor'")
	}
	if len(problems) == 0 {
		return nil
	}
	return errors.Join(ErrInsecureConfig, errors.New(strings.Join(problems, "; ")))
}

func (c *Config) DBDSN() string {
	return "host=" + c.DBHost +
		" port=" + c.DBPort +
		" user=" + c.DBUser +
		" password=" + c.DBPassword +
		" dbname=" + c.DBName +
		" sslmode=" + c.DBSSLMode
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getIntEnv(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getFloatEnv(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getBoolEnv(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		return v == "true" || v == "1"
	}
	return fallback
}

func getCSVEnv(key string) []string {
	if v := os.Getenv(key); v != "" {
		parts := strings.Split(v, ",")
		var out []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return nil
}
