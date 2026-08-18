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
	ticket, err := c.login("root@pam", "secret")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if ticket != "PVE:root@pam:XYZ==" {
		t.Errorf("ticket = %q", ticket)
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nodes/pve1/lxc/101/termproxy" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "PVEAPIToken=user@pve!token=secret-token" {
			t.Errorf("Authorization = %q", got)
		}
		_, _ = w.Write([]byte(`{"data":{"ticket":"PVEVNC:XYZ==","port":"31000","user":"root@pam","upid":"UPID:..."}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "user@pve!token", "secret-token", false)
	ticket, port, user, err := c.createTermproxy("pve1", 101)
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
	if _, _, _, err := c.createTermproxy("pve1", 101); err == nil {
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
		_, _ = w.Write([]byte(`{"data":{"ticket":"PVE:root@pam:AUTH=="}}`))
	})
	mux.HandleFunc("/nodes/pve1/lxc/101/termproxy", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"ticket":"PVEVNC:TERM==","port":31000,"user":"root@pam","upid":"UPID:.."}}`))
	})
	mux.HandleFunc("/nodes/pve1/lxc/101/vncwebsocket", func(w http.ResponseWriter, r *http.Request) {
		if cookie := r.Header.Get("Cookie"); !strings.Contains(cookie, "PVEAuthCookie=PVE:root@pam:AUTH==") {
			t.Errorf("missing/incorrect PVEAuthCookie: %q", cookie)
		}
		if r.URL.Query().Get("vncticket") != "PVEVNC:TERM==" {
			t.Errorf("vncticket = %q", r.URL.Query().Get("vncticket"))
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
