package handlers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/models"
	authnsvc "github.com/serversupervisor/server/internal/services/authn"
	oidcsvc "github.com/serversupervisor/server/internal/services/oidc"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type handlerMockRepo struct {
	authStates map[string]*models.OIDCAuthState
}

func (m *handlerMockRepo) GetUserByOIDCSub(ctx context.Context, sub string) (*models.User, error) {
	return &models.User{ID: 1, Username: "testuser", Role: "admin", AuthProvider: "oidc"}, nil
}
func (m *handlerMockRepo) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return nil, nil
}
func (m *handlerMockRepo) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	return nil, nil
}
func (m *handlerMockRepo) CreateOIDCUser(ctx context.Context, username, email, sub, role string) (*models.User, error) {
	return &models.User{ID: 1, Username: username, Role: role, AuthProvider: "oidc"}, nil
}
func (m *handlerMockRepo) LinkOIDCUser(ctx context.Context, userID int64, sub string, email string) error {
	return nil
}
func (m *handlerMockRepo) UpdateUserRole(ctx context.Context, id int64, role string) error {
	return nil
}
func (m *handlerMockRepo) CreateOIDCAuthState(ctx context.Context, stateID, nonce, codeVerifier, redirectURL string, expiresAt time.Time) error {
	m.authStates[stateID] = &models.OIDCAuthState{
		StateID:      stateID,
		Nonce:        nonce,
		CodeVerifier: codeVerifier,
		RedirectURL:  redirectURL,
		ExpiresAt:    expiresAt,
	}
	return nil
}
func (m *handlerMockRepo) GetAndConsumeOIDCAuthState(ctx context.Context, stateID string) (*models.OIDCAuthState, error) {
	st, ok := m.authStates[stateID]
	if !ok {
		return nil, nil
	}
	delete(m.authStates, stateID)
	return st, nil
}
func (m *handlerMockRepo) CleanExpiredOIDCStates(ctx context.Context) error {
	return nil
}
func (m *handlerMockRepo) CreateLoginEvent(ctx context.Context, username, ipAddress, userAgent string, success bool) error {
	return nil
}

type handlerMockIssuer struct{}

func (m *handlerMockIssuer) IssueSession(ctx context.Context, user *models.User) (*authnsvc.SessionTokens, error) {
	return &authnsvc.SessionTokens{
		AccessToken:      "mock_access_token",
		AccessExpiresAt:  time.Now().Add(15 * time.Minute),
		RefreshToken:     "mock_refresh_token",
		RefreshExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		CSRFToken:        "mock_csrf_token",
	}, nil
}

func setupHandlerMockOIDCServer(t *testing.T) (*httptest.Server, *rsa.PrivateKey, string) {
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
			claims := jwt.MapClaims{
				"iss":                server.URL,
				"sub":                "sub-test-111",
				"aud":                "test-client-id",
				"exp":                time.Now().Add(1 * time.Hour).Unix(),
				"iat":                time.Now().Unix(),
				"nonce":              "test-nonce-123",
				"preferred_username": "handleruser",
				"groups":             []string{"admins"},
			}
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			token.Header["kid"] = keyID
			signedIDToken, _ := token.SignedString(privKey)

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

func TestOIDCHandler_GetStatus(t *testing.T) {
	cfg := &config.Config{
		OIDCEnabled:         true,
		OIDCDisplayName:     "Keycloak SSO",
		OIDCAllowLocalLogin: true,
	}
	svc := oidcsvc.NewService(nil, nil, cfg)
	handler := NewOIDCHandler(svc, cfg)

	r := gin.New()
	r.GET("/api/auth/oidc/status", handler.GetStatus)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var res models.OIDCStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !res.Enabled || res.DisplayName != "Keycloak SSO" || !res.AllowLocalLogin {
		t.Fatalf("unexpected status payload: %+v", res)
	}
}

func TestOIDCHandler_Login(t *testing.T) {
	server, _, _ := setupHandlerMockOIDCServer(t)
	defer server.Close()

	cfg := &config.Config{
		OIDCEnabled:     true,
		OIDCIssuerURL:   server.URL,
		OIDCClientID:    "test-client-id",
		OIDCRedirectURL: "http://localhost:8080/api/auth/oidc/callback",
		OIDCScopes:      []string{"openid", "profile"},
	}
	repo := &handlerMockRepo{authStates: make(map[string]*models.OIDCAuthState)}
	svc := oidcsvc.NewService(repo, &handlerMockIssuer{}, cfg)
	handler := NewOIDCHandler(svc, cfg)

	r := gin.New()
	r.GET("/api/auth/oidc/login", handler.Login)

	// 1. JSON Accept header
	reqJSON := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login?return_to=/hosts", nil)
	reqJSON.Header.Set("Accept", "application/json")
	recJSON := httptest.NewRecorder()
	r.ServeHTTP(recJSON, reqJSON)

	if recJSON.Code != http.StatusOK {
		t.Fatalf("expected 200 for JSON login request, got %d", recJSON.Code)
	}
	var res map[string]string
	_ = json.Unmarshal(recJSON.Body.Bytes(), &res)
	if res["auth_url"] == "" {
		t.Fatalf("expected auth_url in JSON response, got %+v", res)
	}

	// 2. Browser redirect
	reqHTML := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login?redirect=/settings", nil)
	recHTML := httptest.NewRecorder()
	r.ServeHTTP(recHTML, reqHTML)

	if recHTML.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect for browser login request, got %d", recHTML.Code)
	}

	// 3. Error when disabled
	cfgDisabled := &config.Config{OIDCEnabled: false}
	svcDisabled := oidcsvc.NewService(repo, &handlerMockIssuer{}, cfgDisabled)
	handlerDisabled := NewOIDCHandler(svcDisabled, cfgDisabled)
	rDisabled := gin.New()
	rDisabled.GET("/api/auth/oidc/login", handlerDisabled.Login)

	reqErr := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login", nil)
	recErr := httptest.NewRecorder()
	rDisabled.ServeHTTP(recErr, reqErr)

	if recErr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when OIDC is disabled, got %d", recErr.Code)
	}
}

