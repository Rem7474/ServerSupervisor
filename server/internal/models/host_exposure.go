package models

import "time"

// HostExposedDomain is one NPM domain that routes to a specific host (matched
// by npm_proxy_hosts.forward_host == host.ip_address), enriched with
// aggregated web-log traffic for that domain over the requested window. The
// traffic rows are looked up by domain, not host_id: they are collected by
// whichever agent parses the reverse-proxy's access logs (typically the NPM
// host itself), which is usually a different host than the backend this
// domain forwards to.
type HostExposedDomain struct {
	ProxyHostID        string   `json:"proxy_host_id"`
	ConnectionID       string   `json:"connection_id"`
	ConnectionName     string   `json:"connection_name"`
	DomainNames        []string `json:"domain_names"`
	ForwardPort        int      `json:"forward_port"`
	SSLEnabled         bool     `json:"ssl_enabled"`
	NPMEnabled         bool     `json:"npm_enabled"`
	Requests           int64    `json:"requests"`
	Bytes              int64    `json:"bytes"`
	Errors4xx          int64    `json:"errors_4xx"`
	Errors5xx          int64    `json:"errors_5xx"`
	SuspiciousRequests int64    `json:"suspicious_requests"`
	BlockedRequests    int64    `json:"blocked_requests"`
}

// HostExposure is the compute-on-read correlation result for one host: every
// NPM domain that forwards to it (matched by IP) plus aggregated web-log
// traffic for those domains over the requested window. Nothing here is
// persisted — it is recomputed from npm_proxy_hosts + web_log_requests on
// every request.
type HostExposure struct {
	HostID          string              `json:"host_id"`
	IPAddress       string              `json:"ip_address"`
	Since           time.Time           `json:"since"`
	Domains         []HostExposedDomain `json:"domains"`
	TotalRequests   int64               `json:"total_requests"`
	TotalSuspicious int64               `json:"total_suspicious_requests"`
	TotalBlocked    int64               `json:"total_blocked_requests"`
}
