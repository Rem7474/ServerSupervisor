package models

import "time"

// ========== Uptime / Synthetic Monitoring ==========

// UptimeProbe configures a periodic HTTP or TCP check executed from the server.
type UptimeProbe struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	Type                string     `json:"type"`              // "http" | "tcp"
	Target              string     `json:"target"`            // URL for http, host:port for tcp
	IntervalSec         int        `json:"interval_sec"`
	TimeoutSec          int        `json:"timeout_sec"`
	ExpectedStatus      int        `json:"expected_status"`   // http only
	ExpectedBodyRegex   string     `json:"expected_body_regex"`
	FollowRedirects     bool       `json:"follow_redirects"`
	VerifyTLS           bool       `json:"verify_tls"`
	Enabled             bool       `json:"enabled"`
	LastStatus          string     `json:"last_status"`       // up | down | unknown
	LastLatencyMs       *int       `json:"last_latency_ms,omitempty"`
	LastStatusCode      *int       `json:"last_status_code,omitempty"`
	LastError           string     `json:"last_error,omitempty"`
	LastCheckedAt       *time.Time `json:"last_checked_at,omitempty"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// UptimeProbeRequest is the create/update body for an uptime probe. The pointer
// fields default to true server-side when omitted (see uptimeProbeFromRequest).
type UptimeProbeRequest struct {
	Name              string `json:"name" binding:"required"`
	Type              string `json:"type" binding:"required,oneof=http tcp"`
	Target            string `json:"target" binding:"required"`
	IntervalSec       int    `json:"interval_sec"`
	TimeoutSec        int    `json:"timeout_sec"`
	ExpectedStatus    int    `json:"expected_status"`
	ExpectedBodyRegex string `json:"expected_body_regex"`
	FollowRedirects   *bool  `json:"follow_redirects"`
	VerifyTLS         *bool  `json:"verify_tls"`
	Enabled           *bool  `json:"enabled"`
}

// UptimeProbeResult is one historical execution sample for an UptimeProbe.
type UptimeProbeResult struct {
	ID         int64     `json:"id"`
	ProbeID    string    `json:"probe_id"`
	CheckedAt  time.Time `json:"checked_at"`
	Success    bool      `json:"success"`
	StatusCode *int      `json:"status_code,omitempty"`
	LatencyMs  int       `json:"latency_ms"`
	Error      string    `json:"error,omitempty"`
}

// UptimeStats aggregates the success/failure split of a probe over a window.
type UptimeStats struct {
	WindowHours      int     `json:"window_hours"`
	TotalChecks      int     `json:"total_checks"`
	SuccessfulChecks int     `json:"successful_checks"`
	UptimePercent    float64 `json:"uptime_percent"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
	P95LatencyMs     int     `json:"p95_latency_ms"`
}

// ========== SSL/TLS Certificate Monitoring ==========

// SSLCertificate represents a monitored TLS endpoint and its last check result.
type SSLCertificate struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Host          string     `json:"host"`
	Port          int        `json:"port"`
	ServerName    string     `json:"server_name,omitempty"` // SNI override; empty = use Host
	Enabled       bool       `json:"enabled"`
	LastCheckedAt *time.Time `json:"last_checked_at,omitempty"`
	ValidFrom     *time.Time `json:"valid_from,omitempty"`
	ValidTo       *time.Time `json:"valid_to,omitempty"`
	Issuer        string     `json:"issuer,omitempty"`
	Subject       string     `json:"subject,omitempty"`
	SerialNumber  string     `json:"serial_number,omitempty"`
	DNSNames      []string   `json:"dns_names,omitempty"`
	DaysRemaining *int       `json:"days_remaining,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// SSLCertificateRequest is the create/update body for a monitored TLS endpoint.
// Port defaults to 443 and Enabled to true when omitted (see sslCertFromRequest).
type SSLCertificateRequest struct {
	Name       string `json:"name" binding:"required"`
	Host       string `json:"host" binding:"required"`
	Port       int    `json:"port"`
	ServerName string `json:"server_name"`
	Enabled    *bool  `json:"enabled"`
}
