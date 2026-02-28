package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/serversupervisor/server/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// RefreshTokenRecord holds the data fetched from refresh_tokens.
type RefreshTokenRecord struct {
	UserID    int64
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// ========== Users ==========

func (db *DB) CreateUser(username, passwordHash, role string, mustChangePassword ...bool) error {
	mcp := len(mustChangePassword) > 0 && mustChangePassword[0]
	_, err := db.conn.Exec(
		`INSERT INTO users (username, password_hash, role, must_change_password) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (username) DO NOTHING`,
		username, passwordHash, role, mcp,
	)
	return err
}

func (db *DB) SetUserMustChangePassword(username string, value bool) error {
	_, err := db.conn.Exec(
		`UPDATE users SET must_change_password = $1 WHERE username = $2`,
		value, username,
	)
	return err
}

func (db *DB) GetUserByUsername(username string) (*models.User, error) {
	var u models.User
	err := db.conn.QueryRow(
		`SELECT id, username, password_hash, role, totp_secret, backup_codes, mfa_enabled, must_change_password, created_at
		 FROM users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.TOTPSecret, &u.BackupCodes, &u.MFAEnabled, &u.MustChangePassword, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) GetUserByID(id int64) (*models.User, error) {
	var u models.User
	err := db.conn.QueryRow(
		`SELECT id, username, password_hash, role, totp_secret, backup_codes, mfa_enabled, must_change_password, created_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.TOTPSecret, &u.BackupCodes, &u.MFAEnabled, &u.MustChangePassword, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) GetUsers() ([]models.User, error) {
	rows, err := db.conn.Query(
		`SELECT id, username, role, created_at FROM users ORDER BY username`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt); err != nil {
			continue
		}
		users = append(users, u)
	}
	return users, nil
}

func (db *DB) UpdateUserRole(id int64, role string) error {
	_, err := db.conn.Exec(
		`UPDATE users SET role = $1 WHERE id = $2`,
		role, id,
	)
	return err
}

func (db *DB) UpdateUserPassword(username, passwordHash string) error {
	_, err := db.conn.Exec(
		`UPDATE users SET password_hash = $1, must_change_password = FALSE WHERE username = $2`,
		passwordHash, username,
	)
	return err
}

func (db *DB) DeleteUser(id int64) error {
	_, err := db.conn.Exec(`DELETE FROM users WHERE id = $1`, id)
	return err
}

// ========== Refresh Tokens ==========

func (db *DB) CreateRefreshToken(userID int64, tokenHash string, expiresAt time.Time) error {
	_, err := db.conn.Exec(
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt,
	)
	return err
}

func (db *DB) GetRefreshToken(tokenHash string) (*RefreshTokenRecord, error) {
	var rec RefreshTokenRecord
	var revoked sql.NullTime
	err := db.conn.QueryRow(
		`SELECT user_id, expires_at, revoked_at FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash,
	).Scan(&rec.UserID, &rec.ExpiresAt, &revoked)
	if err != nil {
		return nil, err
	}
	if revoked.Valid {
		rec.RevokedAt = &revoked.Time
	}
	return &rec, nil
}

func (db *DB) RevokeRefreshToken(tokenHash string) error {
	_, err := db.conn.Exec(
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1 AND revoked_at IS NULL`,
		tokenHash,
	)
	return err
}

// ========== User TOTP / MFA ==========

func (db *DB) SetUserTOTPSecret(userID int64, secret, backupCodes string, enabled bool) error {
	_, err := db.conn.Exec(
		`UPDATE users SET totp_secret = $1, backup_codes = $2, mfa_enabled = $3 WHERE id = $4`,
		secret, backupCodes, enabled, userID,
	)
	return err
}

func (db *DB) GetUserTOTPSecret(username string) (string, error) {
	var secret string
	err := db.conn.QueryRow(
		`SELECT COALESCE(totp_secret, '') FROM users WHERE username = $1`,
		username,
	).Scan(&secret)
	return secret, err
}

func (db *DB) GetUserMFAStatus(username string) (bool, error) {
	var enabled bool
	err := db.conn.QueryRow(
		`SELECT mfa_enabled FROM users WHERE username = $1`,
		username,
	).Scan(&enabled)
	return enabled, err
}

func (db *DB) DisableUserMFA(username string) error {
	_, err := db.conn.Exec(
		`UPDATE users SET mfa_enabled = FALSE, totp_secret = '', backup_codes = '[]' WHERE username = $1`,
		username,
	)
	return err
}

func (db *DB) ConsumeMFABackupCode(username, usedCode string) error {
	var rawJSON string
	err := db.conn.QueryRow(
		`SELECT backup_codes FROM users WHERE username = $1`, username,
	).Scan(&rawJSON)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	var codes []string
	if err := json.Unmarshal([]byte(rawJSON), &codes); err != nil {
		return fmt.Errorf("invalid backup codes format: %w", err)
	}

	var remaining []string
	var found bool
	for _, hashed := range codes {
		if bcrypt.CompareHashAndPassword([]byte(hashed), []byte(usedCode)) == nil {
			found = true
			continue
		}
		remaining = append(remaining, hashed)
	}

	if !found {
		return fmt.Errorf("backup code not found or invalid")
	}

	newJSON, _ := json.Marshal(remaining)
	_, err = db.conn.Exec(
		`UPDATE users SET backup_codes = $1 WHERE username = $2`, string(newJSON), username,
	)
	return err
}
