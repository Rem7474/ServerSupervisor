package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Server
	Port    string
	BaseURL string

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// Auth
	JWTSecret     string
	JWTExpiration time.Duration
	APIKeyHeader  string
	AdminUser     string
	AdminPassword string

	// Rate limiting
	RateLimitRPS   int
	RateLimitBurst int

	// GitHub
	GitHubToken        string
	GitHubPollInterval time.Duration

	// Metrics retention
	MetricsRetentionDays int
}

func Load() *Config {
	return &Config{
		Port:    getEnv("SERVER_PORT", "8080"),
		BaseURL: getEnv("BASE_URL", "http://localhost:8080"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "supervisor"),
		DBPassword: getEnv("DB_PASSWORD", "supervisor"),
		DBName:     getEnv("DB_NAME", "serversupervisor"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		JWTSecret:     getEnv("JWT_SECRET", "change-me-in-production"),
		JWTExpiration: getDurationEnv("JWT_EXPIRATION", 24*time.Hour),
		APIKeyHeader:  "X-API-Key",
		AdminUser:     getEnv("ADMIN_USER", "admin"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "admin"),

		RateLimitRPS:   getIntEnv("RATE_LIMIT_RPS", 100),
		RateLimitBurst: getIntEnv("RATE_LIMIT_BURST", 200),

		GitHubToken:        getEnv("GITHUB_TOKEN", ""),
		GitHubPollInterval: getDurationEnv("GITHUB_POLL_INTERVAL", 15*time.Minute),

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
