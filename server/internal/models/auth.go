package models

import "time"

// ========== Auth Requests & Responses ==========

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	TOTPCode string `json:"totp_code"` // Optional: TOTP code if user has MFA enabled
}

type LoginResponse struct {
	Token      string    `json:"token"`
	ExpiresAt  time.Time `json:"expires_at"`
	Role       string    `json:"role"`
	RequireMFA bool      `json:"require_mfa"` // True if needs TOTP step
}

type TOTPSecretResponse struct {
	Secret      string   `json:"secret"`       // Base32 encoded secret
	QRCode      string   `json:"qr_code"`      // Data URL for QR code
	BackupCodes []string `json:"backup_codes"` // 10 single-use backup codes
}

// ========== RBAC & Permissions ==========

const (
	RoleAdmin    = "admin"    // Full access
	RoleOperator = "operator" // Can launch APT commands + read all
	RoleViewer   = "viewer"   // Read-only
)
