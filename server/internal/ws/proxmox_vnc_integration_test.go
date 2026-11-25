package ws

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/cookies"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/events"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/rfb"
	"github.com/serversupervisor/server/internal/rfb/rfbtest"
	proxmoxsvc "github.com/serversupervisor/server/internal/services/proxmox"
	"github.com/serversupervisor/server/internal/testutil"
)

// setupVNCTestHandler wires a Postgres-backed WSHandler with one QEMU VM
// (vmid 100) and one LXC container (vmid 101) on a Proxmox connection that
// points at a fake PVE (login + vncproxy + vncwebsocket speaking RFB with VNC
// Authentication, see rfbtest).
func setupVNCTestHandler(t *testing.T) (h *WSHandler, vmID, ctID string) {
	t.Helper()
	db, cfg := testutil.NewPostgresDBWithConfig(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/access/ticket", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"ticket":"test-pve-ticket","CSRFPreventionToken":"csrf"}}`))
	})
	mux.HandleFunc("/nodes/pve1/qemu/100/vncproxy", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"ticket":"vncpw:PVEVNC:Q==","port":5900,"password":"vncpw"}}`))
	})
	mux.HandleFunc("/nodes/pve1/qemu/100/vncwebsocket", rfbtest.Handler("vncpw", "vncpw:PVEVNC:Q=="))
	pve := httptest.NewServer(mux)
	t.Cleanup(pve.Close)

	ctx := context.Background()
	connID, err := db.CreateProxmoxConnection(ctx, models.ProxmoxConnectionRequest{
		Name: "test", APIURL: pve.URL, TokenID: "user@pve!token", TokenSecret: "secret",
		Enabled: true, PollIntervalSec: 60,
		PVEUsername: "root@pam", PVEPassword: "hunter2",
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	if err := db.UpsertProxmoxGuest(ctx, connID, "pve1", "vm", 100, "win11", "running", 2, 0, 2048, 0, 0, 0, 0, ""); err != nil {
		t.Fatalf("upsert vm: %v", err)
	}
	if err := db.UpsertProxmoxGuest(ctx, connID, "pve1", "lxc", 101, "web1", "running", 1, 0, 512, 0, 0, 0, 0, ""); err != nil {
		t.Fatalf("upsert ct: %v", err)
	}
	guests, err := db.ListProxmoxGuestsByNode(ctx, connID, "pve1")
	if err != nil || len(guests) != 2 {
		t.Fatalf("list guests: %v %+v", err, guests)
	}
	for _, g := range guests {
		if g.GuestType == "vm" {
			vmID = g.ID
		} else {
			ctID = g.ID
		}
	}

	proxmoxService := proxmoxsvc.NewService(db, cfg, events.NewBus())
	h = NewWSHandler(db, cfg, NewNotificationHub(), events.NewBus(), func() string { return "" }, proxmoxService)
	return h, vmID, ctID
}

func dialVNC(t *testing.T, h *WSHandler, guestID, jwtToken string) *websocket.Conn {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/vnc/:guest_id", h.ProxmoxVNC)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	header := http.Header{}
	header.Set("Cookie", cookies.AccessTokenName+"="+jwtToken)
	conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/vnc/"+guestID, header)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial vnc: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// expectVNCClose reads until the server closes and checks the close code and
// reason the frontend relies on to show a translated error.
func expectVNCClose(t *testing.T, conn *websocket.Conn, wantCode int, wantReason string) {
	t.Helper()
	_, _, err := conn.ReadMessage()
	var ce *websocket.CloseError
	if !errors.As(err, &ce) {
		t.Fatalf("expected a close frame, got %v", err)
	}
	if ce.Code != wantCode || ce.Text != wantReason {
		t.Errorf("close = %d %q, want %d %q", ce.Code, ce.Text, wantCode, wantReason)
	}
}

func vncAudits(t *testing.T, h *WSHandler) []models.AuditLog {
	t.Helper()
	logs, err := h.db.GetAuditLogs(context.Background(), 50, 0, database.AuditLogFilter{})
	if err != nil {
		t.Fatalf("get audit logs: %v", err)
	}
	var out []models.AuditLog
	for _, l := range logs {
		if l.Action == "proxmox_vnc_open" {
			out = append(out, l)
		}
	}
	return out
}

func TestProxmoxVNC_HandshakeRelayAndAudit(t *testing.T) {
	h, vmID, _ := setupVNCTestHandler(t)
	conn := dialVNC(t, h, vmID, mintTestJWT(t, h.cfg.JWTSecret, "admin1", models.RoleAdmin))

	// The browser leg is offered "None" only: no password ever reaches it.
	browser := rfb.NewWSStream(conn)
	if err := rfb.ClientHandshake(browser, ""); err != nil {
		t.Fatalf("browser-side handshake: %v", err)
	}
	if _, err := browser.Write([]byte{1}); err != nil { // ClientInit, shared
		t.Fatalf("write ClientInit: %v", err)
	}
	init := make([]byte, len(rfbtest.ServerInit))
	if _, err := io.ReadFull(browser, init); err != nil || !bytes.Equal(init, rfbtest.ServerInit) {
		t.Fatalf("ServerInit = %v, %v", init, err)
	}
	if _, err := browser.Write([]byte("keys")); err != nil {
		t.Fatalf("write: %v", err)
	}
	msgType, out, err := conn.ReadMessage()
	if err != nil || string(out) != "echo:keys" {
		t.Fatalf("relay = %q, %v", out, err)
	}
	if msgType != websocket.BinaryMessage {
		t.Errorf("relayed frame type = %d, want binary", msgType)
	}

	audits := vncAudits(t, h)
	if len(audits) != 1 || audits[0].Username != "admin1" || audits[0].Status != "success" ||
		!strings.Contains(audits[0].Details, "vmid=100") {
		t.Errorf("audit rows = %+v, want one success row for admin1 on vmid 100", audits)
	}
}

func TestProxmoxVNC_NonAdminRejected(t *testing.T) {
	h, vmID, _ := setupVNCTestHandler(t)
	conn := dialVNC(t, h, vmID, mintTestJWT(t, h.cfg.JWTSecret, "viewer1", "viewer"))
	expectVNCClose(t, conn, vncCloseForbidden, apperr.CodeProxmoxConsoleAdminOnly)
	if audits := vncAudits(t, h); len(audits) != 0 {
		t.Errorf("a rejected role must not reach PVE nor audit an open, got %+v", audits)
	}
}

func TestProxmoxVNC_UnknownGuest(t *testing.T) {
	h, _, _ := setupVNCTestHandler(t)
	conn := dialVNC(t, h, "00000000-0000-0000-0000-000000000000", mintTestJWT(t, h.cfg.JWTSecret, "admin1", models.RoleAdmin))
	expectVNCClose(t, conn, vncCloseNotFound, apperr.CodeProxmoxGuestNotFound)
}

func TestProxmoxVNC_NonQemuGuestRejectedAndAudited(t *testing.T) {
	h, _, ctID := setupVNCTestHandler(t)
	conn := dialVNC(t, h, ctID, mintTestJWT(t, h.cfg.JWTSecret, "admin1", models.RoleAdmin))
	expectVNCClose(t, conn, vncCloseBadRequest, apperr.CodeProxmoxVNCQemuOnly)

	audits := vncAudits(t, h)
	if len(audits) != 1 || audits[0].Status != "failure" || !strings.Contains(audits[0].Details, "vmid=101") {
		t.Errorf("audit rows = %+v, want one failure row on vmid 101", audits)
	}
}

func TestProxmoxVNC_MissingGuestID(t *testing.T) {
	h, _, _ := setupVNCTestHandler(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/vnc/", nil)
	h.ProxmoxVNC(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
