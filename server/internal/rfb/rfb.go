// Package rfb implements the small part of the RFB (VNC) protocol the
// Proxmox QEMU console proxy needs: the connection-opening handshake, on both
// legs of the proxy. Everything after the handshake (ClientInit/ServerInit,
// framebuffer updates, input events) is relayed byte for byte and never
// parsed here.
//
// The proxy terminates VNC authentication itself: it answers PVE's
// VNC-Authentication challenge with the one-time password PVE's vncproxy
// returned, and offers the browser the "None" security type only. The
// password therefore never leaves the server, and the browser leg is
// protected by the session cookie and the admin role check instead
// (see ws.WSHandler.ProxmoxVNC).
//
// Reference: RFC 6143 (The Remote Framebuffer Protocol), sections 7.1 and 7.2.
package rfb

import (
	"bytes"
	"crypto/des" //nolint:gosec // DES is mandated by RFB's VNC Authentication (RFC 6143 §7.2.2), not chosen.
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Security types (RFC 6143 §7.2 / IANA registry).
const (
	SecurityNone    byte = 1
	SecurityVNCAuth byte = 2
)

const (
	versionLen = 12 // "RFB xxx.yyy\n"
	// maxReasonLen caps a failure-reason string read from the remote side, so
	// a misbehaving server cannot make the proxy allocate an arbitrary buffer.
	maxReasonLen = 4096
)

// Version38 is the protocol version both legs of the proxy speak.
var Version38 = []byte("RFB 003.008\n")

// ErrAuthFailed is returned when the remote server rejects the VNC password.
var ErrAuthFailed = errors.New("vnc authentication failed")

// parseVersion returns the minor version of a ProtocolVersion message
// ("RFB 003.008\n" -> 8). Only major version 3 exists.
func parseVersion(v []byte) (int, error) {
	var major, minor int
	if len(v) != versionLen || v[versionLen-1] != '\n' {
		return 0, fmt.Errorf("malformed protocol version %q", v)
	}
	if _, err := fmt.Sscanf(string(v), "RFB %03d.%03d\n", &major, &minor); err != nil || major != 3 {
		return 0, fmt.Errorf("unsupported protocol version %q", v)
	}
	return minor, nil
}

// readReason reads a U32-length-prefixed failure reason string.
func readReason(r io.Reader) string {
	var n uint32
	if err := binary.Read(r, binary.BigEndian, &n); err != nil {
		return ""
	}
	if n > maxReasonLen {
		n = maxReasonLen
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return ""
	}
	return string(buf)
}

// ClientHandshake performs the client side of the RFB handshake against a VNC
// server (here: PVE's QEMU VNC socket, reached through vncwebsocket), up to
// and including the SecurityResult. password is used only if the server
// requires VNC Authentication. On return the connection is positioned right
// before the ClientInit message, which the caller relays from the browser.
//
// Protocol 3.7 and 3.8 are supported (QEMU speaks 3.8); 3.3, whose security
// negotiation has a different shape, is rejected.
func ClientHandshake(rw io.ReadWriter, password string) error {
	serverVersion := make([]byte, versionLen)
	if _, err := io.ReadFull(rw, serverVersion); err != nil {
		return fmt.Errorf("read server version: %w", err)
	}
	minor, err := parseVersion(serverVersion)
	if err != nil {
		return err
	}
	if minor < 7 {
		return fmt.Errorf("unsupported protocol version %q", serverVersion)
	}
	if minor > 8 {
		minor = 8
	}
	if _, err := fmt.Fprintf(rw, "RFB 003.%03d\n", minor); err != nil {
		return fmt.Errorf("write client version: %w", err)
	}

	var count [1]byte
	if _, err := io.ReadFull(rw, count[:]); err != nil {
		return fmt.Errorf("read security types: %w", err)
	}
	if count[0] == 0 {
		return fmt.Errorf("server refused the connection: %s", readReason(rw))
	}
	types := make([]byte, count[0])
	if _, err := io.ReadFull(rw, types); err != nil {
		return fmt.Errorf("read security types: %w", err)
	}

	// Prefer None when offered (no secret involved), otherwise VNC
	// Authentication. Anything else (VeNCrypt, TLS, …) is not implemented.
	var chosen byte
	switch {
	case bytes.IndexByte(types, SecurityNone) >= 0:
		chosen = SecurityNone
	case bytes.IndexByte(types, SecurityVNCAuth) >= 0:
		chosen = SecurityVNCAuth
	default:
		return fmt.Errorf("no supported security type offered (got %v)", types)
	}
	if _, err := rw.Write([]byte{chosen}); err != nil {
		return fmt.Errorf("write security type: %w", err)
	}

	if chosen == SecurityVNCAuth {
		if password == "" {
			return errors.New("server requires VNC authentication but no password was provided")
		}
		challenge := make([]byte, 16)
		if _, err := io.ReadFull(rw, challenge); err != nil {
			return fmt.Errorf("read auth challenge: %w", err)
		}
		response, err := vncAuthResponse(challenge, password)
		if err != nil {
			return err
		}
		if _, err := rw.Write(response); err != nil {
			return fmt.Errorf("write auth response: %w", err)
		}
	} else if minor == 7 {
		// RFB 3.7 sends no SecurityResult for the None type.
		return nil
	}

	var result uint32
	if err := binary.Read(rw, binary.BigEndian, &result); err != nil {
		return fmt.Errorf("read security result: %w", err)
	}
	if result != 0 {
		if minor >= 8 {
			if reason := readReason(rw); reason != "" {
				return fmt.Errorf("%w: %s", ErrAuthFailed, reason)
			}
		}
		return ErrAuthFailed
	}
	return nil
}

// ServerHandshakeNoAuth performs the server side of the RFB 3.8 handshake
// toward the browser (noVNC), offering the None security type only — the real
// VNC authentication already happened on the PVE leg (see the package doc).
// On return the next bytes from the client are its ClientInit message.
func ServerHandshakeNoAuth(rw io.ReadWriter) error {
	if _, err := rw.Write(Version38); err != nil {
		return fmt.Errorf("write server version: %w", err)
	}
	clientVersion := make([]byte, versionLen)
	if _, err := io.ReadFull(rw, clientVersion); err != nil {
		return fmt.Errorf("read client version: %w", err)
	}
	if !bytes.Equal(clientVersion, Version38) {
		return fmt.Errorf("unsupported client protocol version %q", clientVersion)
	}
	if _, err := rw.Write([]byte{1, SecurityNone}); err != nil {
		return fmt.Errorf("write security types: %w", err)
	}
	var chosen [1]byte
	if _, err := io.ReadFull(rw, chosen[:]); err != nil {
		return fmt.Errorf("read security type: %w", err)
	}
	if chosen[0] != SecurityNone {
		return fmt.Errorf("client chose unsupported security type %d", chosen[0])
	}
	// SecurityResult OK (RFB 3.8 sends it for the None type too).
	if _, err := rw.Write([]byte{0, 0, 0, 0}); err != nil {
		return fmt.Errorf("write security result: %w", err)
	}
	return nil
}

// vncAuthResponse encrypts the 16-byte VNC Authentication challenge with the
// password, as RFC 6143 §7.2.2 specifies: DES in ECB mode, the key being the
// first 8 bytes of the password (zero-padded) with the bit order of every
// byte reversed — a historical quirk of the original VNC implementation.
func vncAuthResponse(challenge []byte, password string) ([]byte, error) {
	if len(challenge) != 16 {
		return nil, fmt.Errorf("invalid challenge length %d", len(challenge))
	}
	key := make([]byte, 8)
	copy(key, password)
	for i, b := range key {
		key[i] = reverseBits(b)
	}
	// codeql[go/weak-crypto-algorithm]: DES is the cipher RFB's VNC Authentication mandates (RFC 6143 §7.2.2); the proxy cannot choose another one
	block, err := des.NewCipher(key) // NOSONAR — protocol-mandated cipher, see the comment above
	if err != nil {
		return nil, fmt.Errorf("vnc auth cipher: %w", err)
	}
	out := make([]byte, 16)
	block.Encrypt(out[:8], challenge[:8]) // NOSONAR — protocol-mandated cipher (RFC 6143 §7.2.2)
	block.Encrypt(out[8:], challenge[8:]) // NOSONAR — protocol-mandated cipher (RFC 6143 §7.2.2)
	return out, nil
}

func reverseBits(b byte) byte {
	var r byte
	for i := 0; i < 8; i++ {
		r = r<<1 | b&1
		b >>= 1
	}
	return r
}
