// Package oidc is the application/service layer for OpenID Connect (OIDC / SSO)
// authentication. It manages provider discovery, PKCE authorization ceremonies,
// token exchange and validation, claim/role mapping, JIT user provisioning, and
// session issuance behind a Repository port + live *config.Config.
package oidc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	coreosoidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/models"
	authnsvc "github.com/serversupervisor/server/internal/services/authn"
)

const (
	stateTTL = 5 * time.Minute
)

// Repository is the data-access port for OIDC operations.
type Repository interface {
	GetUserByOIDCSub(ctx context.Context, sub string) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
	CreateOIDCUser(ctx context.Context, username, email, sub, role string) (*models.User, error)
	LinkOIDCUser(ctx context.Context, userID int64, sub string, email string) error
	UpdateUserRole(ctx context.Context, id int64, role string) error
	CreateOIDCAuthState(ctx context.Context, stateID, nonce, codeVerifier, redirectURL string, expiresAt time.Time) error
	GetAndConsumeOIDCAuthState(ctx context.Context, stateID string) (*models.OIDCAuthState, error)
	CleanExpiredOIDCStates(ctx context.Context) error
	CreateLoginEvent(ctx context.Context, username, ipAddress, userAgent string, success bool) error
}

// SessionIssuer provides session issuance capabilities.
type SessionIssuer interface {
	IssueSession(ctx context.Context, user *models.User) (*authnsvc.SessionTokens, error)
}

// Service holds the OIDC business logic.
type Service struct {
	repo         Repository
	issuer       SessionIssuer
	cfg          *config.Config
	httpClient   *http.Client
	providerMu   sync.RWMutex
	cachedIssuer string
	provider     *coreosoidc.Provider
	verifier     *coreosoidc.IDTokenVerifier
}

// NewService instantiates a new OIDC service.
func NewService(repo Repository, issuer SessionIssuer, cfg *config.Config) *Service {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.OIDCInsecureSkipVerify, //nolint:gosec // intentional for self-signed test environments
		},
	}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}

	return &Service{
		repo:       repo,
		issuer:     issuer,
		cfg:        cfg,
		httpClient: httpClient,
	}
}

// Status returns the public OIDC configuration status.
func (s *Service) Status() models.OIDCStatusResponse {
	return models.OIDCStatusResponse{
		Enabled:         s.cfg.OIDCEnabled,
		DisplayName:     s.cfg.OIDCDisplayName,
		AllowLocalLogin: s.cfg.OIDCAllowLocalLogin,
	}
}

// getProvider lazily discovers and caches the OIDC provider.
func (s *Service) getProvider(ctx context.Context) (*coreosoidc.Provider, *coreosoidc.IDTokenVerifier, error) {
	if !s.cfg.OIDCEnabled {
		return nil, nil, apperr.Validation("OIDC is not enabled")
	}
	if s.cfg.OIDCIssuerURL == "" {
		return nil, nil, apperr.Validation("OIDC issuer URL is not configured")
	}

	s.providerMu.RLock()
	if s.provider != nil && s.cachedIssuer == s.cfg.OIDCIssuerURL {
		p, v := s.provider, s.verifier
		s.providerMu.RUnlock()
		return p, v, nil
	}
	s.providerMu.RUnlock()

	s.providerMu.Lock()
	defer s.providerMu.Unlock()

	if s.provider != nil && s.cachedIssuer == s.cfg.OIDCIssuerURL {
		return s.provider, s.verifier, nil
	}

	oidcCtx := coreosoidc.ClientContext(ctx, s.httpClient)
	provider, err := coreosoidc.NewProvider(oidcCtx, s.cfg.OIDCIssuerURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to discover OIDC provider at %s: %w", s.cfg.OIDCIssuerURL, err)
	}

	verifier := provider.Verifier(&coreosoidc.Config{
		ClientID: s.cfg.OIDCClientID,
	})

	s.provider = provider
	s.verifier = verifier
	s.cachedIssuer = s.cfg.OIDCIssuerURL
	return provider, verifier, nil
}

