package rfbtest

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/serversupervisor/server/internal/rfb"
)

func dial(t *testing.T, srv *httptest.Server, ticket string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	d := websocket.Dialer{Subprotocols: []string{"binary"}}
	conn, resp, err := d.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/?vncticket="+ticket, nil)
	if conn != nil {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		t.Cleanup(func() { _ = conn.Close() })
	}
	return conn, resp, err
}

// The fake must agree with the real client: a full handshake with the right
// password reaches ServerInit and the echo relay.
func TestHandler_HandshakeThenEcho(t *testing.T) {
	srv := httptest.NewServer(Handler("pw123456", "T1"))
	defer srv.Close()
	conn, resp, err := dial(t, srv, "T1")
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	s := rfb.NewWSStream(conn)
	if err := rfb.ClientHandshake(s, "pw123456"); err != nil {
		t.Fatalf("handshake: %v", err)
	}
	if _, err := s.Write([]byte{1}); err != nil {
		t.Fatalf("client init: %v", err)
	}
	init := make([]byte, len(ServerInit))
	if _, err := io.ReadFull(s, init); err != nil || !bytes.Equal(init, ServerInit) {
		t.Fatalf("server init = %v, %v", init, err)
	}
	_, _ = s.Write([]byte("key"))
	if msg, err := s.ReadMessage(); err != nil || string(msg) != "echo:key" {
		t.Fatalf("echo = %q, %v", msg, err)
	}
}

func TestHandler_WrongPassword(t *testing.T) {
	srv := httptest.NewServer(Handler("pw123456", ""))
	defer srv.Close()
	conn, resp, err := dial(t, srv, "")
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if err := rfb.ClientHandshake(rfb.NewWSStream(conn), "nope"); err == nil {
		t.Fatal("expected the fake to reject a wrong password")
	}
}

func TestHandler_WrongTicket(t *testing.T) {
	srv := httptest.NewServer(Handler("pw", "T1"))
	defer srv.Close()
	_, resp, err := dial(t, srv, "other")
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected a 401 for a wrong ticket, got %v", err)
	}
}
