package main

import (
	"context"
	"strings"
	"testing"

	"github.com/serversupervisor/server/internal/testutil"
)

func TestBootstrapAdminAccount_FirstRun_GeneratesPasswordAndForcesChange(t *testing.T) {
	db, cfg := testutil.NewPostgresDBWithConfig(t)
	cfg.AdminPassword = "" // unset → generated, must_change_password forced
	ctx := context.Background()

	if err := bootstrapAdminAccount(ctx, db, cfg); err != nil {
		t.Fatalf("bootstrapAdminAccount: %v", err)
	}

	u, err := db.GetUserByUsername(ctx, cfg.AdminUser)
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if u.Role != "admin" {
		t.Errorf("role = %q, want admin", u.Role)
	}
	if !u.MustChangePassword {
		t.Error("expected must_change_password = true for a generated first-run password")
	}
}

func TestBootstrapAdminAccount_FirstRun_ConfiguredPasswordDoesNotForceChange(t *testing.T) {
	db, cfg := testutil.NewPostgresDBWithConfig(t)
	cfg.AdminPassword = "a-strong-configured-password"
	ctx := context.Background()

	if err := bootstrapAdminAccount(ctx, db, cfg); err != nil {
		t.Fatalf("bootstrapAdminAccount: %v", err)
	}

	u, err := db.GetUserByUsername(ctx, cfg.AdminUser)
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if u.MustChangePassword {
		t.Error("expected must_change_password = false when ADMIN_PASSWORD was explicitly configured")
	}
}

func TestBootstrapAdminAccount_ExistingInstall_DefaultPasswordReassertsMustChange(t *testing.T) {
	db, cfg := testutil.NewPostgresDBWithConfig(t)
	ctx := context.Background()

	// Simulate a pre-existing installation whose admin account was created
	// long ago (must_change_password already cleared) but is still
	// configured with the literal default password "admin" — the exact
	// regression bootstrapAdminAccount's safety net exists for.
	cfg.AdminPassword = "admin"
	if err := bootstrapAdminAccount(ctx, db, cfg); err != nil {
		t.Fatalf("initial bootstrap: %v", err)
	}
	if err := db.SetUserMustChangePassword(ctx, cfg.AdminUser, false); err != nil {
		t.Fatalf("clear must_change_password: %v", err)
	}

	// A restart with the same still-default config must re-flag the account,
	// not just skip past it because an admin already exists.
	if err := bootstrapAdminAccount(ctx, db, cfg); err != nil {
		t.Fatalf("bootstrapAdminAccount on restart: %v", err)
	}

	u, err := db.GetUserByUsername(ctx, cfg.AdminUser)
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if !u.MustChangePassword {
		t.Error("expected must_change_password to be re-asserted for an existing install still on the default password")
	}
}

func TestBootstrapAdminAccount_ExistingInstall_NonDefaultPasswordLeavesFlagAlone(t *testing.T) {
	db, cfg := testutil.NewPostgresDBWithConfig(t)
	ctx := context.Background()

	// First run with a real configured password (must_change_password=false),
	// then simulate a config change to a *different* non-default password —
	// the safety net must never touch an account whose configured password
	// isn't the literal default.
	cfg.AdminPassword = "first-strong-password"
	if err := bootstrapAdminAccount(ctx, db, cfg); err != nil {
		t.Fatalf("initial bootstrap: %v", err)
	}

	cfg.AdminPassword = "a-different-strong-password"
	if err := bootstrapAdminAccount(ctx, db, cfg); err != nil {
		t.Fatalf("bootstrapAdminAccount on restart: %v", err)
	}

	u, err := db.GetUserByUsername(ctx, cfg.AdminUser)
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if u.MustChangePassword {
		t.Error("must_change_password should stay false when the configured password is not the literal default")
	}
}

func TestBootstrapAdminAccount_PropagatesHasAdminUserError(t *testing.T) {
	db, cfg := testutil.NewPostgresDBWithConfig(t)

	// A canceled context makes the underlying QueryRowContext fail
	// deterministically — this is the standard way to force a DB error
	// without a broken-connection mock, and it exercises the actual
	// concern the wrapping message promises: the caller (main) must see
	// a real error, not a nil/false result, when the admin check itself
	// can't run.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := bootstrapAdminAccount(ctx, db, cfg)
	if err == nil {
		t.Fatal("expected an error when the admin-check query can't run")
	}
	if !strings.Contains(err.Error(), "failed to check existing admin user") {
		t.Errorf("error = %q, want it to identify the admin-check step", err.Error())
	}
}
