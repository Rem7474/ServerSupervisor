package models

import "time"

// OIDCStatusResponse is returned to public callers (such as LoginView.vue)
// so the UI knows whether to display the SSO button and with what label.
type OIDCStatusResponse struct {
	Enabled         bool   `json:"enabled"`
	DisplayName     string `json:"display_name"`
	AllowLocalLogin bool   `json:"allow_local_login"`
}

// OIDCAuthState represents a pending authorization session before callback.
type OIDCAuthState struct {
	StateID      string    `json:"state_id" db:"state_id"`
	Nonce        string    `json:"nonce" db:"nonce"`
	CodeVerifier string    `json:"code_verifier" db:"code_verifier"`
	RedirectURL  string    `json:"redirect_url" db:"redirect_url"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`
}

// OIDCTokenClaims represents the claims extracted from an ID token.
type OIDCTokenClaims struct {
	Sub               string   `json:"sub"`
	Email             string   `json:"email"`
	EmailVerified     bool     `json:"email_verified"`
	PreferredUsername string   `json:"preferred_username"`
	Name              string   `json:"name"`
	Groups            []string `json:"groups"`
	Roles             []string `json:"roles"`
}
