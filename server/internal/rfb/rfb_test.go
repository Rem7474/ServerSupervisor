package rfb

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// TestVNCAuthResponse_KnownVector checks the DES response against a value
// computed independently with OpenSSL (des-ecb, key = bit-reversed password
// bytes), so the bit-reversal quirk is verified rather than assumed.
func TestVNCAuthResponse_KnownVector(t *testing.T) {
	challenge, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f")
	got, err := vncAuthResponse(challenge, "Pa55w0rd")
	if err != nil {
		t.Fatalf("vncAuthResponse: %v", err)
	}
	if want := "e401ff411db1cf4cedbba1df58af05e2"; hex.EncodeToString(got) != want {
		t.Errorf("response = %x, want %s", got, want)
	}
}

func TestVNCAuthResponse_OnlyFirstEightBytesOfPasswordCount(t *testing.T) {
	challenge := bytes.Repeat([]byte{0x42}, 16)
	a, _ := vncAuthResponse(challenge, "Pa55w0rd")
	b, _ := vncAuthResponse(challenge, "Pa55w0rd-and-more")
	if !bytes.Equal(a, b) {
		t.Error("characters past the 8th must be ignored, as in every VNC implementation")
	}
}

func TestVNCAuthResponse_BadChallenge(t *testing.T) {
	if _, err := vncAuthResponse([]byte{1, 2, 3}, "x"); err == nil {
		t.Error("expected an error for a short challenge")
	}
}

func TestReverseBits(t *testing.T) {
	cases := map[byte]byte{0x01: 0x80, 0x80: 0x01, 0x0f: 0xf0, 0x00: 0x00, 0xa5: 0xa5, 0x50: 0x0a}
	for in, want := range cases {
		if got := reverseBits(in); got != want {
			t.Errorf("reverseBits(%#x) = %#x, want %#x", in, got, want)
		}
	}
}

// pipe returns both ends of an in-memory connection with a deadline, so a
// handshake bug fails the test instead of hanging it.
func pipe(t *testing.T) (client, server net.Conn) {
	t.Helper()
	client, server = net.Pipe()
	deadline := time.Now().Add(5 * time.Second)
	_ = client.SetDeadline(deadline)
	_ = server.SetDeadline(deadline)
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	return client, server
}

// fakeVNCServer scripts the server side of a handshake and reports what it
// observed on errc (nil when the script ran to completion).
type fakeVNCServer struct {
	version  string
	types    []byte
	refusal  string // sent when types is empty
	password string // expected for VNC auth
	result   uint32 // SecurityResult to send (0 = OK)
	reason   string // failure reason (3.8 only)
}

func (f fakeVNCServer) run(conn net.Conn, errc chan<- error) {
	errc <- func() error {
		if _, err := conn.Write([]byte(f.version)); err != nil {
			return err
		}
		clientVersion := make([]byte, 12)
		if _, err := io.ReadFull(conn, clientVersion); err != nil {
			return err
		}
		if len(f.types) == 0 {
			_, _ = conn.Write([]byte{0})
			_ = binary.Write(conn, binary.BigEndian, uint32(len(f.refusal)))
			_, _ = conn.Write([]byte(f.refusal))
			return nil
		}
		if _, err := conn.Write(append([]byte{byte(len(f.types))}, f.types...)); err != nil {
			return err
		}
		chosen := make([]byte, 1)
		if _, err := io.ReadFull(conn, chosen); err != nil {
			return err
		}
		minor8 := string(clientVersion) == "RFB 003.008\n"
		switch chosen[0] {
		case SecurityVNCAuth:
			challenge := bytes.Repeat([]byte{7}, 16)
			if _, err := conn.Write(challenge); err != nil {
				return err
			}
			resp := make([]byte, 16)
			if _, err := io.ReadFull(conn, resp); err != nil {
				return err
			}
			want, _ := vncAuthResponse(challenge, f.password)
			result := f.result
			if !bytes.Equal(resp, want) {
				result = 1
			}
			_ = binary.Write(conn, binary.BigEndian, result)
			if result != 0 && minor8 {
				_ = binary.Write(conn, binary.BigEndian, uint32(len(f.reason)))
				_, _ = conn.Write([]byte(f.reason))
			}
		case SecurityNone:
			if minor8 {
				_ = binary.Write(conn, binary.BigEndian, f.result)
				if f.result != 0 {
					_ = binary.Write(conn, binary.BigEndian, uint32(len(f.reason)))
					_, _ = conn.Write([]byte(f.reason))
				}
			}
		default:
			return errors.New("unexpected security type")
		}
		return nil
	}()
}

func runClient(t *testing.T, f fakeVNCServer, password string) error {
	t.Helper()
	client, server := pipe(t)
	errc := make(chan error, 1)
	go f.run(server, errc)
	err := ClientHandshake(client, password)
	_ = client.Close()
	<-errc
	return err
}

func TestClientHandshake_VNCAuthSuccess(t *testing.T) {
	err := runClient(t, fakeVNCServer{version: "RFB 003.008\n", types: []byte{SecurityVNCAuth}, password: "s3cretpw"}, "s3cretpw")
	if err != nil {
		t.Fatalf("handshake: %v", err)
	}
}

func TestClientHandshake_VNCAuthWrongPasswordReportsReason(t *testing.T) {
	err := runClient(t, fakeVNCServer{version: "RFB 003.008\n", types: []byte{SecurityVNCAuth}, password: "s3cretpw", reason: "bad password"}, "wrong")
	if !errors.Is(err, ErrAuthFailed) || !strings.Contains(err.Error(), "bad password") {
		t.Fatalf("err = %v, want ErrAuthFailed with the server's reason", err)
	}
}

