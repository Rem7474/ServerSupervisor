package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/serversupervisor/server/internal/auth"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"golang.org/x/crypto/bcrypt"
)

const (
	bruteForceWindow   = 5 * time.Minute
	bruteForceMaxFails = 5
)

// isIPBlocked checks the persistent login_events table (respecting manual unblocks)
// so brute-force blocks survive server restarts.
func (h *AuthHandler) isIPBlocked(ip string) bool {
	since := time.Now().Add(-bruteForceWindow)
	count, err := h.db.CountRecentFailedLoginsAfterUnblock(ip, since)
	if err != nil {
		// Fail open: don't block if DB is temporarily unavailable.
		log.Printf("warn: isIPBlocked DB query failed for %s: %v", ip, err)
		return false
	}
	return count >= bruteForceMaxFails
}

// clientIP returns the real client IP using gin's trusted-proxy-aware method.
// Configure gin.SetTrustedProxies on the engine if the app runs behind a reverse proxy.
func clientIP(c *gin.Context) string {
	return c.ClientIP()
}

type AuthHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewAuthHandler(db *database.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{db: db, cfg: cfg}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		TOTPCode string `json:"totp_code"` // Optional: if MFA is enabled
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ip := clientIP(c)
	userAgent := c.GetHeader("User-Agent")

	if h.isIPBlocked(ip) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many failed login attempts, try again later"})
		return
	}

	user, err := h.db.GetUserByUsername(req.Username)
	if err != nil {
		_ = h.db.CreateLoginEvent(req.Username, ip, userAgent, false)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		_ = h.db.CreateLoginEvent(req.Username, ip, userAgent, false)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Check if user has MFA enabled
	if user.MFAEnabled {
		if user.TOTPSecret == "" {
			// MFA flag is set but no secret configured - shouldn't happen
			c.JSON(http.StatusInternalServerError, gin.H{"error": "MFA configuration error"})
			return
		}

		if req.TOTPCode == "" {
			// MFA required but not provided - return flag to prompt user
			c.JSON(http.StatusOK, gin.H{
				"require_mfa": true,
				"message":     "MFA code required",
			})
			return
		}

		// Verify TOTP code
		if !auth.VerifyTOTPCode(user.TOTPSecret, req.TOTPCode) {
			// Try backup codes
			if !auth.VerifyBackupCode(user.BackupCodes, req.TOTPCode) {
				_ = h.db.CreateLoginEvent(req.Username, ip, userAgent, false)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid TOTP code"})
				return
			}
			// Consume the backup code to prevent reuse
			if err := h.db.ConsumeMFABackupCode(user.Username, req.TOTPCode); err != nil {
				log.Printf("Warning: failed to consume backup code for %s: %v", user.Username, err)
				// Don't fail login, just log the error
			}
		}
	}

	_ = h.db.CreateLoginEvent(req.Username, ip, userAgent, true)

	expiresAt := time.Now().Add(h.cfg.JWTExpiration)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  user.Username,
		"role": user.Role,
		"exp":  expiresAt.Unix(),
		"iat":  time.Now().Unix(),
	})

	tokenString, err := token.SignedString([]byte(h.cfg.JWTSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	refreshToken, err := generateRefreshToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"})
		return
	}
	refreshExpiresAt := time.Now().Add(h.cfg.RefreshTokenExpiration)
	if err := h.db.CreateRefreshToken(user.ID, hashToken(refreshToken), refreshExpiresAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":                tokenString,
		"expires_at":           expiresAt,
		"role":                 user.Role,
		"refresh_token":        refreshToken,
		"refresh_expires_at":   refreshExpiresAt,
		"must_change_password": user.MustChangePassword,
	})
}

// RefreshToken exchanges a refresh token for a new JWT + refresh token
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	newRefreshToken, err := generateRefreshToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"})
		return
	}
	refreshExpiresAt := time.Now().Add(h.cfg.RefreshTokenExpiration)

	userID, err := h.db.RotateRefreshToken(hashToken(req.RefreshToken), hashToken(newRefreshToken), refreshExpiresAt)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}

	user, err := h.db.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}

	// Issue new JWT
	expiresAt := time.Now().Add(h.cfg.JWTExpiration)
	newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  user.Username,
		"role": user.Role,
		"exp":  expiresAt.Unix(),
		"iat":  time.Now().Unix(),
	})
	newTokenString, err := newToken.SignedString([]byte(h.cfg.JWTSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":              newTokenString,
		"expires_at":         expiresAt,
		"refresh_token":      newRefreshToken,
		"refresh_expires_at": refreshExpiresAt,
	})
}

