package database_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/authctx"
	"github.com/serversupervisor/server/internal/database"
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
	k, err := db.CreateAPIKey(ctx, owner.ID, 25, "grafana", "ssk_aaaaaaaa", hash, models.APIKeyScopeRead, &future)
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	if k.Username != "keyowner" || k.Prefix != "ssk_aaaaaaaa" || k.ExpiresAt == nil || k.LastUsedAt != nil || k.RevokedAt != nil {
		t.Fatalf("unexpected created key: %+v", k)
	}
	if _, err := db.CreateAPIKey(ctx, owner.ID, 25, "dup", "ssk_aaaaaaaa", hash, models.APIKeyScopeRead, nil); err == nil {
		t.Fatal("a duplicate prefix must be rejected")
	}
	if _, err := db.CreateAPIKey(ctx, owner.ID, 25, "bad", "ssk_bbbbbbbb", hash, "admin", nil); err == nil {
		t.Fatal("an unknown scope must be rejected by the CHECK constraint")
	}
	past := time.Now().Add(-time.Hour)
	if _, err := db.CreateAPIKey(ctx, owner.ID, 25, "old", "ssk_cccccccc", hash, models.APIKeyScopeWrite, &past); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateAPIKey(ctx, other.ID, 25, "theirs", "ssk_dddddddd", hash, models.APIKeyScopeWrite, nil); err != nil {
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
	// One active key and one expired: a limit of 2 still admits a key (the
	// expired one does not count), a limit of 1 does not.
	if _, err := db.CreateAPIKey(ctx, owner.ID, 1, "over", "ssk_eeeeeeee", hash, models.APIKeyScopeRead, nil); !errors.Is(err, database.ErrAPIKeyLimitReached) {
		t.Fatalf("limit 1 with one active key: got %v, want ErrAPIKeyLimitReached", err)
	}
	if _, err := db.CreateAPIKey(ctx, owner.ID, 2, "second", "ssk_ffffffff", hash, models.APIKeyScopeRead, nil); err != nil {
		t.Fatalf("limit 2 with one active key: %v", err)
	}
	if _, err := db.CreateAPIKey(ctx, 999999, 25, "ghost", "ssk_gggggggg", hash, models.APIKeyScopeRead, nil); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("unknown user: got %v, want sql.ErrNoRows", err)
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

// Concurrent creations for one user never overshoot the limit: the owner's row
// lock serializes them.
func TestAPIKeys_LimitIsAtomic(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	if err := db.CreateUser(ctx, "racer", "x", models.RoleOperator); err != nil {
		t.Fatal(err)
	}
	u, _ := db.GetUserByUsername(ctx, "racer")

	const limit, attempts = 3, 12
	var wg sync.WaitGroup
	var mu sync.Mutex
	created, refused := 0, 0
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			prefix := "ssk_race" + string(rune('a'+i)) + "xxx"
			_, err := db.CreateAPIKey(ctx, u.ID, limit, "k", prefix[:12], strings.Repeat("b", 64), models.APIKeyScopeRead, nil)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				created++
			case errors.Is(err, database.ErrAPIKeyLimitReached):
				refused++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if created != limit || refused != attempts-limit {
		t.Fatalf("created %d, refused %d; want %d and %d", created, refused, limit, attempts-limit)
	}
}

func TestAPIKeys_RevokeAllForUser(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	for _, name := range []string{"carol", "dave"} {
		if err := db.CreateUser(ctx, name, "x", models.RoleOperator); err != nil {
			t.Fatal(err)
		}
	}
	carol, _ := db.GetUserByUsername(ctx, "carol")
	dave, _ := db.GetUserByUsername(ctx, "dave")
	hash := strings.Repeat("c", 64)
	k1, _ := db.CreateAPIKey(ctx, carol.ID, 25, "a", "ssk_carolaaa", hash, models.APIKeyScopeRead, nil)
	k2, _ := db.CreateAPIKey(ctx, carol.ID, 25, "b", "ssk_carolbbb", hash, models.APIKeyScopeWrite, nil)
	kd, _ := db.CreateAPIKey(ctx, dave.ID, 25, "d", "ssk_daveaaaa", hash, models.APIKeyScopeRead, nil)
	if _, err := db.RevokeAPIKey(ctx, k1.ID); err != nil {
		t.Fatal(err)
	}
	before, _ := db.GetAPIKeyByID(ctx, k1.ID)

	if n, err := db.RevokeAllAPIKeysForUser(ctx, "carol"); err != nil || n != 1 {
		t.Fatalf("RevokeAllAPIKeysForUser = %d, %v; want 1 (the already revoked key is kept as is)", n, err)
	}
	if got, _ := db.GetAPIKeyByID(ctx, k2.ID); got.RevokedAt == nil {
		t.Fatal("carol's active key must be revoked")
	}
	if got, _ := db.GetAPIKeyByID(ctx, k1.ID); !got.RevokedAt.Equal(*before.RevokedAt) {
		t.Fatal("an already revoked key keeps its first revocation time")
	}
	if got, _ := db.GetAPIKeyByID(ctx, kd.ID); got.RevokedAt != nil {
		t.Fatal("another user's key must not be revoked")
	}
	if n, err := db.RevokeAllAPIKeysForUser(ctx, "nobody"); err != nil || n != 0 {
		t.Fatalf("unknown user: %d, %v", n, err)
	}
}

// An audit entry written while serving an API-key request records the key.
func TestAuditLog_RecordsAPIKeyFromContext(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	if _, err := db.CreateAuditLog(ctx, "erin", "docker_restart", "", "10.0.0.1", "session", "success"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateAuditLog(authctx.WithAPIKeyID(ctx, 77), "erin", "docker_stop", "", "10.0.0.1", "key", "success"); err != nil {
		t.Fatal(err)
	}
	logs, err := db.GetAuditLogsByUser(ctx, "erin", 10)
	if err != nil || len(logs) != 2 {
		t.Fatalf("GetAuditLogsByUser = %d, %v", len(logs), err)
	}
	byAction := map[string]*int64{}
	for _, l := range logs {
		byAction[l.Action] = l.APIKeyID
	}
	if byAction["docker_restart"] != nil {
		t.Fatalf("a session entry must carry no key, got %d", *byAction["docker_restart"])
	}
	if id := byAction["docker_stop"]; id == nil || *id != 77 {
		t.Fatalf("api_key_id = %v, want 77", id)
	}
}