func TestClientHandshake_VNCAuthFailureWithoutReason(t *testing.T) {
	err := runClient(t, fakeVNCServer{version: "RFB 003.007\n", types: []byte{SecurityVNCAuth}, password: "s3cretpw"}, "wrong")
	if !errors.Is(err, ErrAuthFailed) {
		t.Fatalf("err = %v, want ErrAuthFailed", err)
	}
}

func TestClientHandshake_PrefersNone(t *testing.T) {
	// Password empty: choosing VNC auth would fail, so success proves None won.
	if err := runClient(t, fakeVNCServer{version: "RFB 003.008\n", types: []byte{SecurityVNCAuth, SecurityNone}}, ""); err != nil {
		t.Fatalf("handshake: %v", err)
	}
}

func TestClientHandshake_None37HasNoSecurityResult(t *testing.T) {
	if err := runClient(t, fakeVNCServer{version: "RFB 003.007\n", types: []byte{SecurityNone}}, ""); err != nil {
		t.Fatalf("handshake: %v", err)
	}
}

func TestClientHandshake_NewerServerVersionNegotiatedDownTo38(t *testing.T) {
	if err := runClient(t, fakeVNCServer{version: "RFB 003.889\n", types: []byte{SecurityNone}}, ""); err != nil {
		t.Fatalf("handshake: %v", err)
	}
}

func TestClientHandshake_Errors(t *testing.T) {
	cases := []struct {
		name     string
		srv      fakeVNCServer
		password string
		want     string
	}{
		{"refused", fakeVNCServer{version: "RFB 003.008\n", refusal: "too many clients"}, "", "too many clients"},
		{"unsupported types", fakeVNCServer{version: "RFB 003.008\n", types: []byte{19}}, "", "no supported security type"},
		{"no password", fakeVNCServer{version: "RFB 003.008\n", types: []byte{SecurityVNCAuth}}, "", "no password"},
		{"rfb 3.3", fakeVNCServer{version: "RFB 003.003\n"}, "", "unsupported protocol version"},
		{"garbage version", fakeVNCServer{version: "HTTP/1.1 200\n"}, "", "protocol version"},
		{"none rejected", fakeVNCServer{version: "RFB 003.008\n", types: []byte{SecurityNone}, result: 1}, "", "authentication failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := runClient(t, tc.srv, tc.password)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestClientHandshake_ShortVersion(t *testing.T) {
	client, server := pipe(t)
	go func() { _, _ = server.Write([]byte("RFB")); _ = server.Close() }()
	if err := ClientHandshake(client, ""); err == nil {
		t.Fatal("expected an error on a truncated version")
	}
}

func TestParseVersion_Malformed(t *testing.T) {
	if _, err := parseVersion([]byte("RFB 003.008")); err == nil {
		t.Error("expected an error for a version without trailing newline")
	}
}

func TestServerHandshakeNoAuth_Success(t *testing.T) {
	browser, proxy := pipe(t)
	errc := make(chan error, 1)
	go func() { errc <- ServerHandshakeNoAuth(proxy) }()

	version := make([]byte, 12)
	if _, err := io.ReadFull(browser, version); err != nil || string(version) != "RFB 003.008\n" {
		t.Fatalf("version = %q, %v", version, err)
	}
	_, _ = browser.Write(Version38)
	types := make([]byte, 2)
	if _, err := io.ReadFull(browser, types); err != nil || !bytes.Equal(types, []byte{1, SecurityNone}) {
		t.Fatalf("security types = %v, %v; want only None", types, err)
	}
	_, _ = browser.Write([]byte{SecurityNone})
	result := make([]byte, 4)
	if _, err := io.ReadFull(browser, result); err != nil || !bytes.Equal(result, []byte{0, 0, 0, 0}) {
		t.Fatalf("security result = %v, %v", result, err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("ServerHandshakeNoAuth: %v", err)
	}
}

func TestServerHandshakeNoAuth_RejectsOtherVersion(t *testing.T) {
	browser, proxy := pipe(t)
	errc := make(chan error, 1)
	go func() { errc <- ServerHandshakeNoAuth(proxy) }()
	_, _ = io.ReadFull(browser, make([]byte, 12))
	_, _ = browser.Write([]byte("RFB 003.003\n"))
	if err := <-errc; err == nil || !strings.Contains(err.Error(), "unsupported client protocol version") {
		t.Fatalf("err = %v", err)
	}
}

func TestServerHandshakeNoAuth_RejectsOtherSecurityType(t *testing.T) {
	browser, proxy := pipe(t)
	errc := make(chan error, 1)
	go func() { errc <- ServerHandshakeNoAuth(proxy) }()
	_, _ = io.ReadFull(browser, make([]byte, 12))
	_, _ = browser.Write(Version38)
	_, _ = io.ReadFull(browser, make([]byte, 2))
	_, _ = browser.Write([]byte{SecurityVNCAuth})
	if err := <-errc; err == nil || !strings.Contains(err.Error(), "unsupported security type") {
		t.Fatalf("err = %v", err)
	}
}

func TestServerHandshakeNoAuth_ClientGone(t *testing.T) {
	browser, proxy := pipe(t)
	errc := make(chan error, 1)
	go func() { errc <- ServerHandshakeNoAuth(proxy) }()
	_, _ = io.ReadFull(browser, make([]byte, 12))
	_ = browser.Close()
	if err := <-errc; err == nil {
		t.Fatal("expected an error when the client disconnects mid-handshake")
	}
}
