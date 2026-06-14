// Package authn is the application/service layer for authentication: credential
// verification, brute-force/IP blocking, MFA (TOTP + backup codes), password
// change, session issuance (JWT + refresh token + CSRF) and the login-event /
// security queries — all behind a Repository port + the live *config.Config.
//
// HTTP concerns stay in the handler: reading/writing the auth cookies, request
// binding and response shaping. The service returns SessionTokens; the handler
// turns them into cookies.
package authn

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/auth"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/cookies"
	"github.com/serversupervisor/server/internal/models"
)

const (
	bruteForceWindow   = 5 * time.Minute
	bruteForceMaxFails = 5
)

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
	GetUserByID(ctx context.Context, id int64) (*models.User, error)
	CreateLoginEvent(ctx context.Context, username, ipAddress, userAgent string, success bool) error
	CountRecentFailedLoginsAfterUnblock(ctx context.Context, ipAddress string, since time.Time) (int, error)
	ConsumeMFABackupCode(ctx context.Context, username, usedCode string) error
	CreateRefreshToken(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error
	RotateRefreshToken(ctx context.Context, currentTokenHash, newTokenHash string, expiresAt time.Time) (int64, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeAllOtherSessions(ctx context.Context, username, currentTokenHash string) error
	UpdateUserPassword(ctx context.Context, username, passwordHash string) error
	SetUserTOTPSecret(ctx context.Context, userID int64, secret, backupCodes string, enabled bool) error
	DisableUserMFA(ctx context.Context, username string) error
	GetLoginStats(ctx context.Context, since time.Time) (*models.LoginStats, error)
	GetTopFailedIPs(ctx context.Context, since time.Time, limit int) ([]models.IPFailCount, error)
	GetCurrentlyBlockedIPs(ctx context.Context, since time.Time, threshold int) ([]string, error)
	UpsertIPUnblock(ctx context.Context, ipAddress, unblockedBy string) error
	CreateAuditLog(ctx context.Context, username, action, hostID, ipAddress, details, status string) (int64, error)
	GetLoginEventsByUser(ctx context.Context, username string, limit, offset int) ([]models.LoginEvent, error)
	GetAllLoginEvents(ctx context.Context, limit, offset int) ([]models.LoginEvent, error)
	CountLoginEvents(ctx context.Context) (int64, error)
}

// Service holds the authentication use-cases.
type Service struct {
	repo        Repository
	cfg         *config.Config
	memFailures map[string][]time.Time
	memMu       sync.Mutex
}

func NewService(repo Repository, cfg *config.Config) *Service {
	return &Service{repo: repo, cfg: cfg, memFailures: make(map[string][]time.Time)}
}

// SessionTokens carries the freshly minted credentials for a session; the handler
// writes them as cookies and echoes the CSRF token + expiry.
type SessionTokens struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	CSRFToken        string
}

// ===== brute-force / IP blocking =====

func (s *Service) ipBlocked(ctx context.Context, ip string) bool {
	since := time.Now().Add(-bruteForceWindow)
	count, err := s.repo.CountRecentFailedLoginsAfterUnblock(ctx, ip, since)
	if err != nil {
		slog.ErrorContext(ctx, "isIPBlocked DB query failed — using in-memory fallback", slog.String("ip", ip), slog.Any("err", err))
		return s.memIsBlocked(ip)
	}
	return count >= bruteForceMaxFails
}

