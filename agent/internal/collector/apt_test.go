package collector

import (
	"context"
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
		// apt-get missing entirely is the only expected error path; anything
		// else means the cancelled ctx wasn't respected the way we expect.
		t.Fatalf("unexpected error: %v", err)
	}
	if status == nil {
		t.Fatal("expected a non-nil status even when the simulate call is cancelled")
	}
}
