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
	discoverysvc "github.com/serversupervisor/server/internal/services/discovery"
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
	h := handlers.NewHostHandler(hostsvc.NewService(db, dispatch.New(db), func() string { return "" }, nil))

	r := gin.New()
	r.Use(withRole(role))
	r.POST("/hosts", h.RegisterHost)
	r.POST("/hosts/bulk", h.RegisterHostsBulk)
	r.GET("/hosts", h.ListHosts)
	r.GET("/hosts/:id", h.GetHost)
	r.PATCH("/hosts/:id", h.UpdateHost)
	r.DELETE("/hosts/:id", h.DeleteHost)
	r.POST("/hosts/:id/rotate-key", h.RotateAPIKey)
	return r, db
}

func newDiscoveryRouter(t *testing.T, role string) *gin.Engine {
	t.Helper()
	db, _ := testutil.NewPostgresDBWithConfig(t)
	h := handlers.NewDiscoveryHandler(discoverysvc.NewService(db))

	r := gin.New()
	r.Use(withRole(role))
	r.POST("/hosts/discover", h.Scan)
	return r
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

func TestHostsRegisterBulk(t *testing.T) {
	r, _ := newHostsRouter(t, "admin")
	w := doJSON(t, r, http.MethodPost, "/hosts/bulk", map[string]any{
		"hosts": []map[string]any{
			{"name": "disc-1", "ip_address": "10.10.10.1"},
			{"name": "disc-2", "ip_address": "not-an-ip"},
			{"name": "disc-3", "ip_address": "10.10.10.3", "tags": []string{"discovered"}},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("bulk register status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Created int `json:"created"`
		Results []struct {
			Name    string `json:"name"`
			Created bool   `json:"created"`
			HostID  string `json:"host_id"`
			APIKey  string `json:"api_key"`
			Error   string `json:"error"`
		} `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Created != 2 {
		t.Fatalf("created = %d, want 2", resp.Created)
	}
	if len(resp.Results) != 3 {
		t.Fatalf("results = %d, want 3", len(resp.Results))
	}
	if !resp.Results[0].Created || resp.Results[0].HostID == "" || resp.Results[0].APIKey == "" {
		t.Errorf("disc-1 should be created with id+key, got %+v", resp.Results[0])
	}
	if resp.Results[1].Created || resp.Results[1].Error == "" {
		t.Errorf("disc-2 (bad ip) should fail, got %+v", resp.Results[1])
	}
	if !resp.Results[2].Created {
		t.Errorf("disc-3 should be created, got %+v", resp.Results[2])
	}

	// Both valid hosts should now be listed.
	wl := doJSON(t, r, http.MethodGet, "/hosts", nil)
	var hosts []map[string]any
	_ = json.Unmarshal(wl.Body.Bytes(), &hosts)
	if len(hosts) != 2 {
		t.Fatalf("expected 2 persisted hosts, got %d", len(hosts))
	}
}

func TestHostsRegisterBulkForbiddenForNonAdmin(t *testing.T) {
	r, _ := newHostsRouter(t, "operator")
	w := doJSON(t, r, http.MethodPost, "/hosts/bulk", map[string]any{
		"hosts": []map[string]any{{"name": "x", "ip_address": "10.0.0.1"}},
	})
	if w.Code != http.StatusForbidden {
		t.Errorf("operator bulk register = %d, want 403", w.Code)
	}
}

func TestDiscoveryScan_RejectsBadCIDR(t *testing.T) {
	r := newDiscoveryRouter(t, "admin")
	w := doJSON(t, r, http.MethodPost, "/hosts/discover", map[string]any{"cidr": "not-a-cidr"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("bad cidr = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}

func TestDiscoveryScan_RejectsTooLargeNetwork(t *testing.T) {
	r := newDiscoveryRouter(t, "admin")
	w := doJSON(t, r, http.MethodPost, "/hosts/discover", map[string]any{"cidr": "10.0.0.0/8"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("/8 cidr = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}

func TestDiscoveryScan_ForbiddenForNonAdmin(t *testing.T) {
	r := newDiscoveryRouter(t, "viewer")
	w := doJSON(t, r, http.MethodPost, "/hosts/discover", map[string]any{"cidr": "10.0.0.0/30"})
	if w.Code != http.StatusForbidden {
		t.Errorf("viewer scan = %d, want 403", w.Code)
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