func (s *Service) memRecordFailure(ip string) {
	s.memMu.Lock()
	defer s.memMu.Unlock()
	now := time.Now()
	cutoff := now.Add(-bruteForceWindow)
	prev := s.memFailures[ip]
	filtered := prev[:0]
	for _, t := range prev {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	s.memFailures[ip] = append(filtered, now)
}

func (s *Service) memIsBlocked(ip string) bool {
	s.memMu.Lock()
	defer s.memMu.Unlock()
	cutoff := time.Now().Add(-bruteForceWindow)
	count := 0
	for _, t := range s.memFailures[ip] {
		if t.After(cutoff) {
			count++
		}
	}
	return count >= bruteForceMaxFails
}

func (s *Service) recordFailure(ctx context.Context, username, ip, userAgent string) {
	_ = s.repo.CreateLoginEvent(ctx, username, ip, userAgent, false)
	s.memRecordFailure(ip)
}

// ===== login / sessions =====

// Authenticate verifies credentials + MFA. It returns (user, false, nil) on
// success, (nil, true, nil) when MFA is enabled but no code was supplied (the
// caller should prompt), or a typed error (429 blocked / 401 invalid / 500 misconfig).
func (s *Service) Authenticate(ctx context.Context, username, password, totpCode, ip, userAgent string) (*models.User, bool, error) {
	if s.ipBlocked(ctx, ip) {
		return nil, false, apperr.TooManyRequests("Too many failed login attempts, try again later")
	}
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		s.recordFailure(ctx, username, ip, userAgent)
		return nil, false, apperr.Unauthorized("invalid credentials")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		s.recordFailure(ctx, username, ip, userAgent)
		return nil, false, apperr.Unauthorized("invalid credentials")
	}

	if user.MFAEnabled {
		if user.TOTPSecret == "" {
			return nil, false, apperr.Failed("MFA configuration error")
		}
		if totpCode == "" {
			return nil, true, nil
		}
		if !auth.VerifyTOTPCode(user.TOTPSecret, totpCode) {
			if !auth.VerifyBackupCode(user.BackupCodes, totpCode) {
				s.recordFailure(ctx, username, ip, userAgent)
				return nil, false, apperr.Unauthorized("invalid TOTP code")
			}
			if err := s.repo.ConsumeMFABackupCode(ctx, user.Username, totpCode); err != nil {
				slog.ErrorContext(ctx, "failed to consume backup code", slog.String("user", user.Username), slog.Any("err", err))
			}
		}
	}

	_ = s.repo.CreateLoginEvent(ctx, username, ip, userAgent, true)
	return user, false, nil
}

// IssueSession mints + stores a new session (JWT + refresh token + CSRF) for a user.
func (s *Service) IssueSession(ctx context.Context, user *models.User) (*SessionTokens, error) {
	access, accessExp, csrf, err := s.signAccess(user)
	if err != nil {
		return nil, err
	}
	refresh, err := generateRefreshToken()
	if err != nil {
		return nil, apperr.Internal(err)
	}
	refreshExp := time.Now().Add(s.cfg.RefreshTokenExpiration)
	if err := s.repo.CreateRefreshToken(ctx, user.ID, hashToken(refresh), refreshExp); err != nil {
		return nil, apperr.Internal(err)
	}
	return &SessionTokens{AccessToken: access, AccessExpiresAt: accessExp, RefreshToken: refresh, RefreshExpiresAt: refreshExp, CSRFToken: csrf}, nil
}

// RefreshSession rotates the refresh token and issues a fresh session. An invalid
// refresh token yields apperr.Unauthorized (the handler should clear cookies).
func (s *Service) RefreshSession(ctx context.Context, refreshStr string) (*models.User, *SessionTokens, error) {
	newRefresh, err := generateRefreshToken()
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}
	refreshExp := time.Now().Add(s.cfg.RefreshTokenExpiration)
	userID, err := s.repo.RotateRefreshToken(ctx, hashToken(refreshStr), hashToken(newRefresh), refreshExp)
	if err != nil {
		return nil, nil, apperr.Unauthorized("invalid refresh token")
	}
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, nil, apperr.Unauthorized("invalid refresh token")
	}
	access, accessExp, csrf, err := s.signAccess(user)
	if err != nil {
		return nil, nil, err
	}
	return user, &SessionTokens{AccessToken: access, AccessExpiresAt: accessExp, RefreshToken: newRefresh, RefreshExpiresAt: refreshExp, CSRFToken: csrf}, nil
}

// Logout best-effort revokes the supplied refresh token.
func (s *Service) Logout(ctx context.Context, refreshStr string) {
	if refreshStr != "" {
		_ = s.repo.RevokeRefreshToken(ctx, hashToken(refreshStr))
	}
}

func (s *Service) signAccess(user *models.User) (token string, expiresAt time.Time, csrf string, err error) {
	expiresAt = time.Now().Add(s.cfg.JWTExpiration)
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  user.Username,
		"role": user.Role,
		"exp":  expiresAt.Unix(),
		"iat":  time.Now().Unix(),
	})
	token, err = t.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", time.Time{}, "", apperr.Internal(err)
	}
	csrf, err = cookies.GenerateCSRFToken()
	if err != nil {
		return "", time.Time{}, "", apperr.Internal(err)
	}
	return token, expiresAt, csrf, nil
}

// ===== password / MFA =====

