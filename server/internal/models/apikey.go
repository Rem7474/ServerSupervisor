package models

import "time"

// API key scopes. Whatever the scope, a key never reaches the routes closed to
// API keys (account, users, API keys, settings/config — see
// api.apiKeyForbiddenRoute) nor more than its owner's role allows.
const (
	APIKeyScopeRead  = "read"  // GET/HEAD only
	APIKeyScopeWrite = "write" // every method
)

// APIKey is a personal API key as listed to its owner (or to an admin). The
// secret is never part of it: only the public Prefix identifies the key.
type APIKey struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scope      string     `json:"scope"` // read | write
	Username   string     `json:"username"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// APIKeyCreateRequest is the body of POST /api/v1/auth/api-keys.
// ExpiresInDays = 0 means the key never expires.
type APIKeyCreateRequest struct {
	Name          string `json:"name"`
	Scope         string `json:"scope"`
	ExpiresInDays int    `json:"expires_in_days"`
}

// APIKeyCreated is returned once, at creation: Secret is the full key and
// cannot be retrieved again.
type APIKeyCreated struct {
	Key    APIKey `json:"key"`
	Secret string `json:"secret"`
}
