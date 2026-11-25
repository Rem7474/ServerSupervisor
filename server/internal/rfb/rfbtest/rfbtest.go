// Package rfbtest provides a fake PVE vncwebsocket endpoint for tests: it
// upgrades the request, plays the server side of an RFB 3.8 handshake with
// VNC Authentication, then echoes every message back prefixed with "echo:".
package rfbtest

import (
	"bytes"
	"crypto/des" //nolint:gosec // VNC Authentication is DES by specification (RFC 6143 §7.2.2).
	"encoding/binary"
	"io"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/serversupervisor/server/internal/rfb"
)

// ServerInit is what the fake sends after receiving ClientInit: a minimal
// 1x1 framebuffer description with an empty name.
var ServerInit = []byte{0, 1, 0, 1, 32, 24, 0, 1, 0, 255, 0, 255, 0, 255, 16, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

// Handler returns an http.HandlerFunc emulating PVE's vncwebsocket for a VM
// whose one-time VNC password is password. When wantTicket is not empty the
// request must carry it as vncticket.
func Handler(password, wantTicket string) http.HandlerFunc {
	upgrader := websocket.Upgrader{
		CheckOrigin:  func(*http.Request) bool { return true },
		Subprotocols: []string{"binary"},
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if wantTicket != "" && r.URL.Query().Get("vncticket") != wantTicket {
			http.Error(w, "invalid PVEVNC ticket", http.StatusUnauthorized)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		s := rfb.NewWSStream(conn)
		if !serverHandshake(s, password) {
			return
		}
		clientInit := make([]byte, 1)
		if _, err := io.ReadFull(s, clientInit); err != nil {
			return
		}
		if err := s.WriteMessage(ServerInit); err != nil {
			return
		}
		for {
			msg, err := s.ReadMessage()
			if err != nil {
				return
			}
			if err := s.WriteMessage(append([]byte("echo:"), msg...)); err != nil {
				return
			}
		}
	}
}

func serverHandshake(s *rfb.WSStream, password string) bool {
	// Split the version over two messages on purpose: the real stream gives no
	// guarantee that RFB messages and WebSocket messages line up.
	if s.WriteMessage([]byte("RFB 003")) != nil || s.WriteMessage([]byte(".008\n")) != nil {
		return false
	}
	version := make([]byte, 12)
	if _, err := io.ReadFull(s, version); err != nil {
		return false
	}
	if s.WriteMessage([]byte{1, rfb.SecurityVNCAuth}) != nil {
		return false
	}
	chosen := make([]byte, 1)
	if _, err := io.ReadFull(s, chosen); err != nil || chosen[0] != rfb.SecurityVNCAuth {
		return false
	}
	challenge := bytes.Repeat([]byte{0x5a}, 16)
	if s.WriteMessage(challenge) != nil {
		return false
	}
	resp := make([]byte, 16)
	if _, err := io.ReadFull(s, resp); err != nil {
		return false
	}
	if !bytes.Equal(resp, expectedResponse(challenge, password)) {
		reason := []byte("authentication failed")
		msg := binary.BigEndian.AppendUint32(binary.BigEndian.AppendUint32(nil, 1), uint32(len(reason)))
		_ = s.WriteMessage(append(msg, reason...))
		return false
	}
	return s.WriteMessage([]byte{0, 0, 0, 0}) == nil
}

// expectedResponse is an independent implementation of the VNC challenge
// response (bit-reversed DES key), so the fake doesn't just trust the code
// under test.
func expectedResponse(challenge []byte, password string) []byte {
	key := make([]byte, 8)
	copy(key, password)
	for i, b := range key {
		var r byte
		for j := 0; j < 8; j++ {
			if b&(1<<j) != 0 {
				r |= 1 << (7 - j)
			}
		}
		key[i] = r
	}
	block, err := des.NewCipher(key) // NOSONAR — test fake of a protocol-mandated cipher
	if err != nil {
		return nil
	}
	out := make([]byte, 16)
	block.Encrypt(out[:8], challenge[:8]) // NOSONAR — test fake of a protocol-mandated cipher (RFC 6143 §7.2.2)
	block.Encrypt(out[8:], challenge[8:]) // NOSONAR — test fake of a protocol-mandated cipher (RFC 6143 §7.2.2)
	return out
}
