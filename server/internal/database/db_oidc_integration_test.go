package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

func TestOIDC_DatabaseOperations(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	// 1. Create OIDC User
	u1, err := db.CreateOIDCUser(ctx, "oidc_user_1", "user1@example.com", "sub-12345", models.RoleOperator)
	if err != nil {
		t.Fatalf("CreateOIDCUser failed: %v", err)
	}
	if u1 == nil || u1.Username != "oidc_user_1" || u1.AuthProvider != "oidc" || *u1.OIDCSub != "sub-12345" || *u1.Email != "user1@example.com" {
		t.Fatalf("unexpected user created: %+v", u1)
	}

	// 2. Lookup by sub
	uSub, err := db.GetUserByOIDCSub(ctx, "sub-12345")
	if err != nil {
		t.Fatalf("GetUserByOIDCSub failed: %v", err)
	}
	if uSub.ID != u1.ID {
		t.Fatalf("expected user ID %d, got %d", u1.ID, uSub.ID)
	}

	// 3. Lookup by email
	uEmail, err := db.GetUserByEmail(ctx, "user1@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if uEmail.ID != u1.ID {
		t.Fatalf("expected user ID %d, got %d", u1.ID, uEmail.ID)
	}

	// 4. Missing lookups
	if _, err := db.GetUserByOIDCSub(ctx, "non-existent-sub"); err == nil {
		t.Fatal("expected error for non-existent sub")
	}
	if _, err := db.GetUserByEmail(ctx, "nonexistent@example.com"); err == nil {
		t.Fatal("expected error for non-existent email")
	}

	// 5. Link local user to OIDC
	err = db.CreateUser(ctx, "local_user_to_link", "password_hash_123", models.RoleViewer)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	uLocal, err := db.GetUserByUsername(ctx, "local_user_to_link")
	if err != nil {
		t.Fatalf("GetUserByUsername failed: %v", err)
	}
	err = db.LinkOIDCUser(ctx, uLocal.ID, "sub-linked-999", "linked@example.com")
	if err != nil {
		t.Fatalf("LinkOIDCUser failed: %v", err)
	}
	uLinked, err := db.GetUserByID(ctx, uLocal.ID)
	if err != nil || uLinked.OIDCSub == nil || *uLinked.OIDCSub != "sub-linked-999" {
		t.Fatalf("unexpected linked user: %+v, err=%v", uLinked, err)
	}

	// 6. GetUsers list includes auth_provider and oidc_sub
	users, err := db.GetUsers(ctx)
	if err != nil || len(users) < 2 {
		t.Fatalf("GetUsers failed or unexpected count: %v, len=%d", err, len(users))
	}

	// 7. Auth state lifecycle: Create, Consume, and Clean
	stateID := "test-state-abc"
	nonce := "test-nonce-xyz"
	verifier := "test-verifier-123"
	redir := "/dashboard/hosts"
	expiresAt := time.Now().Add(5 * time.Minute)

	err = db.CreateOIDCAuthState(ctx, stateID, nonce, verifier, redir, expiresAt)
	if err != nil {
		t.Fatalf("CreateOIDCAuthState failed: %v", err)
	}

	// Consume valid state
	consumed, err := db.GetAndConsumeOIDCAuthState(ctx, stateID)
	if err != nil || consumed == nil {
		t.Fatalf("GetAndConsumeOIDCAuthState failed: %v, state=%+v", err, consumed)
	}
	if consumed.StateID != stateID || consumed.Nonce != nonce || consumed.CodeVerifier != verifier || consumed.RedirectURL != redir {
		t.Fatalf("unexpected consumed state: %+v", consumed)
	}

	// Second consume should return nil (single use)
	consumedAgain, err := db.GetAndConsumeOIDCAuthState(ctx, stateID)
	if err != nil || consumedAgain != nil {
		t.Fatalf("expected nil on second consume, got %+v, err=%v", consumedAgain, err)
	}

	// Expired state creation and cleanup
	expiredStateID := "expired-state-id"
	err = db.CreateOIDCAuthState(ctx, expiredStateID, "n", "v", "/", time.Now().Add(-10*time.Minute))
	if err != nil {
		t.Fatalf("CreateOIDCAuthState expired failed: %v", err)
	}
	err = db.CleanExpiredOIDCStates(ctx)
	if err != nil {
		t.Fatalf("CleanExpiredOIDCStates failed: %v", err)
	}
	consumedExpired, err := db.GetAndConsumeOIDCAuthState(ctx, expiredStateID)
	if err != nil || consumedExpired != nil {
		t.Fatalf("expected expired state to be gone, got %+v", consumedExpired)
	}
}