// Logout revokes a refresh token
func (h *AuthHandler) Logout(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	_ = h.db.RevokeRefreshToken(hashToken(req.RefreshToken))
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if len(req.NewPassword) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
		return
	}

	user, err := h.db.GetUserByUsername(username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	hash, err := HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
		return
	}

	if err := h.db.UpdateUserPassword(username, hash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password updated"})
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func generateRefreshToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// ========== MFA Endpoints ==========

// SetupMFA initiates TOTP setup for the current user
func (h *AuthHandler) SetupMFA(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	secret, qrCodeURL, backupCodes, err := auth.GenerateTOTPSecret(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate MFA secret"})
		return
	}

	c.JSON(http.StatusOK, models.TOTPSecretResponse{
		Secret:      secret,
		QRCode:      qrCodeURL,
		BackupCodes: backupCodes,
	})
}

// VerifyMFA verifies and enables TOTP for the current user
func (h *AuthHandler) VerifyMFA(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		Secret      string   `json:"secret" binding:"required"`
		TOTPCode    string   `json:"totp_code" binding:"required"`
		BackupCodes []string `json:"backup_codes" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Verify TOTP code with the secret
	if !auth.VerifyTOTPCode(req.Secret, req.TOTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid TOTP code"})
		return
	}

	// Hash backup codes
	hashedCodes, err := auth.HashBackupCodes(req.BackupCodes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash backup codes"})
		return
	}

	// Get user and update MFA
	user, err := h.db.GetUserByUsername(username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := h.db.SetUserTOTPSecret(user.ID, req.Secret, hashedCodes, true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to enable MFA"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "MFA enabled successfully"})
}

// DisableMFA disables TOTP for the current user
func (h *AuthHandler) DisableMFA(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Verify password
	user, err := h.db.GetUserByUsername(username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid password"})
		return
	}

	if err := h.db.DisableUserMFA(username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to disable MFA"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "MFA disabled successfully"})
}

// GetMFAStatus returns MFA status for the current user
func (h *AuthHandler) GetMFAStatus(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, err := h.db.GetUserByUsername(username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"username":    user.Username,
		"mfa_enabled": user.MFAEnabled,
	})
}

// GetProfile returns the current authenticated user's profile
func (h *AuthHandler) GetProfile(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, err := h.db.GetUserByUsername(username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                   user.ID,
		"username":             user.Username,
		"role":                 user.Role,
		"mfa_enabled":          user.MFAEnabled,
		"must_change_password": user.MustChangePassword,
		"created_at":           user.CreatedAt,
	})
}

// GetSecuritySummary returns login stats, currently blocked IPs, and top failed IPs (admin only).
func (h *AuthHandler) GetSecuritySummary(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	hours := 24
	if h := c.Query("hours"); h != "" {
		if n, err := strconv.Atoi(h); err == nil && n > 0 && n <= 8760 {
			hours = n
		}
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	stats, err := h.db.GetLoginStats(since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch login stats"})
		return
	}

	topFailed, _ := h.db.GetTopFailedIPs(since, 10)
	if topFailed == nil {
		topFailed = []models.IPFailCount{}
	}

	blockedIPs, _ := h.db.GetCurrentlyBlockedIPs(time.Now().Add(-bruteForceWindow), bruteForceMaxFails)
	if blockedIPs == nil {
		blockedIPs = []string{}
	}

	c.JSON(http.StatusOK, gin.H{
		"stats":          stats,
		"blocked_ips":    blockedIPs,
		"top_failed_ips": topFailed,
	})
}

func toInt(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float32:
		return int(t)
	case float64:
		return int(t)
	default:
		return 0
	}
}

func toInt64(v any) int64 {
	switch t := v.(type) {
	case int:
		return int64(t)
	case int32:
		return int64(t)
	case int64:
		return t
	case float32:
		return int64(t)
	case float64:
		return int64(t)
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		if err == nil {
			return n
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err == nil {
			return int64(f)
		}
		return 0
	default:
		return 0
	}
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// UnblockIP persists an IP unblock to DB so it survives server restarts (admin only).
func (h *AuthHandler) UnblockIP(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address required"})
		return
	}

	user := c.GetString("username")
	if err := h.db.UpsertIPUnblock(ip, user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unblock IP"})
		return
	}

	_, _ = h.db.CreateAuditLog(user, "unblock_ip", "", c.ClientIP(), "IP unblocked: "+ip, "success")
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "IP unblocked: " + ip})
}

// GetLoginEvents returns the last 50 login events for the current user
func (h *AuthHandler) GetLoginEvents(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	events, err := h.db.GetLoginEventsByUser(username, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch login events"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"events": events})
}

// RevokeAllSessions revokes all refresh tokens for the current user except the one provided.
func (h *AuthHandler) RevokeAllSessions(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token is required"})
		return
	}
	currentHash := hashToken(req.RefreshToken)
	if err := h.db.RevokeAllOtherSessions(username, currentHash); err != nil {
		log.Printf("Failed to revoke sessions for user %s: %v", username, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke sessions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetAllLoginEventsAdmin returns paginated login events for all users (admin only).
func (h *AuthHandler) GetAllLoginEventsAdmin(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	limit := 50
	offset := (page - 1) * limit

	events, err := h.db.GetAllLoginEvents(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch login events"})
		return
	}
	total, _ := h.db.CountLoginEvents()
	if events == nil {
		events = []models.LoginEvent{}
	}
	c.JSON(http.StatusOK, gin.H{"events": events, "total": total, "page": page, "limit": limit})
}
