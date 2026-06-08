package models

// SettingsUpdateRequest is the admin body for PUT /settings. Every field is
// optional; only the ones provided (non-zero, or non-nil for SMTPTLS) are
// persisted to the settings table.
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
}
