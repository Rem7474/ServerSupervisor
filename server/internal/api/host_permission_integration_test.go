package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/serversupervisor/server/internal/api"
	"github.com/serversupervisor/server/internal/cookies"
	"github.com/serversupervisor/server/internal/handlers"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// issueJWT signs a token with the test config and returns its raw string.
func issueJWT(t *testing.T, secret, username, role string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  username,
		"role": role,
	})
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return s
}

// authedRequest builds a GET request that carries the session cookie so the
// JWT + permission middlewares treat the caller as logged in.
func authedRequest(t *testing.T, path, jwtToken string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(&http.Cookie{Name: cookies.AccessTokenName, Value: jwtToken})
	return req
}

func TestHostPermission_AdminBypassesRestrictions(t *testing.T) {
	db, cfg := testutil.NewPostgresDBWithConfig(t)

	// Seed: admin user + one host
	if err := db.CreateUser(context.Background(), "admin", mustHash(t, "x"), "admin"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.RegisterHost(context.Background(), &models.Host{
		ID: "host-1", Name: "h1", Hostname: "h1.local", Status: "online",
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	r := gin.New()
	g := r.Group("/api/v1")
	g.Use(api.JWTMiddleware(cfg))
	g.GET("/hosts/:id", api.HostPermissionMiddleware(db, "viewer"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	token := issueJWT(t, cfg.JWTSecret, "admin", "admin")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(t, "/api/v1/hosts/host-1", token))
	if w.Code != http.StatusOK {
		t.Fatalf("admin must always pass, got %d", w.Code)
	}
}

func TestHostPermission_NonAdminWithoutEntriesUsesGlobalRole(t *testing.T) {
	db, cfg := testutil.NewPostgresDBWithConfig(t)

	// bob is operator at the global level and has NO host_permissions rows.
	if err := db.CreateUser(context.Background(), "bob", mustHash(t, "x"), "operator"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.RegisterHost(context.Background(), &models.Host{ID: "host-1", Name: "h1", Hostname: "h1.local", Status: "online"}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	r := gin.New()
	g := r.Group("/api/v1")
	g.Use(api.JWTMiddleware(cfg))
	g.GET("/hosts/:id", api.HostPermissionMiddleware(db, "viewer"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	token := issueJWT(t, cfg.JWTSecret, "bob", "operator")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(t, "/api/v1/hosts/host-1", token))
	if w.Code != http.StatusOK {
		t.Fatalf("no host_permissions rows ⇒ global role applies, expected 200, got %d", w.Code)
	}
}

func TestHostPermission_RestrictedUserBlockedOnUnlistedHost(t *testing.T) {
	db, cfg := testutil.NewPostgresDBWithConfig(t)

	if err := db.CreateUser(context.Background(), "carol", mustHash(t, "x"), "viewer"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.RegisterHost(context.Background(), &models.Host{ID: "host-1", Name: "h1", Hostname: "h1.local", Status: "online"}); err != nil {
		t.Fatalf("register host: %v", err)
	}
	if err := db.RegisterHost(context.Background(), &models.Host{ID: "host-2", Name: "h2", Hostname: "h2.local", Status: "online"}); err != nil {
		t.Fatalf("register host: %v", err)
	}
	// carol is restricted to host-1 only.
	if err := db.SetHostPermission(context.Background(), "carol", "host-1", "viewer"); err != nil {
		t.Fatalf("set perm: %v", err)
	}

	r := gin.New()
	g := r.Group("/api/v1")
	g.Use(api.JWTMiddleware(cfg))
	g.GET("/hosts/:id", api.HostPermissionMiddleware(db, "viewer"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	token := issueJWT(t, cfg.JWTSecret, "carol", "viewer")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(t, "/api/v1/hosts/host-1", token))
	if w.Code != http.StatusOK {
		t.Fatalf("allowed host must return 200, got %d", w.Code)
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, authedRequest(t, "/api/v1/hosts/host-2", token))
	if w2.Code != http.StatusForbidden {
		t.Fatalf("unauthorised host must return 403, got %d", w2.Code)
	}
}

func TestHostPermission_ViewerCannotPerformOperatorAction(t *testing.T) {
	db, cfg := testutil.NewPostgresDBWithConfig(t)

	if err := db.CreateUser(context.Background(), "dave", mustHash(t, "x"), "viewer"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.RegisterHost(context.Background(), &models.Host{ID: "host-1", Name: "h1", Hostname: "h1.local", Status: "online"}); err != nil {
		t.Fatalf("register host: %v", err)
	}
	if err := db.SetHostPermission(context.Background(), "dave", "host-1", "viewer"); err != nil {
		t.Fatalf("set perm: %v", err)
	}

	r := gin.New()
	g := r.Group("/api/v1")
	g.Use(api.JWTMiddleware(cfg))
	g.GET("/hosts/:id", api.HostPermissionMiddleware(db, "operator"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	token := issueJWT(t, cfg.JWTSecret, "dave", "viewer")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(t, "/api/v1/hosts/host-1", token))
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer must be denied operator action, got %d", w.Code)
	}
}

func mustHash(t *testing.T, p string) string {
	t.Helper()
	h, err := handlers.HashPassword(p)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	return h
}
