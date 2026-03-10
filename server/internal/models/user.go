package models

import "time"

// ========== Auth ==========

type User struct {
	ID                 int64     `json:"id" db:"id"`
	Username           string    `json:"username" db:"username"`
	PasswordHash       string    `json:"-" db:"password_hash"`
	Role               string    `json:"role" db:"role"`      // admin, operator, viewer
	TOTPSecret         string    `json:"-" db:"totp_secret"`  // Encrypted TOTP secret (empty if MFA disabled)
	BackupCodes        string    `json:"-" db:"backup_codes"` // JSON array of backup codes (hashed)
	MFAEnabled         bool      `json:"mfa_enabled" db:"mfa_enabled"`
	MustChangePassword bool      `json:"must_change_password" db:"must_change_password"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
}

// LoginEvent records a login attempt (success or failure) for security auditing.
type LoginEvent struct {
	ID        int64     `json:"id" db:"id"`
	Username  string    `json:"username" db:"username"`
	IPAddress string    `json:"ip_address" db:"ip_address"`
	Success   bool      `json:"success" db:"success"`
	UserAgent string    `json:"user_agent" db:"user_agent"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// RefreshToken represents a long-lived token for re-authentication
type RefreshToken struct {
	ID        string    `json:"id" db:"id"`
	UserID    int64     `json:"user_id" db:"user_id"`
	Token     string    `json:"-" db:"token"` // hashed
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// APIKey allows automated access
type APIKey struct {
	ID        string     `json:"id" db:"id"`
	UserID    int64      `json:"user_id" db:"user_id"`
	KeyHash   string     `json:"-" db:"key_hash"` // hashed
	Name      string     `json:"name" db:"name"`
	ExpiresAt *time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}

// ========== Security Monitoring ==========

// LoginStats aggregates login event counts for a time window.
type LoginStats struct {
	Total     int `json:"total"`
	Failures  int `json:"failures"`
	UniqueIPs int `json:"unique_ips"`
}

// IPFailCount holds the number of failed logins for a single IP.
type IPFailCount struct {
	IPAddress string `json:"ip_address"`
	FailCount int    `json:"fail_count"`
}
