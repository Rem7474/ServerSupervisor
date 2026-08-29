package collector

import (
	"context"
	"strings"
	"testing"
	"time"
)

// A cancelled/expired ctx must make CollectAPT return promptly (no per-package
// security/CVE lookups attempted) instead of hanging — this is the guard that
// keeps a fresh host's large pending-package backlog from holding the caller
// (handler_apt.go's detached goroutine) open indefinitely.
func TestCollectAPT_RespectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	var status *AptStatus
	var err error
	go func() {
		status, err = CollectAPT(ctx, true)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("CollectAPT did not return promptly for an already-cancelled context")
	}

	if err != nil {
		if strings.Contains(err.Error(), "apt-get not found in PATH") || strings.Contains(err.Error(), "executable file not found") {
			t.Skip("apt-get not found on this system")
		}
		t.Fatalf("unexpected error: %v", err)
	}
	if status == nil {
		t.Fatal("expected a non-nil status even when the simulate call is cancelled")
	}
}

// CollectAPTFast must return quickly and never populate SecurityUpdates/CVEList
// (those are the slower CollectAPT(true)'s job) — it's called synchronously on
// the command-completion hot path (handler_apt.go), so it must stay cheap.
func TestCollectAPTFast(t *testing.T) {
	done := make(chan struct{})
	var status *AptStatus
	var err error
	go func() {
		status, err = CollectAPTFast(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(35 * time.Second):
		t.Fatal("CollectAPTFast did not return within its bounded budget")
	}

	if err != nil {
		if strings.Contains(err.Error(), "apt-get not found in PATH") || strings.Contains(err.Error(), "executable file not found") {
			t.Skip("apt-get not found on this system")
		}
		t.Fatalf("unexpected error: %v", err)
	}
	if status == nil {
		t.Fatal("expected a non-nil status")
	}
	if status.SecurityUpdates != 0 {
		t.Errorf("expected SecurityUpdates to stay 0 (no per-package lookups), got %d", status.SecurityUpdates)
	}
	if status.CVEList != "[]" {
		t.Errorf("expected CVEList to stay empty, got %q", status.CVEList)
	}
	if status.PackageList == "" {
		t.Error("expected PackageList to always be set (even to \"[]\"), got empty string")
	}
}
