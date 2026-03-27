package models

import "time"

type WebRequest struct {
	Timestamp string `json:"timestamp"`
	IP        string `json:"ip"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Status    int    `json:"status"`
	Bytes     int64  `json:"bytes"`
	UserAgent string `json:"user_agent"`
	Domain    string `json:"domain"`
	Category  string `json:"category,omitempty"`
}

type NPMPathHit struct {
	Path string `json:"path"`
	Hits int    `json:"hits"`
}

type NPMDomainStat struct {
	Domain    string         `json:"domain"`
	Hits      int            `json:"hits"`
	Bytes     int64          `json:"bytes"`
	Errors4xx int            `json:"errors_4xx"`
	Errors5xx int            `json:"errors_5xx"`
	Methods   map[string]int `json:"methods"`
	TopPaths  []NPMPathHit   `json:"top_paths"`
}

type TrafficSummary struct {
	TotalRequests int             `json:"total_requests"`
	TotalBytes    int64           `json:"total_bytes"`
	Errors4xx     int             `json:"errors_4xx"`
	Errors5xx     int             `json:"errors_5xx"`
	TopDomains    []NPMDomainStat `json:"top_domains"`
}

type BotDetectionIP struct {
	IP          string       `json:"ip"`
	Hits        int          `json:"hits"`
	UniquePaths int          `json:"unique_paths"`
	FirstSeen   string       `json:"first_seen"`
	LastSeen    string       `json:"last_seen"`
	Category    string       `json:"category"`
	UserAgents  []string     `json:"user_agents"`
	Requests    []WebRequest `json:"requests"`
}

type BotDetectionPath struct {
	Path     string `json:"path"`
	Category string `json:"category"`
	Hits     int    `json:"hits"`
}

type ThreatSummary struct {
	SuspiciousRequests  int                `json:"suspicious_requests"`
	UniqueSuspiciousIPs int                `json:"unique_suspicious_ips"`
	TopSuspiciousIPs    []BotDetectionIP   `json:"top_suspicious_ips"`
	TopSuspiciousPaths  []BotDetectionPath `json:"top_suspicious_paths"`
}

type WebLogReport struct {
	Source           string          `json:"source"`
	Traffic          *TrafficSummary `json:"traffic"`
	Threats          *ThreatSummary  `json:"threats"`
	Requests         []WebRequest    `json:"requests"`
	LogFilesScanned  []string        `json:"log_files_scanned"`
	TailLinesPerFile int             `json:"tail_lines_per_file"`
	TotalRequests    int             `json:"total_requests"`
	CollectedAt      time.Time       `json:"collected_at"`
}

type WebLogIPTimelineRow struct {
	Timestamp time.Time `json:"timestamp"`
	HostID    string    `json:"host_id"`
	HostName  string    `json:"host_name"`
	Source    string    `json:"source"`
	IP        string    `json:"ip"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Status    int       `json:"status"`
	Bytes     int64     `json:"bytes"`
	UserAgent string    `json:"user_agent"`
	Domain    string    `json:"domain"`
	Category  string    `json:"category"`
}
