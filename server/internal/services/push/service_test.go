package push

import (
	"context"
	"testing"
)

type fakeRepo struct {
	settings   map[string]string
	setCalls   int
	savedEnd   string
	deletedEnd string
}

func (f *fakeRepo) GetSetting(_ context.Context, key string) (string, error) {
	return f.settings[key], nil
}
func (f *fakeRepo) SetSetting(_ context.Context, key, value string) error {
	if f.settings == nil {
		f.settings = map[string]string{}
	}
	f.settings[key] = value
	f.setCalls++
	return nil
}
func (f *fakeRepo) SavePushSubscription(_ context.Context, _, endpoint, _, _, _ string) error {
	f.savedEnd = endpoint
	return nil
}
func (f *fakeRepo) DeletePushSubscription(_ context.Context, endpoint string) error {
	f.deletedEnd = endpoint
	return nil
}

// TestPublicKey_ReusesStoredKeys verifies a stored key pair is returned as-is
// (no regeneration / no settings writes).
func TestPublicKey_ReusesStoredKeys(t *testing.T) {
	repo := &fakeRepo{settings: map[string]string{
		"vapid_private_key": "priv",
		"vapid_public_key":  "pub",
	}}
	svc := NewService(repo)
	pub, err := svc.PublicKey(context.Background())
	if err != nil {
		t.Fatalf("PublicKey: %v", err)
	}
	if pub != "pub" {
		t.Errorf("expected stored public key, got %q", pub)
	}
	if repo.setCalls != 0 {
		t.Error("must not regenerate/persist when a key pair already exists")
	}
}

// TestPublicKey_GeneratesWhenMissing verifies a key pair is generated + persisted
// on first use.
func TestPublicKey_GeneratesWhenMissing(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	pub, err := svc.PublicKey(context.Background())
	if err != nil {
		t.Fatalf("PublicKey: %v", err)
	}
	if pub == "" {
		t.Error("expected a generated public key")
	}
	if repo.settings["vapid_private_key"] == "" || repo.settings["vapid_public_key"] == "" {
		t.Error("generated key pair must be persisted")
	}
}

func TestSubscribeUnsubscribe(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	if err := svc.Subscribe(context.Background(), "alice", "https://push/endpoint", "p", "a", "ua"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if repo.savedEnd != "https://push/endpoint" {
		t.Errorf("subscription endpoint not saved, got %q", repo.savedEnd)
	}
	if err := svc.Unsubscribe(context.Background(), "https://push/endpoint"); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}
	if repo.deletedEnd != "https://push/endpoint" {
		t.Errorf("subscription not deleted, got %q", repo.deletedEnd)
	}
}
