package rfb

import (
	"sync"

	"github.com/gorilla/websocket"
)

// WSStream adapts a WebSocket carrying an RFB byte stream to io.ReadWriter.
// RFB over WebSocket (as PVE's vncwebsocket and noVNC speak it) is a plain
// byte stream split into binary messages at arbitrary points: a message can
// hold half an RFB message or several of them, so the handshake must read it
// as a stream rather than message by message.
//
// Read keeps the unread tail of the current message; once the handshake is
// over, the relay drains that tail first (TakeBuffered) and then forwards
// whole messages without copying them through Read.
type WSStream struct {
	conn *websocket.Conn
	buf  []byte
	// writeMu serializes writes: gorilla/websocket allows at most one
	// concurrent writer per connection.
	writeMu sync.Mutex
}

// NewWSStream wraps conn.
func NewWSStream(conn *websocket.Conn) *WSStream {
	return &WSStream{conn: conn}
}

// Read implements io.Reader over consecutive WebSocket messages.
func (s *WSStream) Read(p []byte) (int, error) {
	for len(s.buf) == 0 {
		_, msg, err := s.conn.ReadMessage()
		if err != nil {
			return 0, err
		}
		s.buf = msg
	}
	n := copy(p, s.buf)
	s.buf = s.buf[n:]
	return n, nil
}

// Write sends p as one binary message.
func (s *WSStream) Write(p []byte) (int, error) {
	if err := s.WriteMessage(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// WriteMessage sends p as one binary message.
func (s *WSStream) WriteMessage(p []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.BinaryMessage, p)
}

// ReadMessage returns the bytes buffered by Read first, then the next
// message from the connection.
func (s *WSStream) ReadMessage() ([]byte, error) {
	if pending := s.TakeBuffered(); len(pending) > 0 {
		return pending, nil
	}
	_, msg, err := s.conn.ReadMessage()
	return msg, err
}

// TakeBuffered returns and clears the bytes Read received but did not
// consume yet.
func (s *WSStream) TakeBuffered() []byte {
	pending := s.buf
	s.buf = nil
	return pending
}

// Conn returns the underlying connection.
func (s *WSStream) Conn() *websocket.Conn { return s.conn }
