package proxmoxclient

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/serversupervisor/server/internal/rfb/rfbtest"
)

// The vncproxy ticket PVE returns when it generates a VNC password is
// "<password>:<PVEVNC ticket>" (qemu-server's API2/Qemu.pm); it is passed back
// to vncwebsocket verbatim.
const (
	fakeVNCPassword = "vncPw123"
	fakeVNCTicket   = fakeVNCPassword + ":PVEVNC:QEMU=="
)

// fakeVNCProxyServer simulates PVE's login + vncproxy REST calls and the
// vncwebsocket endpoint (RFB server with VNC Authentication, see rfbtest).
func fakeVNCProxyServer(t *testing.T, vncproxyBody string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/access/ticket", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"ticket":"test-pve-ticket","CSRFPreventionToken":"csrf-tok"}}`))
	})
	mux.HandleFunc("/nodes/pve1/qemu/100/vncproxy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("vncproxy method = %s", r.Method)
		}
		if cookie := r.Header.Get("Cookie"); !strings.Contains(cookie, "PVEAuthCookie=test-pve-ticket") {
			t.Errorf("vncproxy missing/incorrect PVEAuthCookie: %q", cookie)
		}
		if got := r.Header.Get("CSRFPreventionToken"); got != "csrf-tok" {
			t.Errorf("vncproxy CSRFPreventionToken = %q", got)
		}
		if err := r.ParseForm(); err != nil || r.PostForm.Get("websocket") != "1" || r.PostForm.Get("generate-password") != "1" {
			t.Errorf("vncproxy form = %v, want websocket=1 and generate-password=1", r.PostForm)
		}
		_, _ = w.Write([]byte(vncproxyBody))
	})
	fake := rfbtest.Handler(fakeVNCPassword, fakeVNCTicket)
	mux.HandleFunc("/nodes/pve1/qemu/100/vncwebsocket", func(w http.ResponseWriter, r *http.Request) {
		if cookie := r.Header.Get("Cookie"); !strings.Contains(cookie, "PVEAuthCookie=test-pve-ticket") {
			t.Errorf("vncwebsocket missing/incorrect PVEAuthCookie: %q", cookie)
		}
		if r.URL.Query().Get("port") != "5901" {
			t.Errorf("vncwebsocket port = %q", r.URL.Query().Get("port"))
		}
		fake(w, r)
	})
	return httptest.NewServer(mux)
}

const okVNCProxyBody = `{"data":{"ticket":"` + fakeVNCTicket + `","port":"5901","password":"` + fakeVNCPassword + `","user":"root@pam","cert":"-","upid":"UPID:.."}}`

func TestOpenQEMUVNC_HandshakeThenRelay(t *testing.T) {
	srv := fakeVNCProxyServer(t, okVNCProxyBody)
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)
	session, err := c.OpenQEMUVNC(context.Background(), "pve1", 100, "root@pam", "hunter2")
	if err != nil {
		t.Fatalf("OpenQEMUVNC: %v", err)
	}
	defer func() { _ = session.Close() }()

	// Positioned before ClientInit: sending it must yield ServerInit.
	if err := session.Write([]byte{1}); err != nil {
		t.Fatalf("write ClientInit: %v", err)
	}
	init, err := session.ReadMessage()
	if err != nil || !bytes.Equal(init, rfbtest.ServerInit) {
		t.Fatalf("ServerInit = %v, %v", init, err)
	}
	if err := session.Write([]byte("key")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if out, err := session.ReadMessage(); err != nil || string(out) != "echo:key" {
		t.Fatalf("relay = %q, %v", out, err)
	}
}

func TestOpenQEMUVNC_WrongPasswordFailsHandshake(t *testing.T) {
	body := strings.Replace(okVNCProxyBody, `"password":"`+fakeVNCPassword+`"`, `"password":"other"`, 1)
	srv := fakeVNCProxyServer(t, body)
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)
	_, err := c.OpenQEMUVNC(context.Background(), "pve1", 100, "root@pam", "hunter2")
	if err == nil || !strings.Contains(err.Error(), "vnc handshake") {
		t.Fatalf("err = %v, want a vnc handshake failure", err)
	}
}

func TestOpenQEMUVNC_LoginFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)
	_, err := c.OpenQEMUVNC(context.Background(), "pve1", 100, "root@pam", "wrong")
	if err == nil || !strings.Contains(err.Error(), "pve login") {
		t.Fatalf("err = %v, want a pve login failure", err)
	}
}

