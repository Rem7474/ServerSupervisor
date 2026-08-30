package oidc

import (
	"context"
	"database/sql"
	"testing"
	"time"

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
	if u, ok := m.usersBySub[sub]; ok {
		return u, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockRepo) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	if u, ok := m.usersByEmail[email]; ok {
		return u, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockRepo) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	if u, ok := m.usersByUsername[username]; ok {
		return u, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockRepo) CreateOIDCUser(ctx context.Context, username, email, sub, role string) (*models.User, error) {
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

type mockIssuer struct{}

func (m *mockIssuer) IssueSession(ctx context.Context, user *models.User) (*authnsvc.SessionTokens, error) {
	return &authnsvc.SessionTokens{
		AccessToken:      "mock_access",
		AccessExpiresAt:  time.Now().Add(15 * time.Minute),
		RefreshToken:     "mock_refresh",
		RefreshExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		CSRFToken:        "mock_csrf",
	}, nil
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
}
