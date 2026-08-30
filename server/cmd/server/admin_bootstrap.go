package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/handlers"
)

// bootstrapAdminAccount implements first-run / account bootstrap: if no admin
// user exists yet, it creates one (generating and printing a temporary
// password when ADMIN_PASSWORD is unset). Otherwise it re-applies the
// must_change_password safety net whenever an existing installation is still
// configured with the literal default password "admin" — a restart alone
// must not silently leave a known-default credential unflagged.
func bootstrapAdminAccount(ctx context.Context, db *database.DB, cfg *config.Config) error {
	hasAdmin, err := db.HasAdminUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to check existing admin user: %w", err)
	}

	if !hasAdmin {
		adminUser := cfg.AdminUser
		if adminUser == "" {
			adminUser = "admin"
		}

		adminPassword := cfg.AdminPassword
		mustChangePassword := false
		isGenerated := false

		if adminPassword == "" {
			adminPassword = config.GenerateRandomPassword(16)
			mustChangePassword = true
			isGenerated = true
		}

		hash, err := handlers.HashPassword(adminPassword)
		if err != nil {
			return fmt.Errorf("failed to hash admin password: %w", err)
		}

		if err := db.CreateUser(ctx, adminUser, hash, "admin", mustChangePassword); err != nil {
			return fmt.Errorf("failed to create initial admin user: %w", err)
		}

		if isGenerated {
			fmt.Printf("\n" +
				"========================================================================\n" +
				"🚀 SERVERSUPERVISOR — COMPTE ADMINISTRATEUR INITIALISÉ\n" +
				"========================================================================\n" +
				"  Identifiant  : %s\n" +
				"  Mot de passe : %s\n\n" +
				"  ⚠️  IMPORTANT : Conservez ce mot de passe temporaire !\n" +
				"  ⚠️  Vous serez invité à le modifier dès votre première connexion.\n" +
				"========================================================================\n\n",
				adminUser, adminPassword)
			slog.InfoContext(ctx, "first run: initial admin account created with generated password", slog.String("username", adminUser))
		} else {
			slog.InfoContext(ctx, "first run: initial admin account created with configured password", slog.String("username", adminUser))
		}
		return nil
	}

	// Existing installation safety net: if configured with default password "admin", enforce must_change_password flag.
	// HasAdminUser only proves *some* admin exists — verify cfg.AdminUser specifically
	// resolves to a real account before touching it. ADMIN_USER changed after first
	// boot (the original admin is still under its old name) must not silently no-op
	// SetUserMustChangePassword against a username nothing matches.
	if cfg.AdminPassword == "admin" {
		if _, err := db.GetUserByUsername(ctx, cfg.AdminUser); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				slog.WarnContext(ctx, "ADMIN_USER does not match any existing user; an admin already exists under a different username, must_change_password safety net not applied", slog.String("configured_admin_user", cfg.AdminUser))
			} else {
				slog.WarnContext(ctx, "failed to look up configured admin user", slog.Any("err", err), slog.String("username", cfg.AdminUser))
			}
		} else if err := db.SetUserMustChangePassword(ctx, cfg.AdminUser, true); err != nil {
			slog.WarnContext(ctx, "failed to enforce must_change_password for admin user", slog.Any("err", err), slog.String("username", cfg.AdminUser))
		} else {
			slog.WarnContext(ctx, "default admin password configured: must_change_password enforced", slog.String("username", cfg.AdminUser))
		}
	}
	slog.InfoContext(ctx, "admin account already initialized", slog.String("username", cfg.AdminUser))
	return nil
}
