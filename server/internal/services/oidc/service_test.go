package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/models"
	authnsvc "github.com/serversupervisor/server/internal/services/authn"
)

type mockRepo struct {
	usersBySub      map[string]*models.User
	usersByEmail    map[string]*models.User
	usersByUsername map[string]*models.User
	authStates      map[string]*models.OIDCAuthState
	loginEvents     []string
	errOnState      bool
	errOnUser       bool
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		usersBySub:      make(map[string]*models.User),
		usersByEmail:    make(map[string]*models.User),
		usersByUsername: make(map[string]*models.User),
		authStates:      make(map[string]*models.OIDCAuthState),
	}
}

func (m *mockRepo) GetUserByOIDCSub(ctx context.Context, sub string) (*models.User, error) {
	if m.errOnUser {
		return nil, errors.New("db error")
	}
	if u, ok := m.usersBySub[sub]; ok {
		return u, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockRepo) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.errOnUser {
		return nil, errors.New("db error")
	}
	if u, ok := m.usersByEmail[email]; ok {
		return u, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockRepo) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	if m.errOnUser {
		return nil, errors.New("db error")
	}
	if u, ok := m.usersByUsername[username]; ok {
		return u, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockRepo) CreateOIDCUser(ctx context.Context, username, email, sub, role string) (*models.User, error) {
	if m.errOnUser {
		return nil, errors.New("db error on create")
	}
	subCopy := sub
	emailCopy := email
	u := &models.User{
		ID:           int64(len(m.usersByUsername) + 1),
		Username:     username,
		Role:         role,
		AuthProvider: "oidc",
		OIDCSub:      &subCopy,
		Email:        &emailCopy,
		CreatedAt:    time.Now(),
	}
	m.usersBySub[sub] = u
	m.usersByUsername[username] = u
	if email != "" {
		m.usersByEmail[email] = u
	}
	return u, nil
}

func (m *mockRepo) LinkOIDCUser(ctx context.Context, userID int64, sub string, email string) error {
	for _, u := range m.usersByUsername {
		if u.ID == userID {
			subCopy := sub
			u.OIDCSub = &subCopy
			m.usersBySub[sub] = u
			if email != "" {
				emailCopy := email
				u.Email = &emailCopy
				m.usersByEmail[email] = u
			}
			return nil
		}
	}
	return nil
}

func (m *mockRepo) UpdateUserRole(ctx context.Context, id int64, role string) error {
	for _, u := range m.usersByUsername {
		if u.ID == id {
			u.Role = role
			return nil
		}
	}
	return nil
}

func (m *mockRepo) CreateOIDCAuthState(ctx context.Context, stateID, nonce, codeVerifier, redirectURL string, expiresAt time.Time) error {
	if m.errOnState {
		return errors.New("db error on save state")
	}
	m.authStates[stateID] = &models.OIDCAuthState{
		StateID:      stateID,
		Nonce:        nonce,
		CodeVerifier: codeVerifier,
		RedirectURL:  redirectURL,
		ExpiresAt:    expiresAt,
	}
	return nil
}

func (m *mockRepo) GetAndConsumeOIDCAuthState(ctx context.Context, stateID string) (*models.OIDCAuthState, error) {
	if m.errOnState {
		return nil, errors.New("db error on get state")
	}
	st, ok := m.authStates[stateID]
	if !ok {
		return nil, nil
	}
	delete(m.authStates, stateID)
	return st, nil
}

func (m *mockRepo) CleanExpiredOIDCStates(ctx context.Context) error {
	return nil
}

func (m *mockRepo) CreateLoginEvent(ctx context.Context, username, ipAddress, userAgent string, success bool) error {
	m.loginEvents = append(m.loginEvents, username)
	return nil
}

type mockIssuer struct {
	errOnIssue bool
}

func (m *mockIssuer) IssueSession(ctx context.Context, user *models.User) (*authnsvc.SessionTokens, error) {
	if m.errOnIssue {
		return nil, errors.New("issue session failed")
	}
	return &authnsvc.SessionTokens{
		AccessToken:      "mock_access",
		AccessExpiresAt:  time.Now().Add(15 * time.Minute),
		RefreshToken:     "mock_refresh",
		RefreshExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		CSRFToken:        "mock_csrf",
	}, nil
}

// setupMockOIDCServer creates an in-memory OIDC identity provider server with JWKS endpoint.
func setupMockOIDCServer(t *testing.T) (*httptest.Server, *rsa.PrivateKey, string) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate rsa key: %v", err)
	}

	keyID := "test-key-id"
	var server *httptest.Server

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"issuer":                 server.URL,
				"authorization_endpoint": server.URL + "/auth",
				"token_endpoint":         server.URL + "/token",
				"jwks_uri":               server.URL + "/keys",
				"id_token_signing_alg_values_supported": []string{"RS256"},
				"response_types_supported":              []string{"code"},
				"subject_types_supported":               []string{"public"},
			})
		case "/keys":
			w.Header().Set("Content-Type", "application/json")
			pubKey := &privKey.PublicKey
			nStr := base64.RawURLEncoding.EncodeToString(pubKey.N.Bytes())
			eStr := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pubKey.E)).Bytes())

			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"keys": []map[string]interface{}{
					{
						"kty": "RSA",
						"use": "sig",
						"alg": "RS256",
						"kid": keyID,
						"n":   nStr,
						"e":   eStr,
					},
				},
			})
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			// Return a token response
			codeVal := r.FormValue("code")
			if codeVal == "exchange-error" {
				http.Error(w, `{"error":"invalid_grant"}`, 400)
				return
			}
			if codeVal == "no-id-token" {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"access_token": "mock-access-token",
					"token_type":   "Bearer",
				})
				return
			}

			nonceVal := "valid-nonce"
			if codeVal == "bad-nonce" {
				nonceVal = "wrong-nonce"
			}

			groupsVal := []interface{}{"test-admins", "custom-role"}
			if codeVal == "no-group" {
				groupsVal = []interface{}{}
			}

			claims := jwt.MapClaims{
				"iss":                server.URL,
				"sub":                "sub-test-999",
				"aud":                "test-client-id",
				"exp":                time.Now().Add(1 * time.Hour).Unix(),
				"iat":                time.Now().Unix(),
				"nonce":              nonceVal,
				"email":              "test@example.com",
				"email_verified":     true,
				"preferred_username": "testuser",
				"name":               "Test User",
				"groups":             groupsVal,
			}
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			token.Header["kid"] = keyID
			signedIDToken, err := token.SignedString(privKey)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}

			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "mock-access-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
				"id_token":     signedIDToken,
			})
		default:
			http.NotFound(w, r)
		}
	})

	server = httptest.NewServer(handler)
	return server, privKey, keyID
}

