package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/cookies"
	"github.com/serversupervisor/server/internal/models"
	authnsvc "github.com/serversupervisor/server/internal/services/authn"
)

// AuthHandler translates HTTP to the authn service. It keeps the cookie wiring
// (it needs the gin context + cfg) and request/response shaping; credential,
// session, MFA and security logic live in internal/services/authn.
type AuthHandler struct {
	svc *authnsvc.Service
	cfg *config.Config
}

func NewAuthHandler(svc *authnsvc.Service, cfg *config.Config) *AuthHandler {
	return &AuthHandler{svc: svc, cfg: cfg}
}

// HashPassword bcrypt-hashes a password. Kept exported here as the bootstrap
// (cmd/server) and tests depend on it.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// readRefreshToken reads the refresh token from the cookie (preferred) or the
// JSON body (legacy clients).
func readRefreshToken(c *gin.Context) string {
	if t := cookies.ReadRefreshToken(c.Request); t != "" {
		return t
	}
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = c.ShouldBindJSON(&req)
	return strings.TrimSpace(req.RefreshToken)
}

func (h *AuthHandler) writeSession(c *gin.Context, tokens *authnsvc.SessionTokens) {
	cookies.SetAccess(c, h.cfg, tokens.AccessToken, tokens.AccessExpiresAt, tokens.CSRFToken)
	cookies.SetRefresh(c, h.cfg, tokens.RefreshToken, tokens.RefreshExpiresAt)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		TOTPCode string `json:"totp_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation("invalid request"))
		return
	}

	user, requireMFA, err := h.svc.Authenticate(c.Request.Context(), req.Username, req.Password, req.TOTPCode, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		respondError(c, err)
		return
	}
	if requireMFA {
		c.JSON(http.StatusOK, gin.H{"require_mfa": true, "message": "MFA code required"})
		return
	}

	tokens, err := h.svc.IssueSession(c.Request.Context(), user)
	if err != nil {
		respondError(c, err)
		return
	}
	h.writeSession(c, tokens)
	c.JSON(http.StatusOK, gin.H{
		"username":             user.Username,
		"role":                 user.Role,
		"expires_at":           tokens.AccessExpiresAt,
		"must_change_password": user.MustChangePassword,
		"csrf_token":           tokens.CSRFToken,
	})
}

// RefreshToken exchanges a refresh token for a new JWT + refresh token.
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	refreshStr := readRefreshToken(c)
	if refreshStr == "" {
		respondError(c, apperr.Unauthorized("missing refresh token"))
		return
	}
	user, tokens, err := h.svc.RefreshSession(c.Request.Context(), refreshStr)
	if err != nil {
		if apperr.From(err).HTTPStatus == http.StatusUnauthorized {
			cookies.Clear(c, h.cfg)
		}
		respondError(c, err)
		return
	}
	h.writeSession(c, tokens)
	c.JSON(http.StatusOK, gin.H{
		"username":   user.Username,
		"role":       user.Role,
		"expires_at": tokens.AccessExpiresAt,
		"csrf_token": tokens.CSRFToken,
	})
}

// Logout revokes the refresh token (if any) and clears the auth cookies.
func (h *AuthHandler) Logout(c *gin.Context) {
	h.svc.Logout(c.Request.Context(), readRefreshToken(c))
	cookies.Clear(c, h.cfg)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		respondError(c, apperr.Unauthorized("unauthorized"))
		return
	}
	var req struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation("invalid request"))
		return
	}
	if err := h.svc.ChangePassword(c.Request.Context(), username, req.CurrentPassword, req.NewPassword); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "password updated"})
}

// SetupMFA initiates TOTP setup for the current user.
func (h *AuthHandler) SetupMFA(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		respondError(c, apperr.Unauthorized("unauthorized"))
		return
	}
	secret, qrCodeURL, backupCodes, err := h.svc.SetupMFA(username)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.TOTPSecretResponse{Secret: secret, QRCode: qrCodeURL, BackupCodes: backupCodes})
}

// VerifyMFA verifies and enables TOTP for the current user.
func (h *AuthHandler) VerifyMFA(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		respondError(c, apperr.Unauthorized("unauthorized"))
		return
	}
	var req struct {
		Secret      string   `json:"secret" binding:"required"`
		TOTPCode    string   `json:"totp_code" binding:"required"`
		BackupCodes []string `json:"backup_codes" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation("invalid request"))
		return
	}
	if err := h.svc.VerifyMFA(c.Request.Context(), username, req.Secret, req.TOTPCode, req.BackupCodes); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "MFA enabled successfully"})
}

// DisableMFA disables TOTP for the current user.
func (h *AuthHandler) DisableMFA(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		respondError(c, apperr.Unauthorized("unauthorized"))
		return
	}
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation("invalid request"))
		return
	}
	if err := h.svc.DisableMFA(c.Request.Context(), username, req.Password); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "MFA disabled successfully"})
}

// GetMFAStatus returns MFA status for the current user.
func (h *AuthHandler) GetMFAStatus(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		respondError(c, apperr.Unauthorized("unauthorized"))
		return
	}
	user, err := h.svc.User(c.Request.Context(), username)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"username": user.Username, "mfa_enabled": user.MFAEnabled})
}

// GetProfile returns the current authenticated user's profile.
func (h *AuthHandler) GetProfile(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		respondError(c, apperr.Unauthorized("unauthorized"))
		return
	}
	user, err := h.svc.User(c.Request.Context(), username)
	if err != nil {
		respondError(c, err)
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

// GetSecuritySummary returns login stats, blocked IPs and top failed IPs (admin only).
func (h *AuthHandler) GetSecuritySummary(c *gin.Context) {
	if c.GetString("role") != "admin" {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}
	hours := 24
	if q := c.Query("hours"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 8760 {
			hours = n
		}
	}
	stats, blockedIPs, topFailed, err := h.svc.SecuritySummary(c.Request.Context(), hours)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"stats": stats, "blocked_ips": blockedIPs, "top_failed_ips": topFailed})
}

// UnblockIP persists an IP unblock (admin only).
func (h *AuthHandler) UnblockIP(c *gin.Context) {
	if c.GetString("role") != "admin" {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}
	ip := c.Param("ip")
	if ip == "" {
		respondError(c, apperr.Validation("IP address required"))
		return
	}
	if err := h.svc.UnblockIP(c.Request.Context(), ip, c.GetString("username"), c.ClientIP()); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "IP unblocked: " + ip})
}

// GetLoginEvents returns paginated login events for the current user.
func (h *AuthHandler) GetLoginEvents(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		respondError(c, apperr.Unauthorized("unauthorized"))
		return
	}
	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	page := 1
	if p := c.Query("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	events, err := h.svc.LoginEvents(c.Request.Context(), username, limit, (page-1)*limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": events, "page": page, "limit": limit})
}

// RevokeAllSessions revokes all refresh tokens for the current user except the current one.
func (h *AuthHandler) RevokeAllSessions(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		respondError(c, apperr.Unauthorized("unauthorized"))
		return
	}
	if err := h.svc.RevokeAllSessions(c.Request.Context(), username, readRefreshToken(c)); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetAllLoginEventsAdmin returns paginated login events for all users (admin only).
func (h *AuthHandler) GetAllLoginEventsAdmin(c *gin.Context) {
	if c.GetString("role") != "admin" {
		respondError(c, apperr.Forbidden("forbidden"))
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	limit := 50
	events, total, err := h.svc.AllLoginEvents(c.Request.Context(), limit, (page-1)*limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": events, "total": total, "page": page, "limit": limit})
}
