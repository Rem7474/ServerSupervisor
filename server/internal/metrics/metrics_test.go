package metrics

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	dto "github.com/prometheus/client_model/go"
)

func init() { gin.SetMode(gin.TestMode) }

func newRouter(enabled bool, token string) *gin.Engine {
	r := gin.New()
	r.Use(Middleware())
	r.GET("/metrics", Handler(enabled, token))
	r.GET("/api/v1/hosts/:id", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/boom", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })
	return r
}

func do(r *gin.Engine, path, auth string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func family(t *testing.T, name string) *dto.MetricFamily {
	t.Helper()
	fams, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fams {
		if f.GetName() == name {
			return f
		}
	}
	return nil
}

func labels(m *dto.Metric) map[string]string {
	out := map[string]string{}
	for _, l := range m.GetLabel() {
		out[l.GetName()] = l.GetValue()
	}
	return out
}

func TestMiddlewareUsesRouteTemplateAndStatusClass(t *testing.T) {
	r := newRouter(true, "tok")
	for _, id := range []string{"host-1", "host-2", "host-3"} {
		do(r, "/api/v1/hosts/"+id, "")
	}
	do(r, "/boom", "")

	var hostSeries, boom float64
	var rawPathLeak bool
	for _, m := range family(t, "serversupervisor_http_requests_total").GetMetric() {
		l := labels(m)
		if strings.Contains(l["route"], "host-") {
			rawPathLeak = true
		}
		switch {
		case l["route"] == "/api/v1/hosts/:id" && l["status_class"] == "2xx" && l["method"] == "GET":
			hostSeries = m.GetCounter().GetValue()
		case l["route"] == "/boom" && l["status_class"] == "5xx":
			boom = m.GetCounter().GetValue()
		}
	}
	if rawPathLeak {
		t.Fatal("raw path ended up in the route label")
	}
	if hostSeries < 3 {
		t.Fatalf("host route counted %v, want >= 3 under a single series", hostSeries)
	}
	if boom < 1 {
		t.Fatalf("5xx not counted: %v", boom)
	}
}

func TestMiddlewareBoundsUnmatchedRoutes(t *testing.T) {
	r := newRouter(true, "tok")
	for _, p := range []string{"/wp-login.php", "/a/b/c", "/.env"} {
		if w := do(r, p, ""); w.Code != http.StatusNotFound {
			t.Fatalf("%s: %d", p, w.Code)
		}
	}
	for _, m := range family(t, "serversupervisor_http_requests_total").GetMetric() {
		l := labels(m)
		for _, raw := range []string{"wp-login", "/a/b/c", ".env"} {
			if strings.Contains(l["route"], raw) {
				t.Fatalf("unmatched path %q leaked into label %q", raw, l["route"])
			}
		}
	}
	found := false
	for _, m := range family(t, "serversupervisor_http_requests_total").GetMetric() {
		if l := labels(m); l["route"] == unmatchedRoute && l["status_class"] == "4xx" {
			found = true
		}
	}
	if !found {
		t.Fatal("no series for the unmatched route label")
	}
}

func TestStatusClass(t *testing.T) {
	cases := map[int]string{200: "2xx", 204: "2xx", 301: "3xx", 404: "4xx", 503: "5xx", 0: "unknown", 700: "unknown"}
	for in, want := range cases {
		if got := statusClass(in); got != want {
			t.Errorf("statusClass(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestHandlerDisabledOrWithoutTokenIs404(t *testing.T) {
	for name, r := range map[string]*gin.Engine{
		"disabled":      newRouter(false, "tok"),
		"enabled-empty": newRouter(true, ""),
	} {
		for _, auth := range []string{"", "Bearer tok", "Bearer "} {
			if w := do(r, "/metrics", auth); w.Code != http.StatusNotFound {
				t.Errorf("%s auth=%q: status %d, want 404", name, auth, w.Code)
			}
		}
	}
}

func TestHandlerRejectsMissingOrWrongToken(t *testing.T) {
	r := newRouter(true, "s3cret")
	for _, auth := range []string{"", "Bearer wrong", "Bearer", "Basic s3cret", "s3cret", "Bearer s3cret2"} {
		w := do(r, "/metrics", auth)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("auth=%q: status %d, want 401", auth, w.Code)
		}
		if w.Header().Get("WWW-Authenticate") == "" {
			t.Errorf("auth=%q: missing WWW-Authenticate", auth)
		}
		if strings.Contains(w.Body.String(), "serversupervisor_") {
			t.Errorf("auth=%q: metrics leaked in 401 body", auth)
		}
	}
}

func TestHandlerServesExpectedMetrics(t *testing.T) {
	SetSources(Sources{
		AgentsConnected: func() int { return 4 },
		WSClients:       func() int { return 2 },
		DBStats: func() sql.DBStats {
			return sql.DBStats{OpenConnections: 7, InUse: 3, Idle: 4, MaxOpenConnections: 25, WaitCount: 9, WaitDuration: 2 * time.Second}
		},
	})
	defer SetSources(Sources{})
	CommandDispatched("docker", nil)
	CommandDispatched("docker", errors.New("db down"))
	NotificationSent("ntfy", nil)
	NotificationSent("smtp", errors.New("refused"))

	r := newRouter(true, "s3cret")
	do(r, "/api/v1/hosts/x", "")
	w := do(r, "/metrics", "bEaReR s3cret")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("content type %q", ct)
	}
	body := w.Body.String()
	for _, want := range []string{
		`serversupervisor_http_requests_total{method="GET",route="/api/v1/hosts/:id",status_class="2xx"}`,
		"serversupervisor_http_request_duration_seconds_bucket",
		"serversupervisor_http_requests_in_flight",
		"serversupervisor_agents_connected 4",
		"serversupervisor_websocket_clients 2",
		"serversupervisor_db_connections_open 7",
		"serversupervisor_db_connections_in_use 3",
		"serversupervisor_db_connections_idle 4",
		"serversupervisor_db_connections_max_open 25",
		"serversupervisor_db_connection_wait_total 9",
		"serversupervisor_db_connection_wait_seconds_total 2",
		`serversupervisor_commands_dispatched_total{module="docker",outcome="queued"}`,
		`serversupervisor_commands_dispatched_total{module="docker",outcome="error"}`,
		`serversupervisor_notifications_total{channel="ntfy",outcome="success"}`,
		`serversupervisor_notifications_total{channel="smtp",outcome="error"}`,
		"go_goroutines",
		"process_cpu_seconds_total",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("scrape output lacks %q", want)
		}
	}
}

func TestLiveCollectorNilSourcesReportZero(t *testing.T) {
	SetSources(Sources{})
	f := family(t, "serversupervisor_agents_connected")
	if f == nil || f.GetMetric()[0].GetGauge().GetValue() != 0 {
		t.Fatalf("agents_connected with nil source: %v", f)
	}
}