func TestStatus(t *testing.T) {
	cfg := &config.Config{
		OIDCEnabled:         true,
		OIDCDisplayName:     "Authentik SSO",
		OIDCAllowLocalLogin: false,
	}
	svc := NewService(newMockRepo(), &mockIssuer{}, cfg)
	st := svc.Status()
	if !st.Enabled || st.DisplayName != "Authentik SSO" || st.AllowLocalLogin != false {
		t.Fatalf("unexpected status response: %+v", st)
	}
}

func TestGetProvider_Errors(t *testing.T) {
	ctx := context.Background()

	// 1. OIDC disabled
	cfg := &config.Config{OIDCEnabled: false}
	svc := NewService(newMockRepo(), &mockIssuer{}, cfg)
	_, _, err := svc.getProvider(ctx)
	if err == nil {
		t.Fatal("expected error when OIDC is disabled")
	}

	// 2. Issuer URL empty
	cfg.OIDCEnabled = true
	cfg.OIDCIssuerURL = ""
	_, _, err = svc.getProvider(ctx)
	if err == nil {
		t.Fatal("expected error when OIDCIssuerURL is empty")
	}

	// 3. Unreachable issuer URL
	cfg.OIDCIssuerURL = "http://127.0.0.1:54321/nonexistent"
	_, _, err = svc.getProvider(ctx)
	if err == nil {
		t.Fatal("expected error when OIDC provider is unreachable")
	}
}

