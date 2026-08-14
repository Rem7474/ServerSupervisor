package database

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/serversupervisor/server/internal/models"
)

// ========== WebAuthn credentials (passkeys / security keys) ==========

// CreateWebAuthnCredential persists a newly registered credential. cred.ID is
// ignored (DB-generated); cred.Credential is marshaled whole into
// credential_data, matching the go-webauthn storage guidance of round-tripping
// the entire Credential Record rather than splitting it across columns.
func (db *DB) CreateWebAuthnCredential(ctx context.Context, userID int64, name string, cred webauthn.Credential) (*models.WebAuthnCredential, error) {
	data, err := json.Marshal(cred)
	if err != nil {
		return nil, err
	}
	var out models.WebAuthnCredential
	err = db.conn.QueryRowContext(ctx,
		`INSERT INTO webauthn_credentials (user_id, credential_id, name, credential_data)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, name, created_at, last_used_at`,
		userID, cred.ID, name, data,
	).Scan(&out.ID, &out.Name, &out.CreatedAt, &out.LastUsedAt)
	if err != nil {
		return nil, err
	}
	out.UserID = userID
	out.Credential = cred
	return &out, nil
}

// ListWebAuthnCredentials returns every credential registered for userID,
// including the full go-webauthn record — needed to build the login/registration
// ceremony's allow/exclude lists, not just the display list.
func (db *DB) ListWebAuthnCredentials(ctx context.Context, userID int64) ([]models.WebAuthnCredential, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, user_id, name, credential_data, created_at, last_used_at
		 FROM webauthn_credentials WHERE user_id = $1 ORDER BY created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []models.WebAuthnCredential
	for rows.Next() {
		var c models.WebAuthnCredential
		var data []byte
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &data, &c.CreatedAt, &c.LastUsedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &c.Credential); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if out == nil {
		out = []models.WebAuthnCredential{}
	}
	return out, rows.Err()
}

// GetWebAuthnCredentialByCredentialID looks up a credential (and its owning
// user_id) by the raw credential ID alone, globally across all users — needed
// to resolve a discoverable/usernameless login, where the client never tells
// the server which account it's attempting to authenticate as.
func (db *DB) GetWebAuthnCredentialByCredentialID(ctx context.Context, credentialID []byte) (*models.WebAuthnCredential, error) {
	var c models.WebAuthnCredential
	var data []byte
	err := db.conn.QueryRowContext(ctx,
		`SELECT id, user_id, name, credential_data, created_at, last_used_at
		 FROM webauthn_credentials WHERE credential_id = $1`,
		credentialID,
	).Scan(&c.ID, &c.UserID, &c.Name, &data, &c.CreatedAt, &c.LastUsedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &c.Credential); err != nil {
		return nil, err
	}
	return &c, nil
}

// DeleteWebAuthnCredential removes a credential, scoped to its owner so a user
// can only ever delete their own passkeys.
func (db *DB) DeleteWebAuthnCredential(ctx context.Context, id string, userID int64) error {
	_, err := db.conn.ExecContext(ctx,
		`DELETE FROM webauthn_credentials WHERE id = $1 AND user_id = $2`, id, userID)
	return err
}

// CountWebAuthnCredentials reports how many passkeys userID has registered —
// used to decide whether WebAuthn is an available MFA method at login.
func (db *DB) CountWebAuthnCredentials(ctx context.Context, userID int64) (int, error) {
	var n int
	err := db.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM webauthn_credentials WHERE user_id = $1`, userID,
	).Scan(&n)
	return n, err
}

// UpdateWebAuthnCredentialUsage persists the post-login sign counter (clone
// detection) and bumps last_used_at, keyed by the credential's own ID rather
// than the owning user (the ID is already globally unique per the ceremony).
func (db *DB) UpdateWebAuthnCredentialUsage(ctx context.Context, credentialID []byte, cred webauthn.Credential) error {
	data, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	_, err = db.conn.ExecContext(ctx,
		`UPDATE webauthn_credentials SET credential_data = $1, last_used_at = $2 WHERE credential_id = $3`,
		data, time.Now(), credentialID,
	)
	return err
}
