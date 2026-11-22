package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/handlers"
)

// Database-free coverage for issue #485: the global role must still gate
// operator-level / admin-level mutations when the caller has no explicit
// host_permissions rows.

type fakeHostAccess struct {
	restricted bool
	level      string
}

func (f fakeHostAccess) GetHostAccess(context.Context, string, string) (bool, string, error) {
	return f.restricted, f.level, nil
}

// asRole fakes JWTMiddleware: it sets the context values downstream code reads.
func asRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("role", role)
		c.Set("username", "tester")
		c.Next()
	}
}

func do(r http.Handler, method, path, body string) int {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestHostPermissionMiddleware_GlobalRoleWhenNoRows(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name     string
		role     string
		access   fakeHostAccess
		required string
		want     int
	}{
		{"viewer, 0 rows, operator route -> 403", "viewer", fakeHostAccess{}, "operator", http.StatusForbidden},
		{"viewer, 0 rows, viewer route -> 200", "viewer", fakeHostAccess{}, "viewer", http.StatusOK},
		{"operator, 0 rows, operator route -> 200", "operator", fakeHostAccess{}, "operator", http.StatusOK},
		{"admin, operator route -> 200", "admin", fakeHostAccess{}, "operator", http.StatusOK},
		{"viewer with explicit operator grant -> 200", "viewer", fakeHostAccess{restricted: true, level: "operator"}, "operator", http.StatusOK},
		{"operator with explicit viewer grant -> 403", "operator", fakeHostAccess{restricted: true, level: "viewer"}, "operator", http.StatusForbidden},
		{"restricted user on unlisted host -> 403", "operator", fakeHostAccess{restricted: true}, "viewer", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.Use(asRole(tc.role))
			r.POST("/hosts/:id/do", HostPermissionMiddleware(tc.access, tc.required), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})
			if got := do(r, http.MethodPost, "/hosts/host-1/do", ""); got != tc.want {
				t.Fatalf("status = %d, want %d", got, tc.want)
			}
		})
	}
}

// mutationRoute is one write endpoint under test.
type mutationRoute struct{ method, path, body string }

func TestProxmoxMutations_AdminOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	routes := []mutationRoute{
		{http.MethodPost, "/proxmox/links", `{}`},
		{http.MethodPut, "/proxmox/links/l1", `{}`},
		{http.MethodDelete, "/proxmox/links/l1", ``},
		{http.MethodPost, "/proxmox/nodes/n1/services/pveproxy/restart", ``},
		{http.MethodPost, "/proxmox/nodes/n1/apt-refresh", ``},
		{http.MethodPost, "/proxmox/nodes/n1/guests/100/migrate", `{}`},
		{http.MethodPost, "/proxmox/guests/g1/action", `{}`},
	}

	// nil service: AdminOnlyMiddleware must reject non-admins before any
	// handler (and so the service) runs, otherwise this panics.
	h := handlers.NewProxmoxHandler(nil)
	for _, role := range []string{"viewer", "operator", ""} {
		r := gin.New()
		g := r.Group("")
		g.Use(asRole(role))
		registerProxmoxRoutes(g, h)
		for _, rt := range routes {
			if got := do(r, rt.method, rt.path, rt.body); got != http.StatusForbidden {
				t.Errorf("role %q %s %s = %d, want 403", role, rt.method, rt.path, got)
			}
		}
	}

	// Admin passes the gate: the request reaches the handler (which rejects the
	// empty JSON body with 400, or panics on the nil service → recovered 500).
	r := gin.New()
	r.Use(gin.Recovery())
	g := r.Group("")
	g.Use(asRole("admin"))
	registerProxmoxRoutes(g, h)
	for _, rt := range routes {
		if got := do(r, rt.method, rt.path, rt.body); got == http.StatusForbidden || got == http.StatusNotFound {
			t.Errorf("admin %s %s = %d, must reach the handler", rt.method, rt.path, got)
		}
	}
}

func TestProxmoxReads_RemainOpenToViewers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	g := r.Group("")
	g.Use(asRole("viewer"))
	registerProxmoxRoutes(g, handlers.NewProxmoxHandler(nil))
	// nil service → recovered 500, but never the admin gate's 403.
	for _, path := range []string{"/proxmox/links", "/proxmox/links/l1", "/proxmox/nodes/n1/services"} {
		if got := do(r, http.MethodGet, path, ""); got == http.StatusForbidden {
			t.Errorf("viewer GET %s = 403, reads must stay open", path)
		}
	}
}

func TestNetworkConfigSave_RequiresOperator(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := handlers.NewNetworkHandler(nil)
	build := func(role string) *gin.Engine {
		r := gin.New()
		r.Use(gin.Recovery())
		g := r.Group("")
		g.Use(asRole(role))
		registerDockerRoutes(g, nil, nil, h, nil)
		return r
	}

	if got := do(build("viewer"), http.MethodPut, "/network/config", `{}`); got != http.StatusForbidden {
		t.Errorf("viewer PUT /network/config = %d, want 403", got)
	}
	if got := do(build(""), http.MethodPut, "/network/config", `{}`); got != http.StatusForbidden {
		t.Errorf("role-less PUT /network/config = %d, want 403", got)
	}
	// Operator and admin clear the role check; the invalid body is then
	// rejected with 400 before the (nil) service is reached.
	for _, role := range []string{"operator", "admin"} {
		if got := do(build(role), http.MethodPut, "/network/config", `not-json`); got != http.StatusBadRequest {
			t.Errorf("%s PUT /network/config (bad body) = %d, want 400", role, got)
		}
	}
}

func TestRequestLogger_SanitizesLogInjection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/*rest", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/ok?q=a", nil)
	req.URL.Path = "/ok\r\nforged"
	req.URL.RawQuery = "q=a\r\nb"
	req.RemoteAddr = "1.2.3.4\r\nforged:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("expected single JSON log line, got %q: %v", buf.String(), err)
	}
	for _, field := range []string{"path", "query", "client_ip"} {
		val, _ := line[field].(string)
		if strings.ContainsAny(val, "\r\n") {
			t.Errorf("%s contains CR/LF: %q", field, val)
		}
	}
}
