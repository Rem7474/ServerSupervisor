package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/events"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/scheduler"
	"github.com/serversupervisor/server/internal/testutil"
	"github.com/serversupervisor/server/internal/ws"
)

// TestAPIKeys_EndToEnd drives the real router: a session creates a key, the
// key reads the API, is refused key management, then stops working once
// revoked.
func TestAPIKeys_EndToEnd(t *testing.T) {
	db, cfg := testutil.NewPostgresDBWithConfig(t)
	cfg.RateLimitRPS = 1000
	cfg.RateLimitBurst = 1000
	disp := dispatch.New(db)
	r, _, _, _, cleanup := SetupRouter(db, cfg, ws.NewNotificationHub(), events.NewBus(), scheduler.New(db, disp), disp)
	t.Cleanup(cleanup)

	ctx := context.Background()
	for _, u := range []struct{ name, role string }{{"keyadmin", models.RoleAdmin}, {"keyviewer", models.RoleViewer}} {
		if err := db.CreateUser(ctx, u.name, "x", u.role); err != nil {
			t.Fatal(err)
		}
	}
	session := func(username, role string) string {
		tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": username, "role": role, "exp": time.Now().Add(time.Hour).Unix(),
		}).SignedString([]byte(cfg.JWTSecret))
		if err != nil {
			t.Fatal(err)
		}
		return "Bearer " + tok
	}
	do := func(method, path, auth, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", auth)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	viewer := session("keyviewer", models.RoleViewer)

	w := do(http.MethodPost, "/api/v1/auth/api-keys", viewer, `{"name":"grafana","scope":"read","expires_in_days":30}`)
	if w.Code != http.StatusCreated || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var created models.APIKeyCreated
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	key := "Bearer " + created.Secret

	if w := do(http.MethodPost, "/api/v1/auth/api-keys", viewer, `{"name":"","scope":"read"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid create: %d", w.Code)
	}
	if w := do(http.MethodPost, "/api/v1/auth/api-keys", viewer, `not json`); w.Code != http.StatusBadRequest {
		t.Fatalf("malformed body: %d", w.Code)
	}

	// The listing never exposes the secret.
	w = do(http.MethodGet, "/api/v1/auth/api-keys", viewer, "")
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), created.Secret) || !strings.Contains(w.Body.String(), created.Key.Prefix) {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}

	if w := do(http.MethodGet, "/api/v1/hosts", key, ""); w.Code != http.StatusOK {
		t.Fatalf("key on GET /hosts: %d %s", w.Code, w.Body.String())
	}
	if w := do(http.MethodGet, "/api/v1/auth/api-keys", key, ""); w.Code != http.StatusForbidden {
		t.Fatalf("key on key management: %d", w.Code)
	}
	// Owner is a viewer: the admin key list stays closed to them either way.
	if w := do(http.MethodGet, "/api/v1/api-keys", viewer, ""); w.Code != http.StatusForbidden {
		t.Fatalf("viewer on admin list: %d", w.Code)
	}
	w = do(http.MethodGet, "/api/v1/api-keys", session("keyadmin", models.RoleAdmin), "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "keyviewer") {
		t.Fatalf("admin list: %d %s", w.Code, w.Body.String())
	}

	id := strconv.FormatInt(created.Key.ID, 10)
	if w := do(http.MethodDelete, "/api/v1/auth/api-keys/abc", viewer, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("revoke bad id: %d", w.Code)
	}
	if w := do(http.MethodDelete, "/api/v1/auth/api-keys/"+id, viewer, ""); w.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d %s", w.Code, w.Body.String())
	}
	w = do(http.MethodGet, "/api/v1/hosts", key, "")
	var body gin.H
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != http.StatusUnauthorized || body["i18nKey"] != apperr.CodeAPIKeyRevoked {
		t.Fatalf("revoked key: %d %v", w.Code, body)
	}

	var audits int
	if err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE username = 'keyviewer' AND action IN ('api_key_create', 'api_key_revoke') AND category = 'auth'`,
	).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if audits != 2 {
		t.Fatalf("want 2 auth audit entries (create + revoke), got %d", audits)
	}
}
