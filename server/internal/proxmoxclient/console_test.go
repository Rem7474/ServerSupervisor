package proxmoxclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestLogin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/access/ticket" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = r.ParseForm()
		if r.Form.Get("username") != "root@pam" || r.Form.Get("password") != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"data":null}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"ticket":"PVE:root@pam:XYZ==","CSRFPreventionToken":"abc","username":"root@pam"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)
	auth, err := c.login("root@pam", "secret")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if auth.Ticket != "PVE:root@pam:XYZ==" {
		t.Errorf("ticket = %q", auth.Ticket)
	}
	if auth.CSRFText != "abc" {
		t.Errorf("CSRFText = %q", auth.CSRFText)
	}
}

func TestLoginError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"data":null}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)
	if _, err := c.login("root@pam", "wrong"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCreateTermproxy(t *testing.T) {
	auth := pveLogin{Ticket: "PVE:root@pam:AUTH==", CSRFText: "csrf-tok"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nodes/pve1/lxc/101/termproxy" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		// Must authenticate as the same cookie identity that will present the
		// resulting vncticket to vncwebsocket — an API token here produces a
		// termproxy ticket for the token's user, not pveUsername, and PVE
		// rejects the mismatch as "invalid PVEVNC ticket".
		if cookie := r.Header.Get("Cookie"); !strings.Contains(cookie, "PVEAuthCookie=PVE:root@pam:AUTH==") {
			t.Errorf("missing/incorrect PVEAuthCookie: %q", cookie)
		}
		if got := r.Header.Get("CSRFPreventionToken"); got != "csrf-tok" {
			t.Errorf("CSRFPreventionToken = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("termproxy must not use API token auth, got Authorization = %q", got)
		}
		_, _ = w.Write([]byte(`{"data":{"ticket":"PVEVNC:XYZ==","port":"31000","user":"root@pam","upid":"UPID:..."}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)
	ticket, port, user, err := c.createTermproxy("pve1", 101, auth)
	if err != nil {
		t.Fatalf("createTermproxy: %v", err)
	}
	if ticket != "PVEVNC:XYZ==" || port != 31000 || user != "root@pam" {
		t.Errorf("got ticket=%q port=%d user=%q", ticket, port, user)
	}
}

func TestCreateTermproxyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":{"perm":"no permission"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)
	if _, _, _, err := c.createTermproxy("pve1", 101, pveLogin{Ticket: "x", CSRFText: "y"}); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// fakeTermproxyServer simulates PVE's login/termproxy REST calls and the
// vncwebsocket upgrade + login-handshake + "0:LEN:DATA" echo framing, so
// OpenLXCConsole and TermSession's wire protocol can be tested without a
// real PVE cluster.
func fakeTermproxyServer(t *testing.T) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	mux := http.NewServeMux()
	mux.HandleFunc("/access/ticket", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"ticket":"PVE:root@pam:AUTH==","CSRFPreventionToken":"csrf-tok"}}`))
	})
	mux.HandleFunc("/nodes/pve1/lxc/101/termproxy", func(w http.ResponseWriter, r *http.Request) {
		if cookie := r.Header.Get("Cookie"); !strings.Contains(cookie, "PVEAuthCookie=PVE:root@pam:AUTH==") {
			t.Errorf("termproxy missing/incorrect PVEAuthCookie: %q", cookie)
		}
		if got := r.Header.Get("CSRFPreventionToken"); got != "csrf-tok" {
			t.Errorf("termproxy CSRFPreventionToken = %q", got)
		}
		_, _ = w.Write([]byte(`{"data":{"ticket":"PVEVNC:TERM==","port":31000,"user":"root@pam","upid":"UPID:.."}}`))
	})
	mux.HandleFunc("/nodes/pve1/lxc/101/vncwebsocket", func(w http.ResponseWriter, r *http.Request) {
		if cookie := r.Header.Get("Cookie"); !strings.Contains(cookie, "PVEAuthCookie=PVE:root@pam:AUTH==") {
			t.Errorf("missing/incorrect PVEAuthCookie: %q", cookie)
		}
		if r.URL.Query().Get("vncticket") != "PVEVNC:TERM==" {
			t.Errorf("vncticket = %q", r.URL.Query().Get("vncticket"))
		}
		if proto := r.Header.Get("Sec-WebSocket-Protocol"); proto != "binary" {
			t.Errorf("Sec-WebSocket-Protocol = %q, want %q (PVE rejects the upgrade without it)", proto, "binary")
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()

		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if string(msg) != "root@pam:PVEVNC:TERM==\n" {
			t.Errorf("login handshake = %q", msg)
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, []byte("OK")); err != nil {
			return
		}

		// Echo loop: strip "0:LEN:" framing and bounce the payload back raw,
		// so the test can assert Write()/ReadMessage() round-trip correctly.
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			s := string(msg)
			if strings.HasPrefix(s, "0:") {
				rest := strings.SplitN(s[2:], ":", 2)
				if len(rest) == 2 {
					_ = conn.WriteMessage(websocket.TextMessage, []byte(rest[1]))
				}
			}
		}
	})

	return httptest.NewServer(mux)
}

func TestOpenLXCConsole(t *testing.T) {
	srv := fakeTermproxyServer(t)
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)
	session, err := c.OpenLXCConsole(context.Background(), "pve1", 101, "root@pam", "hunter2")
	if err != nil {
		t.Fatalf("OpenLXCConsole: %v", err)
	}
	defer func() { _ = session.Close() }()

	if err := session.Write([]byte("ls\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out, err := session.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if string(out) != "ls\n" {
		t.Errorf("echoed output = %q, want %q", out, "ls\n")
	}
}

func TestOpenLXCConsoleLoginFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"data":null}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)
	if _, err := c.OpenLXCConsole(context.Background(), "pve1", 101, "root@pam", "wrong"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestOpenLXCConsoleBadHandshake covers a PVE-side rejection of the
// vncwebsocket upgrade itself (e.g. the ticket was rejected, or a required
// header/subprotocol is missing) — this must surface the HTTP status and
// body PVE actually returned, not just gorilla's generic "bad handshake",
// so a misconfiguration is diagnosable from the error message alone.
// TestClientTestLogin covers the exported TestLogin wrapper used by
// Service.TestConnection to validate console credentials from the
// connection-settings UI (see service.go's TestConnection doc comment).
func TestClientTestLogin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("password") != "hunter2" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"data":null}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"ticket":"PVE:root@pam:X==","CSRFPreventionToken":"c"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)
	if err := c.TestLogin("root@pam", "hunter2"); err != nil {
		t.Fatalf("TestLogin with correct credentials: %v", err)
	}
	if err := c.TestLogin("root@pam", "wrong"); err == nil {
		t.Fatal("TestLogin with wrong credentials: expected error, got nil")
	}
}

