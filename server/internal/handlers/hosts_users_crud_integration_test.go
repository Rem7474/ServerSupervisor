package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/handlers"
	hostsvc "github.com/serversupervisor/server/internal/services/host"
	usersvc "github.com/serversupervisor/server/internal/services/user"
	"github.com/serversupervisor/server/internal/testutil"
)

// withRole returns a middleware that injects a role + username, standing in for
// the JWT/permission middlewares the routes normally carry.
func withRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("role", role)
		c.Set("username", "tester")
		c.Next()
	}
}

func newHostsRouter(t *testing.T, role string) (*gin.Engine, *database.DB) {
	t.Helper()
	db, _ := testutil.NewPostgresDBWithConfig(t)
	h := handlers.NewHostHandler(hostsvc.NewService(db, dispatch.New(db), func() string { return "" }))

	r := gin.New()
	r.Use(withRole(role))
	r.POST("/hosts", h.RegisterHost)
	r.GET("/hosts", h.ListHosts)
	r.GET("/hosts/:id", h.GetHost)
	r.PATCH("/hosts/:id", h.UpdateHost)
	r.DELETE("/hosts/:id", h.DeleteHost)
	r.POST("/hosts/:id/rotate-key", h.RotateAPIKey)
	return r, db
}

func TestHostsCRUD(t *testing.T) {
	r, _ := newHostsRouter(t, "admin")

	// Register
	w := doJSON(t, r, http.MethodPost, "/hosts", map[string]any{
		"name": "web-1", "ip_address": "10.0.0.1", "tags": []string{"prod"},
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", w.Code, w.Body.String())
	}
	var reg struct {
		ID     string `json:"id"`
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &reg); err != nil {
		t.Fatalf("decode register: %v", err)
	}
	if reg.ID == "" || reg.APIKey == "" {
		t.Fatalf("register returned empty id/api_key: %+v", reg)
	}
	idPath := "/hosts/" + reg.ID

	// List contains it
	wl := doJSON(t, r, http.MethodGet, "/hosts", nil)
	if wl.Code != http.StatusOK {
		t.Fatalf("list status = %d", wl.Code)
	}
	var hosts []map[string]any
	_ = json.Unmarshal(wl.Body.Bytes(), &hosts)
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}

	// Get
	if g := doJSON(t, r, http.MethodGet, idPath, nil); g.Code != http.StatusOK {
		t.Fatalf("get status = %d", g.Code)
	}

	// Update name
	u := doJSON(t, r, http.MethodPatch, idPath, map[string]any{"name": "web-renamed"})
	if u.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", u.Code, u.Body.String())
	}
	var updated map[string]any
	_ = json.Unmarshal(u.Body.Bytes(), &updated)
	if updated["name"] != "web-renamed" {
		t.Errorf("name = %v, want web-renamed", updated["name"])
	}

	// Rotate API key -> new key, different from the original
	rk := doJSON(t, r, http.MethodPost, idPath+"/rotate-key", nil)
	if rk.Code != http.StatusOK {
		t.Fatalf("rotate status = %d", rk.Code)
	}
	var rotated struct {
		APIKey string `json:"api_key"`
	}
	_ = json.Unmarshal(rk.Body.Bytes(), &rotated)
	if rotated.APIKey == "" || rotated.APIKey == reg.APIKey {
		t.Errorf("rotated key invalid (empty or unchanged)")
	}

	// Delete then 404
	if d := doJSON(t, r, http.MethodDelete, idPath, nil); d.Code != http.StatusOK {
		t.Fatalf("delete status = %d", d.Code)
	}
	if g := doJSON(t, r, http.MethodGet, idPath, nil); g.Code != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", g.Code)
	}
}

func TestHostsRegisterValidation(t *testing.T) {
	r, _ := newHostsRouter(t, "admin")

	// Missing required ip_address -> 400
	if w := doJSON(t, r, http.MethodPost, "/hosts", map[string]any{"name": "x"}); w.Code != http.StatusBadRequest {
		t.Errorf("missing ip = %d, want 400", w.Code)
	}
	// Invalid IP -> 400
	if w := doJSON(t, r, http.MethodPost, "/hosts", map[string]any{"name": "x", "ip_address": "not-an-ip"}); w.Code != http.StatusBadRequest {
		t.Errorf("invalid ip = %d, want 400", w.Code)
	}
	// Update with no fields -> 400 (needs an existing host)
	w := doJSON(t, r, http.MethodPost, "/hosts", map[string]any{"name": "y", "ip_address": "10.0.0.2"})
	var reg struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &reg)
	if e := doJSON(t, r, http.MethodPatch, "/hosts/"+reg.ID, map[string]any{}); e.Code != http.StatusBadRequest {
		t.Errorf("empty update = %d, want 400", e.Code)
	}
}

