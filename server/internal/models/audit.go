package models

import "strings"

// Audit log categories (ROADMAP.md item #13: per-category retention +
// export). Low-maintenance by design — action is free text with no strict
// enum anywhere else (any CreateAuditLog caller can pass a new string), so
// CategorizeAuditAction buckets by a handful of known prefixes/exact matches
// and defaults everything else to "command" rather than requiring every new
// action string to be taught to a growing switch statement.
const (
	AuditCategoryAlert    = "alert"
	AuditCategorySettings = "settings"
	AuditCategoryAuth     = "auth"
	AuditCategoryCommand  = "command"
)

// AuditCategory describes one bucket for the retention-settings UI and the
// audit log browser's category filter.
type AuditCategory struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// AuditCategories returns every known category in a stable display order.
func AuditCategories() []AuditCategory {
	return []AuditCategory{
		{Key: AuditCategoryAlert, Label: "Alertes"},
		{Key: AuditCategoryAuth, Label: "Authentification"},
		{Key: AuditCategorySettings, Label: "Réglages"},
		{Key: AuditCategoryCommand, Label: "Commandes"},
	}
}

// CategorizeAuditAction buckets an audit_logs.action value into one of the
// categories above, computed once at write time (CreateAuditLog) and stored
// in audit_logs.category — not re-derived on read, so a future rename of
// this function's rules doesn't retroactively reclassify old rows (migration
// 089 does a one-time backfill using the same rules at the time it ran).
func CategorizeAuditAction(action string) string {
	switch {
	case strings.HasPrefix(action, "alert_"):
		return AuditCategoryAlert
	case action == "unblock_ip":
		return AuditCategoryAuth
	case action == "update_settings" || action == "cleanup_metrics" || action == "cleanup_audit_logs":
		return AuditCategorySettings
	default:
		return AuditCategoryCommand
	}
}