func TestOIDCHandler_Callback(t *testing.T) {
	server, _, _ := setupHandlerMockOIDCServer(t)
	defer server.Close()

	cfg := &config.Config{
		OIDCEnabled:     true,
		OIDCIssuerURL:   server.URL,
		OIDCClientID:    "test-client-id",
		OIDCRedirectURL: "http://localhost:8080/api/auth/oidc/callback",
		OIDCScopes:      []string{"openid", "profile"},
		OIDCAdminGroup:  "admins",
	}
	repo := &handlerMockRepo{authStates: make(map[string]*models.OIDCAuthState)}
	svc := oidcsvc.NewService(repo, &handlerMockIssuer{}, cfg)
	handler := NewOIDCHandler(svc, cfg)

	r := gin.New()
	r.GET("/api/auth/oidc/callback", handler.Callback)

	// 1. IdP direct error
	reqIdpErr := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?error=access_denied&error_description=User+cancelled", nil)
	recIdpErr := httptest.NewRecorder()
	r.ServeHTTP(recIdpErr, reqIdpErr)
	if recIdpErr.Code != http.StatusFound || recIdpErr.Header().Get("Location") != "/login?error=sso_error%3A+User+cancelled" {
		t.Fatalf("unexpected response for IdP error: code=%d loc=%s", recIdpErr.Code, recIdpErr.Header().Get("Location"))
	}

	// 2. Missing state/code params
	reqMissing := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback", nil)
	recMissing := httptest.NewRecorder()
	r.ServeHTTP(recMissing, reqMissing)
	if recMissing.Code != http.StatusFound || recMissing.Header().Get("Location") != "/login?error=missing+authorization+code+or+state" {
		t.Fatalf("unexpected response for missing params: code=%d loc=%s", recMissing.Code, recMissing.Header().Get("Location"))
	}

	// 3. CompleteAuth error (e.g. state expired / invalid)
	reqStateErr := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?state=invalid-state&code=any", nil)
	recStateErr := httptest.NewRecorder()
	r.ServeHTTP(recStateErr, reqStateErr)
	if recStateErr.Code != http.StatusFound {
		t.Fatalf("expected redirect for complete auth error, got %d", recStateErr.Code)
	}

	// 4. CompleteAuth success
	stateID := "state-success"
	_ = repo.CreateOIDCAuthState(context.Background(), stateID, "test-nonce-123", "verifier-abc", "/hosts/h1", time.Now().Add(5*time.Minute))

	reqSuccess := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?state="+stateID+"&code=valid-code", nil)
	recSuccess := httptest.NewRecorder()
	r.ServeHTTP(recSuccess, reqSuccess)

	if recSuccess.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect on success, got %d", recSuccess.Code)
	}
	if recSuccess.Header().Get("Location") != "/hosts/h1" {
		t.Fatalf("expected redirect to /hosts/h1, got %s", recSuccess.Header().Get("Location"))
	}
	cookies := recSuccess.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookies to be set")
	}
}