// ChangePassword verifies the current password and stores a new one.
func (s *Service) ChangePassword(ctx context.Context, username, current, next string) error {
	if len(next) < 8 {
		return apperr.Validation("password must be at least 8 characters")
	}
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return apperr.Unauthorized("invalid credentials")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(current)) != nil {
		return apperr.Unauthorized("invalid credentials")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return apperr.Internal(err)
	}
	if err := s.repo.UpdateUserPassword(ctx, username, string(hash)); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

// SetupMFA generates a new TOTP secret + backup codes (not yet enabled).
func (s *Service) SetupMFA(username string) (secret, qrCodeURL string, backupCodes []string, err error) {
	secret, qrCodeURL, backupCodes, err = auth.GenerateTOTPSecret(username)
	if err != nil {
		return "", "", nil, apperr.Internal(err)
	}
	return secret, qrCodeURL, backupCodes, nil
}

// VerifyMFA validates the TOTP code against the supplied secret and enables MFA.
func (s *Service) VerifyMFA(ctx context.Context, username, secret, totpCode string, backupCodes []string) error {
	if !auth.VerifyTOTPCode(secret, totpCode) {
		return apperr.Validation("invalid TOTP code")
	}
	hashedCodes, err := auth.HashBackupCodes(backupCodes)
	if err != nil {
		return apperr.Internal(err)
	}
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return apperr.Unauthorized("unauthorized")
	}
	if err := s.repo.SetUserTOTPSecret(ctx, user.ID, secret, hashedCodes, true); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

// DisableMFA turns off MFA after verifying the user's password.
func (s *Service) DisableMFA(ctx context.Context, username, password string) error {
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return apperr.Unauthorized("unauthorized")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return apperr.Unauthorized("invalid password")
	}
	if err := s.repo.DisableUserMFA(ctx, username); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

// User returns a user by username (for profile / MFA status), 401 when absent.
func (s *Service) User(ctx context.Context, username string) (*models.User, error) {
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, apperr.Unauthorized("unauthorized")
	}
	return user, nil
}

// ===== security / login events =====

// SecuritySummary returns login stats, currently-blocked IPs and top failed IPs.
func (s *Service) SecuritySummary(ctx context.Context, hours int) (*models.LoginStats, []string, []models.IPFailCount, error) {
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	stats, err := s.repo.GetLoginStats(ctx, since)
	if err != nil {
		return nil, nil, nil, apperr.Internal(err)
	}
	topFailed, err := s.repo.GetTopFailedIPs(ctx, since, 10)
	if err != nil {
		slog.ErrorContext(ctx, "GetTopFailedIPs", slog.Any("err", err))
	}
	if topFailed == nil {
		topFailed = []models.IPFailCount{}
	}
	blockedIPs, err := s.repo.GetCurrentlyBlockedIPs(ctx, time.Now().Add(-bruteForceWindow), bruteForceMaxFails)
	if err != nil {
		slog.ErrorContext(ctx, "GetCurrentlyBlockedIPs", slog.Any("err", err))
	}
	if blockedIPs == nil {
		blockedIPs = []string{}
	}
	return stats, blockedIPs, topFailed, nil
}

// UnblockIP persists an IP unblock and audits it.
func (s *Service) UnblockIP(ctx context.Context, ip, actor, clientIP string) error {
	if err := s.repo.UpsertIPUnblock(ctx, ip, actor); err != nil {
		return apperr.Internal(err)
	}
	_, _ = s.repo.CreateAuditLog(ctx, actor, "unblock_ip", "", clientIP, "IP unblocked: "+ip, "success")
	return nil
}

// LoginEvents returns a page of the user's own login events.
func (s *Service) LoginEvents(ctx context.Context, username string, limit, offset int) ([]models.LoginEvent, error) {
	events, err := s.repo.GetLoginEventsByUser(ctx, username, limit, offset)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return events, nil
}

// RevokeAllSessions revokes every refresh token for the user except the current one.
func (s *Service) RevokeAllSessions(ctx context.Context, username, currentRefresh string) error {
	if currentRefresh == "" {
		return apperr.Validation("missing refresh token")
	}
	if err := s.repo.RevokeAllOtherSessions(ctx, username, hashToken(currentRefresh)); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

// AllLoginEvents returns a page of login events across all users (admin).
func (s *Service) AllLoginEvents(ctx context.Context, limit, offset int) ([]models.LoginEvent, int64, error) {
	events, err := s.repo.GetAllLoginEvents(ctx, limit, offset)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	total, _ := s.repo.CountLoginEvents(ctx)
	if events == nil {
		events = []models.LoginEvent{}
	}
	return events, total, nil
}

// ===== token helpers =====

func generateRefreshToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