func TestHostsRegisterForbiddenForNonAdmin(t *testing.T) {
	r, _ := newHostsRouter(t, "viewer")
	w := doJSON(t, r, http.MethodPost, "/hosts", map[string]any{"name": "x", "ip_address": "10.0.0.1"})
	if w.Code != http.StatusForbidden {
		t.Errorf("viewer register = %d, want 403", w.Code)
	}
}

func newUsersRouter(t *testing.T, role string) (*gin.Engine, *database.DB) {
	t.Helper()
	db, _ := testutil.NewPostgresDBWithConfig(t)
	h := handlers.NewUserHandler(usersvc.NewService(db))

	r := gin.New()
	r.Use(withRole(role))
	r.GET("/users", h.ListUsers)
	r.POST("/users", h.CreateUser)
	r.PATCH("/users/:id/role", h.UpdateUserRole)
	r.DELETE("/users/:id", h.DeleteUser)
	return r, db
}

func TestUsersCRUD(t *testing.T) {
	r, _ := newUsersRouter(t, "admin")

	// Create
	w := doJSON(t, r, http.MethodPost, "/users", map[string]any{
		"username": "alice", "password": "correct-horse", "role": "viewer",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}

	// List + find id
	wl := doJSON(t, r, http.MethodGet, "/users", nil)
	if wl.Code != http.StatusOK {
		t.Fatalf("list status = %d", wl.Code)
	}
	var users []struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	if err := json.Unmarshal(wl.Body.Bytes(), &users); err != nil {
		t.Fatalf("decode users: %v", err)
	}
	var id int64
	for _, u := range users {
		if u.Username == "alice" {
			id = u.ID
		}
	}
	if id == 0 {
		t.Fatalf("created user not found in list (%d users)", len(users))
	}

	// Update role
	if u := doJSON(t, r, http.MethodPatch, fmt.Sprintf("/users/%d/role", id), map[string]any{"role": "operator"}); u.Code != http.StatusOK {
		t.Fatalf("update role status = %d, body = %s", u.Code, u.Body.String())
	}

	// Delete
	if d := doJSON(t, r, http.MethodDelete, fmt.Sprintf("/users/%d", id), nil); d.Code != http.StatusOK {
		t.Fatalf("delete status = %d", d.Code)
	}
}

func TestUsersCreateValidation(t *testing.T) {
	r, _ := newUsersRouter(t, "admin")

	// Short password -> 400
	if w := doJSON(t, r, http.MethodPost, "/users", map[string]any{"username": "bob", "password": "short", "role": "viewer"}); w.Code != http.StatusBadRequest {
		t.Errorf("short password = %d, want 400", w.Code)
	}
	// Invalid role -> 400
	if w := doJSON(t, r, http.MethodPost, "/users", map[string]any{"username": "bob", "password": "longenough", "role": "superuser"}); w.Code != http.StatusBadRequest {
		t.Errorf("invalid role = %d, want 400", w.Code)
	}
	// Duplicate username -> 409
	if w := doJSON(t, r, http.MethodPost, "/users", map[string]any{"username": "carol", "password": "longenough", "role": "viewer"}); w.Code != http.StatusCreated {
		t.Fatalf("first create = %d, want 201", w.Code)
	}
	if w := doJSON(t, r, http.MethodPost, "/users", map[string]any{"username": "carol", "password": "longenough", "role": "viewer"}); w.Code != http.StatusConflict {
		t.Errorf("duplicate username = %d, want 409", w.Code)
	}
}

func TestUsersForbiddenForNonAdmin(t *testing.T) {
	r, _ := newUsersRouter(t, "viewer")
	if w := doJSON(t, r, http.MethodGet, "/users", nil); w.Code != http.StatusForbidden {
		t.Errorf("viewer list users = %d, want 403", w.Code)
	}
	if w := doJSON(t, r, http.MethodPost, "/users", map[string]any{"username": "x", "password": "longenough", "role": "viewer"}); w.Code != http.StatusForbidden {
		t.Errorf("viewer create user = %d, want 403", w.Code)
	}
}
