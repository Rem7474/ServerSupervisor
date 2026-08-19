package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/serversupervisor/server/internal/cookies"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/events"
	"github.com/serversupervisor/server/internal/models"
	proxmoxsvc "github.com/serversupervisor/server/internal/services/proxmox"
	"github.com/serversupervisor/server/internal/testutil"
)

// fakePVEConsoleServer simulates the three PVE calls a console session needs
// (login, termproxy, vncwebsocket) — see proxmoxclient/console_test.go for
// the same fake, kept independent here since it's a different package and
// this test cares about the ws-handler wiring around it, not the client
// itself.
func fakePVEConsoleServer(t *testing.T) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	mux := http.NewServeMux()
	mux.HandleFunc("/access/ticket", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"ticket":"PVE:root@pam:AUTH=="}}`))
	})
	mux.HandleFunc("/nodes/pve1/lxc/101/termproxy", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"ticket":"PVEVNC:TERM==","port":31000,"user":"root@pam","upid":"UPID:.."}}`))
	})
	mux.HandleFunc("/nodes/pve1/lxc/101/vncwebsocket", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		if _, msg, err := conn.ReadMessage(); err != nil || string(msg) != "root@pam:PVEVNC:TERM==\n" {
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, []byte("OK")); err != nil {
			return
		}
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			s := string(msg)
			switch {
			case strings.HasPrefix(s, "0:"):
				rest := strings.SplitN(s[2:], ":", 2)
				if len(rest) == 2 {
					_ = conn.WriteMessage(websocket.TextMessage, []byte("echo:"+rest[1]))
				}
			case strings.HasPrefix(s, "1:"):
				_ = conn.WriteMessage(websocket.TextMessage, []byte("resized:"+s[2:]))
			}
		}
	})
	return httptest.NewServer(mux)
}

func mintTestJWT(t *testing.T, secret, username, role string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": username, "role": role})
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return s
}

// setupConsoleTestHandler wires a real Postgres-backed WSHandler with one
// LXC guest whose Proxmox connection points at a fake PVE server, so
// ProxmoxConsole can be exercised end-to-end (guest resolution, console
// credential lookup, PVE login/termproxy/websocket handshake, byte relay,
// resize translation, audit log) without a real PVE cluster.
func setupConsoleTestHandler(t *testing.T) (h *WSHandler, guestID string) {
	t.Helper()
	db, cfg := testutil.NewPostgresDBWithConfig(t)
	pve := fakePVEConsoleServer(t)
	t.Cleanup(pve.Close)

	ctx := context.Background()
	connID, err := db.CreateProxmoxConnection(ctx, "test", pve.URL, "user@pve!token", "secret",
		false, true, 60, "root@pam", "hunter2")
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	if err := db.UpsertProxmoxGuest(ctx, connID, "pve1", "lxc", 101, "web1", "running",
		1, 0, 512, 0, 0, 0, 0, ""); err != nil {
		t.Fatalf("upsert guest: %v", err)
	}
	guests, err := db.ListProxmoxGuestsByNode(ctx, connID, "pve1")
	if err != nil || len(guests) != 1 {
		t.Fatalf("list guests: %v %+v", err, guests)
	}

	proxmoxService := proxmoxsvc.NewService(db, cfg, events.NewBus())
	h = NewWSHandler(db, cfg, NewNotificationHub(), events.NewBus(), func() string { return "" }, proxmoxService)
	return h, guests[0].ID
}

func newConsoleTestServer(t *testing.T, h *WSHandler) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/console/:guest_id", h.ProxmoxConsole)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func dialConsole(t *testing.T, srv *httptest.Server, guestID, jwtToken string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/console/" + guestID
	header := http.Header{}
	header.Set("Cookie", cookies.AccessTokenName+"="+jwtToken)
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial console: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestProxmoxConsole_RelaysBytesAndResize(t *testing.T) {
	h, guestID := setupConsoleTestHandler(t)
	srv := newConsoleTestServer(t, h)
	token := mintTestJWT(t, h.cfg.JWTSecret, "admin1", models.RoleAdmin)

	conn := dialConsole(t, srv, guestID, token)

	if err := conn.WriteMessage(websocket.TextMessage, []byte("ls\n")); err != nil {
		t.Fatalf("write keystrokes: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, out, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(out) != "echo:ls\n" {
		t.Errorf("echoed output = %q, want %q", out, "echo:ls\n")
	}

	resize, _ := json.Marshal(models.WSConsoleResize{Type: "resize", Cols: 120, Rows: 40})
	if err := conn.WriteMessage(websocket.TextMessage, resize); err != nil {
		t.Fatalf("write resize: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, out, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read resize ack: %v", err)
	}
	if string(out) != "resized:120:40:" {
		t.Errorf("resize forwarded = %q, want %q", out, "resized:120:40:")
	}

	logs, err := h.db.GetAuditLogs(context.Background(), 10, 0, database.AuditLogFilter{})
	if err != nil {
		t.Fatalf("get audit logs: %v", err)
	}
	found := false
	for _, l := range logs {
		if l.Action == "proxmox_console_open" && l.Username == "admin1" {
			found = true
		}
	}
	if !found {
		t.Error("expected a proxmox_console_open audit log row")
	}
}

func TestProxmoxConsole_NonAdminRejected(t *testing.T) {
	h, guestID := setupConsoleTestHandler(t)
	srv := newConsoleTestServer(t, h)
	token := mintTestJWT(t, h.cfg.JWTSecret, "viewer1", "viewer")

	conn := dialConsole(t, srv, guestID, token)

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var msg models.WSConsoleError
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read rejection: %v", err)
	}
	if msg.Type != "console_error" {
		t.Errorf("type = %q, want console_error", msg.Type)
	}

	// The connection should be closed right after; a further read must fail.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Error("expected connection to be closed after non-admin rejection")
	}
}

func TestProxmoxConsole_MissingGuestID(t *testing.T) {
	h, _ := setupConsoleTestHandler(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/console/*guest_id", func(c *gin.Context) {
		c.Params = gin.Params{{Key: "guest_id", Value: ""}}
		h.ProxmoxConsole(c)
	})
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/console/")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
