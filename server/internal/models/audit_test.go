package models

import "testing"

func TestCategorizeAuditAction(t *testing.T) {
	cases := []struct {
		action string
		want   string
	}{
		{"alert_fired", AuditCategoryAlert},
		{"alert_resolved", AuditCategoryAlert},
		{"alert_escalated", AuditCategoryAlert},
		{"unblock_ip", AuditCategoryAuth},
		{"update_settings", AuditCategorySettings},
		{"cleanup_metrics", AuditCategorySettings},
		{"cleanup_audit_logs", AuditCategorySettings},
		{"apt_update", AuditCategoryCommand},
		{"docker_restart", AuditCategoryCommand},
		{"journalctl", AuditCategoryCommand},
		{"webhook_trigger", AuditCategoryCommand},
		{"agent_update", AuditCategoryCommand},
		{"something_never_seen_before", AuditCategoryCommand},
		{"", AuditCategoryCommand},
	}
	for _, c := range cases {
		if got := CategorizeAuditAction(c.action); got != c.want {
			t.Errorf("CategorizeAuditAction(%q) = %q, want %q", c.action, got, c.want)
		}
	}
}

func TestAuditCategories_KeysAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, cat := range AuditCategories() {
		if seen[cat.Key] {
			t.Errorf("duplicate category key %q", cat.Key)
		}
		seen[cat.Key] = true
		if cat.Label == "" {
			t.Errorf("category %q has no label", cat.Key)
		}
	}
}