func TestBeginAuth_SuccessAndErrors(t *testing.T) {
	ctx := context.Background()
	server, _, _ := setupMockOIDCServer(t)
	defer server.Close()

	cfg := &config.Config{
		OIDCEnabled:     true,
		OIDCIssuerURL:   server.URL,
		OIDCClientID:    "test-client-id",
		OIDCRedirectURL: "http://localhost:8080/api/auth/oidc/callback",
		OIDCScopes:      []string{"openid", "profile", "email"},
	}

	repo := newMockRepo()
	svc := NewService(repo, &mockIssuer{}, cfg)

	// 1. Success with returnURL
	authURL, err := svc.BeginAuth(ctx, "/dashboard")
	if err != nil {
		t.Fatalf("BeginAuth failed: %v", err)
	}
	if authURL == "" {
		t.Fatal("expected non-empty authURL")
	}
	if len(repo.authStates) != 1 {
		t.Fatalf("expected 1 auth state saved, got %d", len(repo.authStates))
	}

	// 2. Success with invalid returnURL fallback to /
	authURL2, err := svc.BeginAuth(ctx, "//evil.com")
	if err != nil {
		t.Fatalf("BeginAuth failed: %v", err)
	}
	if authURL2 == "" {
		t.Fatal("expected non-empty authURL")
	}

	// 3. Error when OIDC is disabled
	cfg.OIDCEnabled = false
	_, err = svc.BeginAuth(ctx, "/")
	if err == nil {
		t.Fatal("expected error when OIDC is disabled")
	}

	// 4. Error when repo fails
	cfg.OIDCEnabled = true
	repo.errOnState = true
	_, err = svc.BeginAuth(ctx, "/")
	if err == nil {
		t.Fatal("expected error when repo fails")
	}
}

func TestCompleteAuth_SuccessAndErrors(t *testing.T) {
	ctx := context.Background()
	server, privKey, keyID := setupMockOIDCServer(t)
	defer server.Close()

	cfg := &config.Config{
		OIDCEnabled:        true,
		OIDCIssuerURL:      server.URL,
		OIDCClientID:       "test-client-id",
		OIDCRedirectURL:    "http://localhost:8080/api/auth/oidc/callback",
		OIDCScopes:         []string{"openid", "profile", "email"},
		OIDCAdminGroup:     "test-admins",
		OIDCAutoCreateUser: true,
	}

	repo := newMockRepo()
	svc := NewService(repo, &mockIssuer{}, cfg)

	// Pre-populate an auth state
	stateID := "valid-state-id"
	nonce := "valid-nonce"
	codeVerifier := "test-code-verifier"
	_ = repo.CreateOIDCAuthState(ctx, stateID, nonce, codeVerifier, "/custom-dest", time.Now().Add(5*time.Minute))

	// 1. Validation errors
	if _, _, _, err := svc.CompleteAuth(ctx, "", "", "127.0.0.1", "ua"); err == nil {
		t.Fatal("expected error for empty state/code")
	}

	cfgDisabled := &config.Config{OIDCEnabled: false}
	svcDisabled := NewService(repo, &mockIssuer{}, cfgDisabled)
	if _, _, _, err := svcDisabled.CompleteAuth(ctx, stateID, "code", "127.0.0.1", "ua"); err == nil {
		t.Fatal("expected error when OIDC is disabled")
	}

	// 2. Non-existent / expired state
	if _, _, _, err := svc.CompleteAuth(ctx, "invalid-state", "code", "127.0.0.1", "ua"); err == nil {
		t.Fatal("expected error for invalid state")
	}

	// 3. Successful exchange
	user, tokens, redirectURL, err := svc.CompleteAuth(ctx, stateID, "test-code", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("CompleteAuth failed: %v", err)
	}
	if user == nil || tokens == nil || redirectURL != "/custom-dest" {
		t.Fatalf("unexpected response: user=%v, tokens=%v, redir=%s", user, tokens, redirectURL)
	}

	// 4. Code exchange error
	stateErr := "state-exchange-err"
	_ = repo.CreateOIDCAuthState(ctx, stateErr, "valid-nonce", "test-code-verifier", "/", time.Now().Add(5*time.Minute))
	_, _, _, err = svc.CompleteAuth(ctx, stateErr, "exchange-error", "127.0.0.1", "ua")
	if err == nil {
		t.Fatal("expected error on code exchange failure")
	}

	// 5. No ID token in token response
	stateNoID := "state-no-id"
	_ = repo.CreateOIDCAuthState(ctx, stateNoID, "valid-nonce", "test-code-verifier", "/", time.Now().Add(5*time.Minute))
	_, _, _, err = svc.CompleteAuth(ctx, stateNoID, "no-id-token", "127.0.0.1", "ua")
	if err == nil {
		t.Fatal("expected error when no ID token is returned")
	}

	// 6. Nonce mismatch
	stateBadNonce := "state-bad-nonce"
	_ = repo.CreateOIDCAuthState(ctx, stateBadNonce, "valid-nonce", "test-code-verifier", "/", time.Now().Add(5*time.Minute))
	_, _, _, err = svc.CompleteAuth(ctx, stateBadNonce, "bad-nonce", "127.0.0.1", "ua")
	if err == nil {
		t.Fatal("expected error on nonce mismatch")
	}

	// 7. Role resolution error
	cfgRoleDeny := &config.Config{
		OIDCEnabled:        true,
		OIDCIssuerURL:      server.URL,
		OIDCClientID:       "test-client-id",
		OIDCRedirectURL:    "http://localhost:8080/api/auth/oidc/callback",
		OIDCScopes:         []string{"openid", "profile", "email"},
		OIDCDefaultRole:    "none",
		OIDCAutoCreateUser: true,
	}
	svcRoleDeny := NewService(repo, &mockIssuer{}, cfgRoleDeny)
	stateNoGrp := "state-no-group"
	_ = repo.CreateOIDCAuthState(ctx, stateNoGrp, "valid-nonce", "test-code-verifier", "/", time.Now().Add(5*time.Minute))
	_, _, _, err = svcRoleDeny.CompleteAuth(ctx, stateNoGrp, "no-group", "127.0.0.1", "ua")
	if err == nil {
		t.Fatal("expected error when user has no matching group and default role is none")
	}

	// 8. Session issuer error
	svcIssuerErr := NewService(repo, &mockIssuer{errOnIssue: true}, cfg)
	stateIssErr := "state-issuer-err"
	_ = repo.CreateOIDCAuthState(ctx, stateIssErr, "valid-nonce", "test-code-verifier", "/", time.Now().Add(5*time.Minute))
	_, _, _, err = svcIssuerErr.CompleteAuth(ctx, stateIssErr, "test-code", "127.0.0.1", "ua")
	if err == nil {
		t.Fatal("expected error when session issuer fails")
	}

	_ = privKey
	_ = keyID
}

