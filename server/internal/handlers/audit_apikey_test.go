package handlers

import "testing"

func TestAuditAPIKeyID(t *testing.T) {
	if got := auditAPIKeyID(nil); got != "" {
		t.Fatalf("no key: got %q, want empty", got)
	}
	id := int64(12)
	if got := auditAPIKeyID(&id); got != "12" {
		t.Fatalf("got %q, want 12", got)
	}
}
