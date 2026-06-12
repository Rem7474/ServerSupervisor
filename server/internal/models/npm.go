package models

import "time"

// ========== Nginx Proxy Manager Integration ==========

// NPMConnection holds credentials and state for one Nginx Proxy Manager instance.
// secret is never serialised to JSON to avoid leaks; it is retrieved only by the
// poller and the test-connection endpoint.
type NPMConnection struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	APIURL          string     `json:"api_url"`
	Identity        string     `json:"identity"`
	HostID          *string    `json:"host_id,omitempty"`
	Enabled         bool       `json:"enabled"`
	PollIntervalSec int        `json:"poll_interval_sec"`
	LastError       string     `json:"last_error,omitempty"`
	LastErrorAt     *time.Time `json:"last_error_at,omitempty"`
	LastSuccessAt   *time.Time `json:"last_success_at,omitempty"`
	ProxyHostCount  int        `json:"proxy_host_count"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// NPMConnectionRequest is the create/update payload. Secret is optional on update
// (empty value = keep existing). HostID is optional (nullable).
type NPMConnectionRequest struct {
	Name            string  `json:"name" binding:"required"`
	APIURL          string  `json:"api_url" binding:"required"`
	Identity        string  `json:"identity" binding:"required"`
	Secret          string  `json:"secret"`
	HostID          *string `json:"host_id"`
	Enabled         *bool   `json:"enabled"`
	PollIntervalSec int     `json:"poll_interval_sec"`
}

// NPMProxyHost is one proxy-host entry imported from NPM.
// uptime_probe_id and ssl_certificate_id are SET NULL when the linked resource is
// deleted, so the next import can recreate them.
// monitoring_enabled is the master switch; the sub-flags only take effect when it is true.
type NPMProxyHost struct {
	ID                      string    `json:"id"`
	ConnectionID            string    `json:"connection_id"`
	NPMID                   int       `json:"npm_id"`
	DomainNames             []string  `json:"domain_names"`
	ForwardHost             string    `json:"forward_host"`
	ForwardPort             int       `json:"forward_port"`
	SSLEnabled              bool      `json:"ssl_enabled"`
	NPMEnabled              bool      `json:"npm_enabled"`
	MonitoringEnabled       bool      `json:"monitoring_enabled"`
	UptimeMonitoringEnabled bool      `json:"uptime_monitoring_enabled"`
	SSLMonitoringEnabled    bool      `json:"ssl_monitoring_enabled"`
	UptimeProbeID           *string   `json:"uptime_probe_id,omitempty"`
	SSLCertificateID        *string   `json:"ssl_certificate_id,omitempty"`
	LastSeenAt              time.Time `json:"last_seen_at"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

// NPMProxyHostEnriched is NPMProxyHost enriched with connection name and live
// status from the linked uptime probe and SSL certificate. Returned by the
// global proxy-host list endpoint.
type NPMProxyHostEnriched struct {
	NPMProxyHost
	ConnectionName      string `json:"connection_name"`
	UptimeStatus        string `json:"uptime_status,omitempty"`
	UptimeLastLatencyMs *int   `json:"uptime_last_latency_ms,omitempty"`
	SSLDaysRemaining    *int   `json:"ssl_days_remaining,omitempty"`
}

// NPMProxyHostUpdateRequest is the PATCH body for toggling monitoring flags.
// All fields are optional; only the provided ones are applied.
type NPMProxyHostUpdateRequest struct {
	MonitoringEnabled       *bool `json:"monitoring_enabled"`
	UptimeMonitoringEnabled *bool `json:"uptime_monitoring_enabled"`
	SSLMonitoringEnabled    *bool `json:"ssl_monitoring_enabled"`
}

// NPMProxyHostPreview combines live NPM proxy-host data with its import status
// in ServerSupervisor. Returned by the preview endpoint; nothing is written to DB.
type NPMProxyHostPreview struct {
	NPMID            int      `json:"npm_id"`
	DomainNames      []string `json:"domain_names"`
	ForwardHost      string   `json:"forward_host"`
	ForwardPort      int      `json:"forward_port"`
	SSLEnabled       bool     `json:"ssl_enabled"`
	NPMEnabled       bool     `json:"npm_enabled"`
	AlreadyImported  bool     `json:"already_imported"`
	UptimeProbeID    *string  `json:"uptime_probe_id,omitempty"`
	SSLCertificateID *string  `json:"ssl_certificate_id,omitempty"`
}
