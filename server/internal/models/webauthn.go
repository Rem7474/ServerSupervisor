package models

import (
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthnCredential is a registered passkey/security key for a user — an
// additional MFA factor alongside TOTP. Credential holds the full go-webauthn
// record (public key, sign counter, flags, attestation) needed to run a login
// ceremony; it's never serialized to the client (json:"-"), only Name/dates are.
type WebAuthnCredential struct {
	ID         string              `json:"id" db:"id"`
	UserID     int64               `json:"-" db:"user_id"`
	Name       string              `json:"name" db:"name"`
	CreatedAt  time.Time           `json:"created_at" db:"created_at"`
	LastUsedAt *time.Time          `json:"last_used_at" db:"last_used_at"`
	Credential webauthn.Credential `json:"-" db:"-"`
}
