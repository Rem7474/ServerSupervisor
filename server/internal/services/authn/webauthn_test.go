package authn

import (
	"context"
	"encoding/binary"
	"reflect"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/serversupervisor/server/internal/config"
)

// ===== webauthnSessionStore =====

func TestWebauthnSessionStore_PutTakeIsSingleUse(t *testing.T) {
	store := newWebauthnSessionStore()
	token, err := store.put(webauthn.SessionData{Challenge: "abc"})
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if token == "" {
		t.Fatal("expected a non-empty token")
	}

	got, ok := store.take(token)
	if !ok {
		t.Fatal("expected the session to be found on first take")
	}
	if got.Challenge != "abc" {
		t.Errorf("got challenge %q, want %q", got.Challenge, "abc")
	}

	if _, ok := store.take(token); ok {
		t.Error("expected the session to be gone after being taken once (single-use)")
	}
}

func TestWebauthnSessionStore_UnknownTokenRejected(t *testing.T) {
	store := newWebauthnSessionStore()
	if _, ok := store.take("does-not-exist"); ok {
		t.Error("expected an unknown token to be rejected")
	}
}

func TestWebauthnSessionStore_ExpiredSessionRejected(t *testing.T) {
	store := newWebauthnSessionStore()
	token, err := store.put(webauthn.SessionData{Challenge: "abc"})
	if err != nil {
		t.Fatalf("put: %v", err)
	}

	// Force the entry into the past directly (white-box, same package) rather
	// than waiting out the real TTL.
	store.mu.Lock()
	entry := store.data[token]
	entry.expiresAt = time.Now().Add(-time.Second)
	store.data[token] = entry
	store.mu.Unlock()

	if _, ok := store.take(token); ok {
		t.Error("expected an expired session to be rejected")
	}
}

// ===== buildWebAuthnConfig =====

func TestBuildWebAuthnConfig_DefaultsFromBaseURL(t *testing.T) {
	cfg := &config.Config{
		BaseURL:        "https://supervisor.example.com",
		AllowedOrigins: []string{"https://backup.example.com"},
	}
	waCfg, err := buildWebAuthnConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if waCfg.RPID != "supervisor.example.com" {
		t.Errorf("RPID = %q, want %q", waCfg.RPID, "supervisor.example.com")
	}
	want := []string{"https://supervisor.example.com", "https://backup.example.com"}
	if !reflect.DeepEqual(waCfg.RPOrigins, want) {
		t.Errorf("RPOrigins = %v, want %v", waCfg.RPOrigins, want)
	}
}

func TestBuildWebAuthnConfig_EnvOverride(t *testing.T) {
	t.Setenv("WEBAUTHN_RP_ID", "custom.example.com")
	t.Setenv("WEBAUTHN_RP_ORIGINS", "https://custom.example.com, https://other.example.com")

	cfg := &config.Config{BaseURL: "https://supervisor.example.com"}
	waCfg, err := buildWebAuthnConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if waCfg.RPID != "custom.example.com" {
		t.Errorf("RPID = %q, want the WEBAUTHN_RP_ID override", waCfg.RPID)
	}
	want := []string{"https://custom.example.com", "https://other.example.com"}
	if !reflect.DeepEqual(waCfg.RPOrigins, want) {
		t.Errorf("RPOrigins = %v, want %v (env override, trimmed)", waCfg.RPOrigins, want)
	}
}

// ===== newWebauthnUser =====

func TestNewWebauthnUser_IDEncodingRoundTrips(t *testing.T) {
	u := newWebauthnUser(42, "alice", nil)
	if u.WebAuthnName() != "alice" || u.WebAuthnDisplayName() != "alice" {
		t.Errorf("name/display name = %q/%q, want alice/alice", u.WebAuthnName(), u.WebAuthnDisplayName())
	}
	id := u.WebAuthnID()
	if len(id) != 8 {
		t.Fatalf("expected an 8-byte user handle, got %d bytes", len(id))
	}
	if got := binary.BigEndian.Uint64(id); got != 42 {
		t.Errorf("decoded user ID = %d, want 42", got)
	}
	if len(u.WebAuthnCredentials()) != 0 {
		t.Errorf("expected no credentials for a nil input, got %d", len(u.WebAuthnCredentials()))
	}
}

// ===== ceremonies degrade gracefully when WebAuthn isn't configured =====
//
// These construct a *Service directly with wa left nil (white-box, same
// package) rather than going through NewService/buildWebAuthnConfig: this
// library (go-webauthn v0.17) turned out to accept an empty RPID/origins at
// webauthn.New() time and only fail later when a ceremony actually runs, so
// driving "unconfigured" through a real config was flaky — it depended on
// exactly where the library chose to validate, not on requireWebAuthn's own
// nil check, which is the thing these tests are meant to cover.

func TestRequireWebAuthn_NilFails(t *testing.T) {
	svc := &Service{}
	if httpStatus(svc.requireWebAuthn()) != 500 {
		t.Fatalf("expected 500 when wa is nil, got %v", svc.requireWebAuthn())
	}
}

func TestRequireWebAuthn_ConfiguredSucceeds(t *testing.T) {
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "Test", RPID: "example.com", RPOrigins: []string{"https://example.com"},
	})
	if err != nil {
		t.Fatalf("webauthn.New: %v", err)
	}
	svc := &Service{wa: wa}
	if err := svc.requireWebAuthn(); err != nil {
		t.Errorf("expected no error once wa is configured, got %v", err)
	}
}

func TestBeginWebAuthnRegistration_FailsCleanlyWhenUnconfigured(t *testing.T) {
	svc := &Service{repo: &fakeRepo{user: userWithPassword(t, "correct")}, webauthnSessions: newWebauthnSessionStore()}
	_, _, err := svc.BeginWebAuthnRegistration(context.Background(), "alice")
	if httpStatus(err) != 500 {
		t.Fatalf("expected 500 when webauthn is unconfigured, got %v", err)
	}
}

func TestBeginWebAuthnLogin_FailsCleanlyWhenUnconfigured(t *testing.T) {
	svc := &Service{repo: &fakeRepo{user: userWithPassword(t, "correct")}, webauthnSessions: newWebauthnSessionStore()}
	_, _, err := svc.BeginWebAuthnLogin(context.Background(), "alice", "correct", "1.2.3.4", "ua")
	if httpStatus(err) != 500 {
		t.Fatalf("expected 500 when webauthn is unconfigured, got %v", err)
	}
}

func TestHasWebAuthnCredentials_FalseWhenUnconfigured(t *testing.T) {
	svc := &Service{}
	if svc.HasWebAuthnCredentials(context.Background(), 1) {
		t.Error("expected false when webauthn is unconfigured, regardless of stored credentials")
	}
}
