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
	// Category is empty on the wire from the agent — internal/threatdetect
	// fills it in server-side (via ClassifyRequests) right after decode,
	// before the report is persisted.
	Category      string     `json:"category,omitempty"`
	Blocked       bool       `json:"blocked,omitempty"`
	BlockedSource string     `json:"blocked_source,omitempty"`
	BlockedReason string     `json:"blocked_reason,omitempty"`
	BlockedAt     *time.Time `json:"blocked_at,omitempty"`
	BlockedUntil  *time.Time `json:"blocked_until,omitempty"`
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

type CrowdSecBlockedEntry struct {
	IP           string `json:"ip"`
	Type         string `json:"type,omitempty"` // "ban", "captcha", "audit", etc.
	Reason       string `json:"reason"`
	Origin       string `json:"origin"`
	Country      string `json:"country,omitempty"`
	ASName       string `json:"as_name,omitempty"`
	BlockedUntil string `json:"blocked_until,omitempty"`
}

// ThreatSummary carries only what the agent itself can observe — the local
// CrowdSec correlation. Suspicious-request classification and counts are
// computed server-side (see internal/threatdetect) from the raw Requests
// below, not reported by the agent.
type ThreatSummary struct {
	CrowdSecTotalBlocked int                    `json:"crowdsec_total_blocked,omitempty"`
	CrowdSecTopBlocked   []CrowdSecBlockedEntry `json:"crowdsec_top_blocked,omitempty"`
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
	Timestamp     time.Time  `json:"timestamp"`
	HostID        string     `json:"host_id"`
	HostName      string     `json:"host_name"`
	Source        string     `json:"source"`
	IP            string     `json:"ip"`
	Method        string     `json:"method"`
	Path          string     `json:"path"`
	Status        int        `json:"status"`
	Bytes         int64      `json:"bytes"`
	UserAgent     string     `json:"user_agent"`
	Domain        string     `json:"domain"`
	Category      string     `json:"category"`
	Blocked       bool       `json:"blocked,omitempty"`
	BlockedSource string     `json:"blocked_source,omitempty"`
	BlockedReason string     `json:"blocked_reason,omitempty"`
	BlockedAt     *time.Time `json:"blocked_at,omitempty"`
	BlockedUntil  *time.Time `json:"blocked_until,omitempty"`
}