// TestLoginNoTicketReturned covers a 200 OK response that nonetheless carries
// no ticket — treated as a login failure rather than a nil-ticket success.
func TestLoginNoTicketReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"ticket":""}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)
	if _, err := c.login("root@pam", "secret"); err == nil {
		t.Fatal("expected an error when PVE returns no ticket")
	}
}

func TestWsBaseURL(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		want    string
	}{
		{"https", "https://pve.example.com:8006", "wss://pve.example.com:8006"},
		{"http", "http://pve.example.com:8006", "ws://pve.example.com:8006"},
		{"unrecognized scheme returned as-is", "ftp://pve.example.com", "ftp://pve.example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := New(tc.baseURL, "user@pve!token", "secret-token", false)
			if got := c.wsBaseURL(); got != tc.want {
				t.Errorf("wsBaseURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestTermSessionResizeAndPing covers TermSession.Resize/Ping's wire framing
// directly against a raw WebSocket server, independent of the
// login/termproxy handshake exercised by TestOpenLXCConsole.
func TestTermSessionResizeAndPing(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	received := make(chan string, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		for i := 0; i < 2; i++ {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			received <- string(msg)
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	session := &TermSession{conn: conn}
	defer func() { _ = session.Close() }()

	if err := session.Resize(120, 40); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if err := session.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	if got := <-received; got != "1:120:40:" {
		t.Errorf("Resize frame = %q, want %q", got, "1:120:40:")
	}
	if got := <-received; got != "2" {
		t.Errorf("Ping frame = %q, want %q", got, "2")
	}
}

// TestOpenLXCConsoleTermproxyError covers a successful login followed by a
// PVE-side rejection of the termproxy creation call itself (e.g. missing
// VM.Console privilege on that node/guest).
func TestOpenLXCConsoleTermproxyError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/access/ticket", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"ticket":"PVE:root@pam:AUTH=="}}`))
	})
	mux.HandleFunc("/nodes/pve1/lxc/101/termproxy", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":{"perm":"no VM.Console"}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)
	if _, err := c.OpenLXCConsole(context.Background(), "pve1", 101, "root@pam", "hunter2"); err == nil {
		t.Fatal("expected an error when termproxy creation is rejected")
	}
}

// TestOpenLXCConsoleHandshakeRejected covers the vncwebsocket upgrade
// succeeding but the termproxy login handshake itself being rejected (PVE
// replies with something other than the literal "OK").
func TestOpenLXCConsoleHandshakeRejected(t *testing.T) {
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
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		_ = conn.WriteMessage(websocket.TextMessage, []byte("FAIL"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)
	_, err := c.OpenLXCConsole(context.Background(), "pve1", 101, "root@pam", "hunter2")
	if err == nil {
		t.Fatal("expected an error when the termproxy login handshake is rejected")
	}
	if !strings.Contains(err.Error(), "termproxy login rejected") {
		t.Errorf("error = %v, want it to mention the rejected handshake", err)
	}
}

func TestOpenLXCConsoleBadHandshake(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/access/ticket", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"ticket":"PVE:root@pam:AUTH=="}}`))
	})
	mux.HandleFunc("/nodes/pve1/lxc/101/termproxy", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"ticket":"PVEVNC:TERM==","port":31000,"user":"root@pam","upid":"UPID:.."}}`))
	})
	mux.HandleFunc("/nodes/pve1/lxc/101/vncwebsocket", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("401 Permission denied - invalid ticket"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)
	_, err := c.OpenLXCConsole(context.Background(), "pve1", 101, "root@pam", "hunter2")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 401") || !strings.Contains(err.Error(), "Permission denied") {
		t.Errorf("error should surface PVE's actual status/body, got: %v", err)
	}
}