// BeginAuth initializes the authorization ceremony with PKCE and returns the redirect URL.
func (s *Service) BeginAuth(ctx context.Context, returnURL string) (string, error) {
	if !s.cfg.OIDCEnabled {
		return "", apperr.Validation("OIDC is not enabled")
	}

	provider, _, err := s.getProvider(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "OIDC provider discovery failed", slog.Any("err", err))
		return "", apperr.Failed("OIDC provider is unreachable")
	}

	stateID, err := generateRandomHex(32)
	if err != nil {
		return "", apperr.Internal(err)
	}
	nonce, err := generateRandomHex(32)
	if err != nil {
		return "", apperr.Internal(err)
	}
	codeVerifier, err := generateRandomHex(32)
	if err != nil {
		return "", apperr.Internal(err)
	}
	codeChallenge := computePKCEChallenge(codeVerifier)

	targetURL := "/"
	if returnURL != "" && strings.HasPrefix(returnURL, "/") && !strings.HasPrefix(returnURL, "//") {
		targetURL = returnURL
	}

	expiresAt := time.Now().Add(stateTTL)
	if err := s.repo.CreateOIDCAuthState(ctx, stateID, nonce, codeVerifier, targetURL, expiresAt); err != nil {
		return "", apperr.Internal(err)
	}

	oauthConfig := oauth2.Config{
		ClientID:     s.cfg.OIDCClientID,
		ClientSecret: s.cfg.OIDCClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  s.cfg.OIDCRedirectURL,
		Scopes:       s.cfg.OIDCScopes,
	}

	authCodeURL := oauthConfig.AuthCodeURL(
		stateID,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)

	return authCodeURL, nil
}

// CompleteAuth exchanges the authorization code, validates the ID token, maps claims,
// provisions/updates the user, and issues application session tokens.
func (s *Service) CompleteAuth(ctx context.Context, stateID, code, clientIP, userAgent string) (*models.User, *authnsvc.SessionTokens, string, error) {
	if !s.cfg.OIDCEnabled {
		return nil, nil, "", apperr.Validation("OIDC is not enabled")
	}
	if stateID == "" || code == "" {
		return nil, nil, "", apperr.Validation("missing state or authorization code")
	}

	authState, err := s.repo.GetAndConsumeOIDCAuthState(ctx, stateID)
	if err != nil {
		return nil, nil, "", apperr.Internal(err)
	}
	if authState == nil {
		return nil, nil, "", apperr.Unauthorized("invalid or expired authentication state")
	}

	provider, verifier, err := s.getProvider(ctx)
	if err != nil {
		return nil, nil, "", apperr.Failed("OIDC provider is unreachable")
	}

	oauthConfig := oauth2.Config{
		ClientID:     s.cfg.OIDCClientID,
		ClientSecret: s.cfg.OIDCClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  s.cfg.OIDCRedirectURL,
		Scopes:       s.cfg.OIDCScopes,
	}

	oidcCtx := coreosoidc.ClientContext(ctx, s.httpClient)
	token, err := oauthConfig.Exchange(
		oidcCtx,
		code,
		oauth2.SetAuthURLParam("code_verifier", authState.CodeVerifier),
	)
	if err != nil {
		slog.WarnContext(ctx, "OIDC code exchange failed", slog.Any("err", err))
		return nil, nil, "", apperr.Unauthorized("failed to exchange authorization code")
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, nil, "", apperr.Unauthorized("no ID token returned by OIDC provider")
	}

	idToken, err := verifier.Verify(oidcCtx, rawIDToken)
	if err != nil {
		slog.WarnContext(ctx, "OIDC token verification failed", slog.Any("err", err))
		return nil, nil, "", apperr.Unauthorized("invalid ID token signature or claims")
	}

	if idToken.Nonce != authState.Nonce {
		return nil, nil, "", apperr.Unauthorized("ID token nonce mismatch")
	}

	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, nil, "", apperr.Internal(err)
	}

	extractedClaims := s.extractClaims(claims, idToken.Subject)
	if extractedClaims.Sub == "" {
		return nil, nil, "", apperr.Unauthorized("missing subject in ID token")
	}

	role, err := s.resolveRole(extractedClaims)
	if err != nil {
		return nil, nil, "", err
	}

	user, err := s.provisionOrSyncUser(ctx, extractedClaims, role)
	if err != nil {
		return nil, nil, "", err
	}

	_ = s.repo.CreateLoginEvent(ctx, user.Username, clientIP, userAgent, true)

	tokens, err := s.issuer.IssueSession(ctx, user)
	if err != nil {
		return nil, nil, "", err
	}

	return user, tokens, authState.RedirectURL, nil
}

