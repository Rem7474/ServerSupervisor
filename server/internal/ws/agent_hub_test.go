package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// newTestAgentConn spins up a minimal HTTP server that upgrades one
// connection to WebSocket, returning both ends: the server-side *Conn (what
// AgentHub would register) and the client-side *Conn (to observe what the
// hub writes, as the agent would).
func newTestAgentConn(t *testing.T) (serverConn, clientConn *websocket.Conn) {
	t.Helper()
	upgrader := websocket.Upgrader{}
	serverConnCh := make(chan *websocket.Conn, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}
		serverConnCh <- conn
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	client, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	server := <-serverConnCh
	t.Cleanup(func() { _ = server.Close() })

	return server, client
}

func TestAgentHub_RegisterConnectedUnregister(t *testing.T) {
	hub := NewAgentHub()
	if hub.Connected("host-1") {
		t.Fatal("expected host-1 to be unconnected before Register")
	}

	server, _ := newTestAgentConn(t)
	hub.Register("host-1", server)
	if !hub.Connected("host-1") {
		t.Error("expected host-1 to be connected after Register")
	}

	hub.Unregister("host-1", server)
	if hub.Connected("host-1") {
		t.Error("expected host-1 to be unconnected after Unregister")
	}
}

func TestAgentHub_RegisterReplacesPriorConnectionForSameHost(t *testing.T) {
	hub := NewAgentHub()
	server1, client1 := newTestAgentConn(t)
	server2, _ := newTestAgentConn(t)

	hub.Register("host-1", server1)
	hub.Register("host-1", server2) // simulates the agent reconnecting

	_ = client1.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := client1.ReadMessage(); err == nil {
		t.Error("expected the replaced connection to be closed")
	}

	if !hub.Connected("host-1") {
		t.Error("expected host-1 to still be connected via the new connection")
	}
}

func TestAgentHub_UnregisterIsNoopForAlreadyReplacedConnection(t *testing.T) {
	hub := NewAgentHub()
	server1, _ := newTestAgentConn(t)
	server2, _ := newTestAgentConn(t)

	hub.Register("host-1", server1)
	hub.Register("host-1", server2)

	// Unregistering the OLD (already-replaced) connection must not evict the new one.
	hub.Unregister("host-1", server1)
	if !hub.Connected("host-1") {
		t.Error("expected host-1 to remain connected: Unregister(old) should be a no-op after replacement")
	}
}

func TestAgentHub_Notify(t *testing.T) {
	hub := NewAgentHub()

	t.Run("no connection returns false", func(t *testing.T) {
		if hub.Notify("nobody-home") {
			t.Error("expected Notify to return false with no registered connection")
		}
	})

	t.Run("delivers a poll_now message to the connected agent", func(t *testing.T) {
		server, client := newTestAgentConn(t)
		hub.Register("host-1", server)

		if !hub.Notify("host-1") {
			t.Fatal("expected Notify to return true with a live connection")
		}

		_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
		var msg struct {
			Type string `json:"type"`
		}
		if err := client.ReadJSON(&msg); err != nil {
			t.Fatalf("failed to read pushed message: %v", err)
		}
		if msg.Type != "poll_now" {
			t.Errorf("got type %q, want %q", msg.Type, "poll_now")
		}
	})
}
