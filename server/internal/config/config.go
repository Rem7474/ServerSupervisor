package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const DefaultJWTSecret = "change-me-in-production-please"

type Config struct {
	// Server
	Port       string
	BaseURL    string
	TLSEnabled bool // Whether HTTPS is enabled

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
	RateLimitRPS   int
	RateLimitBurst int

	// GitHub
	GitHubToken        string
	GitHubPollInterval time.Duration

	// Alerts
	NotifyURL string
	SMTPHost  string
	SMTPPort  int
	SMTPUser  string
	SMTPPass  string
	SMTPFrom  string
	SMTPTo    string
	SMTPTLS   bool

	// Metrics retention
	MetricsRetentionDays int
}

func Load() *Config {
	return &Config{
		Port:       getEnv("SERVER_PORT", "8080"),
		BaseURL:    getEnv("BASE_URL", "http://localhost:8080"),
		TLSEnabled: getBoolEnv("TLS_ENABLED", false),

		TrustedProxyCIDRs: getCSVEnv("TRUSTED_PROXIES"),
		AllowedOrigins:    getCSVEnv("ALLOWED_ORIGINS"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "supervisor"),
		DBPassword: getEnv("DB_PASSWORD", "supervisor"),
		DBName:     getEnv("DB_NAME", "serversupervisor"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		JWTSecret:              getEnv("JWT_SECRET", DefaultJWTSecret),
		JWTExpiration:          getDurationEnv("JWT_EXPIRATION", 24*time.Hour),
		RefreshTokenExpiration: getDurationEnv("REFRESH_TOKEN_EXPIRATION", 7*24*time.Hour),
		APIKeyHeader:           "X-API-Key",
		AdminUser:              getEnv("ADMIN_USER", "admin"),
		AdminPassword:          getEnv("ADMIN_PASSWORD", "admin"),

		RateLimitRPS:   getIntEnv("RATE_LIMIT_RPS", 100),
		RateLimitBurst: getIntEnv("RATE_LIMIT_BURST", 200),

		GitHubToken:        getEnv("GITHUB_TOKEN", ""),
		GitHubPollInterval: getDurationEnv("GITHUB_POLL_INTERVAL", 15*time.Minute),

		NotifyURL: getEnv("NOTIFY_URL", ""),
		SMTPHost:  getEnv("SMTP_HOST", ""),
		SMTPPort:  getIntEnv("SMTP_PORT", 587),
		SMTPUser:  getEnv("SMTP_USER", ""),
		SMTPPass:  getEnv("SMTP_PASS", ""),
		SMTPFrom:  getEnv("SMTP_FROM", ""),
		SMTPTo:    getEnv("SMTP_TO", ""),
		SMTPTLS:   getBoolEnv("SMTP_TLS", true),

		MetricsRetentionDays: getIntEnv("METRICS_RETENTION_DAYS", 30),
	}
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