func TestCreateVNCProxy_Errors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"permission denied", http.StatusForbidden, `{"data":null,"message":"Permission check failed (VM.Console)` + strings.Repeat("x", 400) + `"}`, "HTTP 403"},
		{"bad json", http.StatusOK, `not json`, "parse response"},
		{"no ticket", http.StatusOK, `{"data":{"port":5901}}`, "no ticket or port"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			c := New(srv.URL, "user@pve!token", "secret-token", false)
			_, err := c.createVNCProxy("pve1", 100, pveLogin{Ticket: "t", CSRFText: "c"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
			if len(err.Error()) > 400 {
				t.Errorf("error should truncate PVE's body, got %d bytes", len(err.Error()))
			}
		})
	}
}

func TestOpenQEMUVNC_VNCProxyFailure(t *testing.T) {
	srv := fakeVNCProxyServer(t, `{"data":{}}`)
	defer srv.Close()
	c := New(srv.URL, "user@pve!token", "secret-token", false)
	_, err := c.OpenQEMUVNC(context.Background(), "pve1", 100, "root@pam", "hunter2")
	if err == nil || !strings.Contains(err.Error(), "create vncproxy") {
		t.Fatalf("err = %v, want a vncproxy failure", err)
	}
}

// A rejected upgrade (e.g. an invalid ticket) must surface PVE's status and
// body, not only gorilla's generic "bad handshake".
func TestOpenQEMUVNC_UpgradeRejected(t *testing.T) {
	body := strings.Replace(okVNCProxyBody, fakeVNCTicket, fakeVNCPassword+":PVEVNC:STALE==", 1)
	srv := fakeVNCProxyServer(t, body)
	defer srv.Close()
	c := New(srv.URL, "user@pve!token", "secret-token", false)
	_, err := c.OpenQEMUVNC(context.Background(), "pve1", 100, "root@pam", "hunter2")
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") || !strings.Contains(err.Error(), "invalid PVEVNC ticket") {
		t.Fatalf("err = %v, want PVE's 401 and body", err)
	}
}

func TestOpenQEMUVNC_DialFailure(t *testing.T) {
	// vncproxy answers, but the websocket endpoint is not there at all.
	mux := http.NewServeMux()
	mux.HandleFunc("/access/ticket", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"ticket":"test-pve-ticket"}}`))
	})
	mux.HandleFunc("/nodes/pve1/qemu/100/vncproxy", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(okVNCProxyBody))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(srv.URL, "user@pve!token", "secret-token", false)
	if _, err := c.OpenQEMUVNC(context.Background(), "pve1", 100, "root@pam", "hunter2"); err == nil || !strings.Contains(err.Error(), "dial vnc websocket") {
		t.Fatalf("err = %v, want a dial failure", err)
	}
}

func TestOpenQEMUVNC_DialUnreachable(t *testing.T) {
	// Login and vncproxy succeed (they don't take ctx), the websocket dial
	// runs on an already-cancelled context: it fails with no HTTP response.
	srv := fakeVNCProxyServer(t, okVNCProxyBody)
	defer srv.Close()
	c := New(srv.URL, "user@pve!token", "secret-token", false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.OpenQEMUVNC(ctx, "pve1", 100, "root@pam", "hunter2"); err == nil || !strings.Contains(err.Error(), "dial vnc websocket") {
		t.Fatalf("err = %v, want a dial failure", err)
	}
}

// A PVE that upgrades the socket but never speaks RFB must not hang the
// caller: the handshake is bounded by handshakeTimeout.
func TestOpenQEMUVNC_HandshakeTimeout(t *testing.T) {
	old := handshakeTimeout
	handshakeTimeout = 200 * time.Millisecond
	defer func() { handshakeTimeout = old }()

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	mux := http.NewServeMux()
	mux.HandleFunc("/access/ticket", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"ticket":"test-pve-ticket"}}`))
	})
	mux.HandleFunc("/nodes/pve1/qemu/100/vncproxy", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(okVNCProxyBody))
	})
	mux.HandleFunc("/nodes/pve1/qemu/100/vncwebsocket", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _, _ = conn.ReadMessage() // block until the client gives up
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)
	start := time.Now()
	_, err := c.OpenQEMUVNC(context.Background(), "pve1", 100, "root@pam", "hunter2")
	if err == nil || !strings.Contains(err.Error(), "vnc handshake") {
		t.Fatalf("err = %v, want a handshake timeout", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %v, want it bounded by handshakeTimeout", elapsed)
	}
}
