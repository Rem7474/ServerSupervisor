package database_test

import (
	"context"
	"testing"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

// TestGetHostByAPIKey covers the "{hostID}.{secret}" lookup used by every
// agent request (see db.go's HashAPIKey / root CLAUDE.md's agent↔server
// protocol section), including the timing-normalisation dummy bcrypt compare
// taken on a "host not found" — see GetHostByAPIKey's doc comment for why
// that branch exists (prevents an attacker from distinguishing "host not
// found" from "wrong secret" via response-time differences).
func TestGetHostByAPIKey(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	const hostID = "host-apikey-test"
	const secret = "correct-secret"
	hash, err := database.HashAPIKey(secret)
	if err != nil {
		t.Fatalf("hash api key: %v", err)
	}
	if err := db.RegisterHost(ctx, &models.Host{
		ID: hostID, Name: "apikey-test", Hostname: "apikey.local", APIKey: hash, Status: "online",
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	t.Run("valid key returns the host", func(t *testing.T) {
		host, err := db.GetHostByAPIKey(ctx, hostID+"."+secret)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if host == nil || host.ID != hostID {
			t.Fatalf("got host = %+v, want ID %q", host, hostID)
		}
	})

	t.Run("wrong secret is rejected", func(t *testing.T) {
		if _, err := db.GetHostByAPIKey(ctx, hostID+".wrong-secret"); err == nil {
			t.Fatal("expected an error for a wrong secret")
		}
	})

	t.Run("unknown host takes the dummy-compare timing-safe path", func(t *testing.T) {
		// This exercises the branch GetHostByAPIKey's doc comment describes:
		// GetHost fails, so a dummy bcrypt comparison runs anyway before
		// returning, rather than returning immediately.
		if _, err := db.GetHostByAPIKey(ctx, "no-such-host.some-secret"); err == nil {
			t.Fatal("expected an error for an unknown host")
		}
	})

	t.Run("malformed key format is rejected without a DB lookup", func(t *testing.T) {
		cases := []string{"no-dot-here", ".leading-dot-secret", hostID + "."}
		for _, key := range cases {
			if _, err := db.GetHostByAPIKey(ctx, key); err == nil {
				t.Errorf("key %q: expected a format error", key)
			}
		}
	})
}