func TestResolveRole(t *testing.T) {
	cfg := &config.Config{
		OIDCAdminGroup:    "admin-group",
		OIDCOperatorGroup: "op-group",
		OIDCViewerGroup:   "view-group",
		OIDCDefaultRole:   "viewer",
	}
	svc := NewService(newMockRepo(), &mockIssuer{}, cfg)

	tests := []struct {
		name     string
		claims   models.OIDCTokenClaims
		expected string
		wantErr  bool
	}{
		{
			name:     "admin group match",
			claims:   models.OIDCTokenClaims{Groups: []string{"dev", "admin-group"}},
			expected: models.RoleAdmin,
		},
		{
			name:     "operator group match",
			claims:   models.OIDCTokenClaims{Groups: []string{"op-group"}},
			expected: models.RoleOperator,
		},
		{
			name:     "viewer group match",
			claims:   models.OIDCTokenClaims{Groups: []string{"view-group"}},
			expected: models.RoleViewer,
		},
		{
			name:     "default role fallback",
			claims:   models.OIDCTokenClaims{Groups: []string{"other"}},
			expected: models.RoleViewer,
		},
		{
			name:     "deny when role none",
			claims:   models.OIDCTokenClaims{Groups: []string{"other"}},
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				cfg.OIDCDefaultRole = "none"
			} else {
				cfg.OIDCDefaultRole = "viewer"
			}
			role, err := svc.resolveRole(tt.claims)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveRole() error = %v, wantErr %v", err, tt.wantErr)
			}
			if role != tt.expected {
				t.Fatalf("resolveRole() = %v, want %v", role, tt.expected)
			}
		})
	}
}