// extractClaims extracts standard and configured claim fields from the token claims map.
func (s *Service) extractClaims(claims map[string]interface{}, defaultSub string) models.OIDCTokenClaims {
	res := models.OIDCTokenClaims{
		Sub: defaultSub,
	}

	if s.cfg.OIDCUsernameClaim != "" {
		if v, ok := claims[s.cfg.OIDCUsernameClaim].(string); ok && v != "" {
			res.PreferredUsername = v
		}
	}
	if res.PreferredUsername == "" {
		if v, ok := claims["preferred_username"].(string); ok && v != "" {
			res.PreferredUsername = v
		} else if v, ok := claims["email"].(string); ok && v != "" {
			res.PreferredUsername = v
		} else {
			res.PreferredUsername = defaultSub
		}
	}

	emailClaimKey := "email"
	if s.cfg.OIDCEmailClaim != "" {
		emailClaimKey = s.cfg.OIDCEmailClaim
	}
	if v, ok := claims[emailClaimKey].(string); ok {
		res.Email = v
	}

	if v, ok := claims["email_verified"].(bool); ok {
		res.EmailVerified = v
	}
	if v, ok := claims["name"].(string); ok {
		res.Name = v
	}

	groupsClaimKey := "groups"
	if s.cfg.OIDCGroupsClaim != "" {
		groupsClaimKey = s.cfg.OIDCGroupsClaim
	}
	res.Groups = extractStringSlice(claims, groupsClaimKey)
	res.Roles = extractStringSlice(claims, "roles")

	return res
}

// resolveRole calculates the application role based on group claims and configuration.
func (s *Service) resolveRole(claims models.OIDCTokenClaims) (string, error) {
	allGroups := append([]string{}, claims.Groups...)
	allGroups = append(allGroups, claims.Roles...)

	hasGroup := func(target string) bool {
		if target == "" {
			return false
		}
		for _, g := range allGroups {
			if strings.EqualFold(g, target) {
				return true
			}
		}
		return false
	}

	if hasGroup(s.cfg.OIDCAdminGroup) {
		return models.RoleAdmin, nil
	}
	if hasGroup(s.cfg.OIDCOperatorGroup) {
		return models.RoleOperator, nil
	}
	if hasGroup(s.cfg.OIDCViewerGroup) {
		return models.RoleViewer, nil
	}

	defaultRole := strings.ToLower(strings.TrimSpace(s.cfg.OIDCDefaultRole))
	if defaultRole == models.RoleAdmin || defaultRole == models.RoleOperator || defaultRole == models.RoleViewer {
		return defaultRole, nil
	}

	if defaultRole == "none" || defaultRole == "deny" {
		return "", apperr.Forbidden("user is not a member of any authorized OIDC group")
	}

	return models.RoleViewer, nil
}

// provisionOrSyncUser looks up existing users by sub, email, or username and links/creates them.
func (s *Service) provisionOrSyncUser(ctx context.Context, claims models.OIDCTokenClaims, role string) (*models.User, error) {
	// 1. Check if user exists by OIDC sub
	user, err := s.repo.GetUserByOIDCSub(ctx, claims.Sub)
	if err == nil && user != nil {
		if user.Role != role {
			if err := s.repo.UpdateUserRole(ctx, user.ID, role); err == nil {
				user.Role = role
			}
		}
		return user, nil
	}

	// 2. Check if user exists by email
	if claims.Email != "" {
		user, err = s.repo.GetUserByEmail(ctx, claims.Email)
		if err == nil && user != nil {
			_ = s.repo.LinkOIDCUser(ctx, user.ID, claims.Sub, claims.Email)
			user.OIDCSub = &claims.Sub
			if user.Role != role {
				if err := s.repo.UpdateUserRole(ctx, user.ID, role); err == nil {
					user.Role = role
				}
			}
			return user, nil
		}
	}

	// 3. Check if user exists by username
	if claims.PreferredUsername != "" {
		user, err = s.repo.GetUserByUsername(ctx, claims.PreferredUsername)
		if err == nil && user != nil {
			_ = s.repo.LinkOIDCUser(ctx, user.ID, claims.Sub, claims.Email)
			user.OIDCSub = &claims.Sub
			if user.Role != role {
				if err := s.repo.UpdateUserRole(ctx, user.ID, role); err == nil {
					user.Role = role
				}
			}
			return user, nil
		}
	}

	// 4. If user not found, create new if auto-provisioning is enabled
	if !s.cfg.OIDCAutoCreateUser {
		return nil, apperr.Forbidden("user account not found and auto-provisioning is disabled")
	}

	username := claims.PreferredUsername
	if username == "" {
		username = claims.Email
	}
	if username == "" {
		username = "user_" + claims.Sub[:min(8, len(claims.Sub))]
	}

	// Ensure unique username if colliding
	if existing, _ := s.repo.GetUserByUsername(ctx, username); existing != nil {
		username = fmt.Sprintf("%s_%s", username, generateRandomSuffix(4))
	}

	newUser, err := s.repo.CreateOIDCUser(ctx, username, claims.Email, claims.Sub, role)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	return newUser, nil
}

func extractStringSlice(claims map[string]interface{}, key string) []string {
	var out []string
	val, ok := claims[key]
	if !ok || val == nil {
		return out
	}

	switch v := val.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
	case []string:
		out = append(out, v...)
	case string:
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func generateRandomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func generateRandomSuffix(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func computePKCEChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
