package rfb

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// wsPair returns a client-side connection to a server whose handler is
// given the other end.
func wsPair(t *testing.T, serve func(*websocket.Conn)) *websocket.Conn {
	t.Helper()
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		serve(conn)
	}))
	t.Cleanup(srv.Close)
	conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// RFB over WebSocket is a byte stream cut at arbitrary points: a read must
// span message boundaries, and bytes read ahead must come back first from
// ReadMessage once the handshake hands over to the relay.
func TestWSStream_ReadSpansMessagesAndKeepsTail(t *testing.T) {
	conn := wsPair(t, func(c *websocket.Conn) {
		_ = c.WriteMessage(websocket.BinaryMessage, []byte("RFB 00"))
		_ = c.WriteMessage(websocket.BinaryMessage, []byte("3.008\nTAIL"))
		_ = c.WriteMessage(websocket.BinaryMessage, []byte("NEXT"))
		_, _, _ = c.ReadMessage() // keep the connection open until the client is done
	})
	s := NewWSStream(conn)

	version := make([]byte, 12)
	if _, err := io.ReadFull(s, version); err != nil || string(version) != "RFB 003.008\n" {
		t.Fatalf("version = %q, %v", version, err)
	}
	msg, err := s.ReadMessage()
	if err != nil || string(msg) != "TAIL" {
		t.Fatalf("first relayed message = %q, %v; want the read-ahead tail", msg, err)
	}
	msg, err = s.ReadMessage()
	if err != nil || string(msg) != "NEXT" {
		t.Fatalf("second relayed message = %q, %v", msg, err)
	}
	if len(s.TakeBuffered()) != 0 {
		t.Error("buffer should be empty after the tail was relayed")
	}
}

func TestWSStream_WriteSendsBinaryMessage(t *testing.T) {
	got := make(chan int, 1)
	conn := wsPair(t, func(c *websocket.Conn) {
		mt, msg, err := c.ReadMessage()
		if err == nil && string(msg) == "hello" {
			got <- mt
		}
		close(got)
	})
	s := NewWSStream(conn)
	if n, err := s.Write([]byte("hello")); err != nil || n != 5 {
		t.Fatalf("Write = %d, %v", n, err)
	}
	select {
	case mt := <-got:
		if mt != websocket.BinaryMessage {
			t.Errorf("message type = %d, want binary", mt)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not receive the message")
	}
	if s.Conn() != conn {
		t.Error("Conn should return the wrapped connection")
	}
}

func TestWSStream_ErrorsPropagate(t *testing.T) {
	conn := wsPair(t, func(c *websocket.Conn) {})
	s := NewWSStream(conn)
	if _, err := s.Read(make([]byte, 1)); err == nil {
		t.Error("Read should fail once the peer closed")
	}
	if _, err := s.ReadMessage(); err == nil {
		t.Error("ReadMessage should fail once the peer closed")
	}
	_ = conn.Close()
	if _, err := s.Write([]byte("x")); err == nil {
		t.Error("Write should fail on a closed connection")
	}
}
