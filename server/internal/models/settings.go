package models

// SettingsUpdateRequest is the admin body for PUT /settings. Every field is
// optional; only the ones provided (non-zero, or non-nil for SMTPTLS and the
// threat-detection weights) are persisted to the settings table. The threat
// weights use *float64 rather than float64 because 0 is a legitimate value
// for a weight (e.g. "ignore 2xx entirely") and must be distinguishable from
// "not provided" — same reasoning as SMTPTLS being a *bool.
type SettingsUpdateRequest struct {
	SMTPHost             string `json:"smtp_host"`
	SMTPPort             int    `json:"smtp_port"`
	SMTPUser             string `json:"smtp_user"`
	SMTPPass             string `json:"smtp_pass"`
	SMTPFrom             string `json:"smtp_from"`
	SMTPTo               string `json:"smtp_to"`
	SMTPTLS              *bool  `json:"smtp_tls"`
	NtfyURL              string `json:"ntfy_url"`
	GitHubToken          string `json:"github_token"`
	MetricsRetentionDays int    `json:"metrics_retention_days"`
	AuditRetentionDays   int    `json:"audit_retention_days"`
	// AuditRetentionDaysByCategory, when non-nil, replaces the whole map
	// (not a per-key merge) — same semantics as the rest of this struct's
	// "send what you mean the new state to be" fields. A category omitted
	// here falls back to AuditRetentionDays. Keys are models.AuditCategories'
	// Key values; an unknown key or a non-positive value is dropped, not
	// rejected — see Service.Update.
	AuditRetentionDaysByCategory map[string]int `json:"audit_retention_days_by_category,omitempty"`

	ThreatWeightWordPress        *float64 `json:"threat_weight_wordpress"`
	ThreatWeightAdminPanel       *float64 `json:"threat_weight_adminpanel"`
	ThreatWeightPathTraversal    *float64 `json:"threat_weight_pathtraversal"`
	ThreatWeightKnownScanner     *float64 `json:"threat_weight_knownscanner"`
	ThreatWeightSuspiciousMethod *float64 `json:"threat_weight_suspiciousmethod"`
	ThreatWeightStatus2xx        *float64 `json:"threat_weight_status_2xx"`
	ThreatWeightStatus3xx        *float64 `json:"threat_weight_status_3xx"`
	ThreatWeightStatus404        *float64 `json:"threat_weight_status_404"`
	ThreatWeightStatus4xxOther   *float64 `json:"threat_weight_status_4xx"`
	ThreatWeightStatus5xx        *float64 `json:"threat_weight_status_5xx"`
	ThreatWeightBreadth          *float64 `json:"threat_weight_breadth"`
	ThreatWeightHits             *float64 `json:"threat_weight_hits"`
	ThreatThresholdMedium        *float64 `json:"threat_threshold_medium"`
	ThreatThresholdHigh          *float64 `json:"threat_threshold_high"`
	ThreatThresholdCritical      *float64 `json:"threat_threshold_critical"`
}