func TestExtractClaims(t *testing.T) {
	cfg := &config.Config{
		OIDCUsernameClaim: "custom_user",
		OIDCEmailClaim:    "contact_email",
		OIDCGroupsClaim:   "custom_groups",
	}
	svc := NewService(newMockRepo(), &mockIssuer{}, cfg)

	claimsMap := map[string]interface{}{
		"custom_user":    "custom_john",
		"contact_email":  "john@example.com",
		"email_verified": true,
		"name":           "John Doe",
		"custom_groups":  []interface{}{"grp1", "grp2"},
		"roles":          []string{"role1"},
	}

	claims := svc.extractClaims(claimsMap, "sub-fallback")
	if claims.PreferredUsername != "custom_john" {
		t.Fatalf("expected preferred_username custom_john, got %s", claims.PreferredUsername)
	}
	if claims.Email != "john@example.com" || !claims.EmailVerified {
		t.Fatalf("expected email john@example.com, got %s", claims.Email)
	}
	if len(claims.Groups) != 2 || claims.Groups[0] != "grp1" {
		t.Fatalf("expected groups [grp1 grp2], got %v", claims.Groups)
	}

	// Test fallback claims
	cfgFallback := &config.Config{}
	svcFallback := NewService(newMockRepo(), &mockIssuer{}, cfgFallback)
	fallbackMap := map[string]interface{}{
		"preferred_username": "fallback_user",
		"email":              "fallback@example.com",
		"groups":             "single_group",
	}
	fbClaims := svcFallback.extractClaims(fallbackMap, "sub-fb")
	if fbClaims.PreferredUsername != "fallback_user" || len(fbClaims.Groups) != 1 {
		t.Fatalf("unexpected fallback claims: %+v", fbClaims)
	}
}

func TestProvisionOrSyncUser(t *testing.T) {
	cfg := &config.Config{
		OIDCAutoCreateUser: true,
	}
	repo := newMockRepo()
	svc := NewService(repo, &mockIssuer{}, cfg)

	ctx := context.Background()
	claims := models.OIDCTokenClaims{
		Sub:               "sub-12345",
		Email:             "alice@example.com",
		PreferredUsername: "alice",
	}

	// 1. First login: auto provision
	user, err := svc.provisionOrSyncUser(ctx, claims, models.RoleOperator)
	if err != nil {
		t.Fatalf("provisionOrSyncUser failed: %v", err)
	}
	if user.Username != "alice" || user.Role != models.RoleOperator || user.AuthProvider != "oidc" {
		t.Fatalf("unexpected user: %+v", user)
	}

	// 2. Second login: user found by sub and role updated
	user2, err := svc.provisionOrSyncUser(ctx, claims, models.RoleAdmin)
	if err != nil {
		t.Fatalf("second login failed: %v", err)
	}
	if user2.ID != user.ID || user2.Role != models.RoleAdmin {
		t.Fatalf("expected role update to admin, got %+v", user2)
	}

	// 3. User found by email linking
	claimsEmail := models.OIDCTokenClaims{
		Sub:   "sub-email-only",
		Email: "alice@example.com",
	}
	user3, err := svc.provisionOrSyncUser(ctx, claimsEmail, models.RoleAdmin)
	if err != nil || user3.ID != user.ID {
		t.Fatalf("expected user to link via email, got %+v", user3)
	}

	// 4. Auto-create disabled error
	cfgDisabled := &config.Config{OIDCAutoCreateUser: false}
	svcDisabled := NewService(newMockRepo(), &mockIssuer{}, cfgDisabled)
	_, err = svcDisabled.provisionOrSyncUser(ctx, models.OIDCTokenClaims{Sub: "new-sub"}, models.RoleViewer)
	if err == nil {
		t.Fatal("expected error when auto-create is disabled")
	}
}

func TestHelpers(t *testing.T) {
	hexVal, err := generateRandomHex(16)
	if err != nil || len(hexVal) != 32 {
		t.Fatalf("generateRandomHex failed: %s, %v", hexVal, err)
	}

	suffix := generateRandomSuffix(4)
	if len(suffix) != 8 {
		t.Fatalf("generateRandomSuffix unexpected len: %d", len(suffix))
	}

	challenge := computePKCEChallenge("test-verifier")
	if challenge == "" {
		t.Fatal("expected non-empty pkce challenge")
	}

	if min(3, 5) != 3 || min(10, 2) != 2 {
		t.Fatal("min helper failure")
	}
}
