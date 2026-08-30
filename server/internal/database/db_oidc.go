package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// GetUserByOIDCSub looks up a user by their OIDC subject identifier.
func (db *DB) GetUserByOIDCSub(ctx context.Context, sub string) (*models.User, error) {
	var u models.User
	err := db.conn.QueryRowContext(ctx,
		`SELECT id, username, COALESCE(password_hash, ''), role, auth_provider, oidc_sub, email, totp_secret, backup_codes, mfa_enabled, must_change_password, created_at
		 FROM users WHERE oidc_sub = $1`,
		sub,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.AuthProvider, &u.OIDCSub, &u.Email, &u.TOTPSecret, &u.BackupCodes, &u.MFAEnabled, &u.MustChangePassword, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUserByEmail looks up a user by email address (case-insensitive).
func (db *DB) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	err := db.conn.QueryRowContext(ctx,
		`SELECT id, username, COALESCE(password_hash, ''), role, auth_provider, oidc_sub, email, totp_secret, backup_codes, mfa_enabled, must_change_password, created_at
		 FROM users WHERE LOWER(email) = LOWER($1)`,
		email,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.AuthProvider, &u.OIDCSub, &u.Email, &u.TOTPSecret, &u.BackupCodes, &u.MFAEnabled, &u.MustChangePassword, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// CreateOIDCUser provisions a new user authenticated via OIDC.
func (db *DB) CreateOIDCUser(ctx context.Context, username, email, sub, role string) (*models.User, error) {
	var u models.User
	var emailVal *string
	if email != "" {
		emailVal = &email
	}
	err := db.conn.QueryRowContext(ctx,
		`INSERT INTO users (username, password_hash, role, auth_provider, oidc_sub, email, must_change_password)
		 VALUES ($1, NULL, $2, 'oidc', $3, $4, FALSE)
		 RETURNING id, username, COALESCE(password_hash, ''), role, auth_provider, oidc_sub, email, totp_secret, backup_codes, mfa_enabled, must_change_password, created_at`,
		username, role, sub, emailVal,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.AuthProvider, &u.OIDCSub, &u.Email, &u.TOTPSecret, &u.BackupCodes, &u.MFAEnabled, &u.MustChangePassword, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// LinkOIDCUser links an existing local user account to an OIDC subject identifier and optional email.
func (db *DB) LinkOIDCUser(ctx context.Context, userID int64, sub string, email string) error {
	var emailVal *string
	if email != "" {
		emailVal = &email
	}
	_, err := db.conn.ExecContext(ctx,
		`UPDATE users
		 SET oidc_sub = $1, email = COALESCE(email, $2)
		 WHERE id = $3`,
		sub, emailVal, userID,
	)
	return err
}

// CreateOIDCAuthState records a temporary authorization ceremony state.
func (db *DB) CreateOIDCAuthState(ctx context.Context, stateID, nonce, codeVerifier, redirectURL string, expiresAt time.Time) error {
	_, err := db.conn.ExecContext(ctx,
		`INSERT INTO oidc_auth_states (state_id, nonce, code_verifier, redirect_url, expires_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		stateID, nonce, codeVerifier, redirectURL, expiresAt,
	)
	return err
}

// GetAndConsumeOIDCAuthState retrieves and immediately deletes an OIDC auth state (single-use).
func (db *DB) GetAndConsumeOIDCAuthState(ctx context.Context, stateID string) (*models.OIDCAuthState, error) {
	var state models.OIDCAuthState
	err := db.conn.QueryRowContext(ctx,
		`DELETE FROM oidc_auth_states
		 WHERE state_id = $1 AND expires_at > NOW()
		 RETURNING state_id, nonce, code_verifier, redirect_url, created_at, expires_at`,
		stateID,
	).Scan(&state.StateID, &state.Nonce, &state.CodeVerifier, &state.RedirectURL, &state.CreatedAt, &state.ExpiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &state, nil
}

// CleanExpiredOIDCStates purges expired authorization state records.
func (db *DB) CleanExpiredOIDCStates(ctx context.Context) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM oidc_auth_states WHERE expires_at <= NOW()`)
	return err
}
