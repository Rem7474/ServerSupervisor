package database_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

func TestAPIKeys_Lifecycle(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	if err := db.CreateUser(ctx, "keyowner", "x", models.RoleOperator); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateUser(ctx, "other", "x", models.RoleViewer); err != nil {
		t.Fatal(err)
	}
	owner, _ := db.GetUserByUsername(ctx, "keyowner")
	other, _ := db.GetUserByUsername(ctx, "other")

	hash := strings.Repeat("a", 64)
	future := time.Now().Add(time.Hour)
	k, err := db.CreateAPIKey(ctx, owner.ID, "grafana", "ssk_aaaaaaaa", hash, models.APIKeyScopeRead, &future)
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	if k.Username != "keyowner" || k.Prefix != "ssk_aaaaaaaa" || k.ExpiresAt == nil || k.LastUsedAt != nil || k.RevokedAt != nil {
		t.Fatalf("unexpected created key: %+v", k)
	}
	if _, err := db.CreateAPIKey(ctx, owner.ID, "dup", "ssk_aaaaaaaa", hash, models.APIKeyScopeRead, nil); err == nil {
		t.Fatal("a duplicate prefix must be rejected")
	}
	if _, err := db.CreateAPIKey(ctx, owner.ID, "bad", "ssk_bbbbbbbb", hash, "admin", nil); err == nil {
		t.Fatal("an unknown scope must be rejected by the CHECK constraint")
	}
	past := time.Now().Add(-time.Hour)
	if _, err := db.CreateAPIKey(ctx, owner.ID, "old", "ssk_cccccccc", hash, models.APIKeyScopeWrite, &past); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateAPIKey(ctx, other.ID, "theirs", "ssk_dddddddd", hash, models.APIKeyScopeWrite, nil); err != nil {
		t.Fatal(err)
	}

	mine, err := db.ListAPIKeysByUser(ctx, owner.ID)
	if err != nil || len(mine) != 2 {
		t.Fatalf("ListAPIKeysByUser = %d keys, %v", len(mine), err)
	}
	all, err := db.ListAllAPIKeys(ctx)
	if err != nil || len(all) != 3 {
		t.Fatalf("ListAllAPIKeys = %d keys, %v", len(all), err)
	}
	if n, err := db.CountActiveAPIKeys(ctx, owner.ID); err != nil || n != 1 {
		t.Fatalf("CountActiveAPIKeys = %d, %v (the expired key must not count)", n, err)
	}

	rec, err := db.GetAPIKeyForAuth(ctx, "ssk_aaaaaaaa")
	if err != nil || rec.ID != k.ID || rec.KeyHash != hash || rec.Role != models.RoleOperator || rec.Username != "keyowner" {
		t.Fatalf("GetAPIKeyForAuth = %+v, %v", rec, err)
	}
	if _, err := db.GetAPIKeyForAuth(ctx, "ssk_zzzzzzzz"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("unknown prefix: %v", err)
	}

	// The key follows its owner's role change immediately.
	if err := db.UpdateUserRole(ctx, owner.ID, models.RoleViewer); err != nil {
		t.Fatal(err)
	}
	if rec, _ := db.GetAPIKeyForAuth(ctx, "ssk_aaaaaaaa"); rec.Role != models.RoleViewer {
		t.Fatalf("role = %s, want the owner's new role", rec.Role)
	}

	if err := db.TouchAPIKeyLastUsed(ctx, k.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := db.GetAPIKeyByID(ctx, k.ID)
	if got.LastUsedAt == nil {
		t.Fatal("last_used_at not recorded")
	}
	first := *got.LastUsedAt
	if err := db.TouchAPIKeyLastUsed(ctx, k.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.GetAPIKeyByID(ctx, k.ID); !got.LastUsedAt.Equal(first) {
		t.Fatal("a second use within a minute must not rewrite last_used_at")
	}

	if ok, err := db.RevokeAPIKey(ctx, k.ID); err != nil || !ok {
		t.Fatalf("first revoke = %v, %v", ok, err)
	}
	if ok, err := db.RevokeAPIKey(ctx, k.ID); err != nil || ok {
		t.Fatalf("second revoke = %v, %v (want false)", ok, err)
	}
	if rec, _ := db.GetAPIKeyForAuth(ctx, "ssk_aaaaaaaa"); rec.RevokedAt == nil {
		t.Fatal("revoked_at not visible to authentication")
	}

	if err := db.DeleteUser(ctx, owner.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetAPIKeyByID(ctx, k.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleting the user must delete their keys, got %v", err)
	}
}
