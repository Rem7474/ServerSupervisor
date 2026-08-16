//go:build linux

package collector

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// ---- Test packet builders (hand-rolled so the parsers are exercised against
// real byte layouts, not against a mirror of their own assumptions) ----

// buildClientHello assembles a minimal but structurally valid TLS record
// carrying a ClientHello whose only extension is server_name.
func buildClientHello(serverName string) []byte {
	var sni []byte
	if serverName != "" {
		host := []byte(serverName)
		entry := []byte{0x00} // name_type = host_name
		entry = append(entry, byte(len(host)>>8), byte(len(host)))
		entry = append(entry, host...)

		list := []byte{byte(len(entry) >> 8), byte(len(entry))}
		list = append(list, entry...)

		sni = []byte{0x00, 0x00} // extension type = server_name
		sni = append(sni, byte(len(list)>>8), byte(len(list)))
		sni = append(sni, list...)
	}

	body := []byte{0x03, 0x03}                  // client_version
	body = append(body, make([]byte, 32)...)    // random
	body = append(body, 0x00)                   // session_id length
	body = append(body, 0x00, 0x02, 0x13, 0x01) // cipher_suites
	body = append(body, 0x01, 0x00)             // compression_methods
	body = append(body, byte(len(sni)>>8), byte(len(sni)))
	body = append(body, sni...)

	hs := []byte{0x01, byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body))}
	hs = append(hs, body...)

	rec := []byte{0x16, 0x03, 0x01, byte(len(hs) >> 8), byte(len(hs))}
	return append(rec, hs...)
}

func buildTCP(srcPort, dstPort int, payload []byte) []byte {
	h := make([]byte, 20)
	binary.BigEndian.PutUint16(h[0:2], uint16(srcPort))
	binary.BigEndian.PutUint16(h[2:4], uint16(dstPort))
	h[12] = 5 << 4 // data offset = 5 words = 20 bytes
	return append(h, payload...)
}

func buildIPv4(src, dst string, proto byte, payload []byte) []byte {
	h := make([]byte, 20)
	h[0] = 4<<4 | 5
	binary.BigEndian.PutUint16(h[2:4], uint16(20+len(payload)))
	h[9] = proto
	copy(h[12:16], net.ParseIP(src).To4())
	copy(h[16:20], net.ParseIP(dst).To4())
	return append(h, payload...)
}

func buildIPv6(src, dst string, next byte, payload []byte) []byte {
	h := make([]byte, 40)
	h[0] = 6 << 4
	binary.BigEndian.PutUint16(h[4:6], uint16(len(payload)))
	h[6] = next
	copy(h[8:24], net.ParseIP(src).To16())
	copy(h[24:40], net.ParseIP(dst).To16())
	return append(h, payload...)
}

func tlsPacketV4(src, dst string, dstPort int, serverName string) []byte {
	return buildIPv4(src, dst, ipProtoTCP, buildTCP(45000, dstPort, buildClientHello(serverName)))
}

// ---- Parser tests ----

func TestParseTLSClientHelloSNI(t *testing.T) {
	t.Run("extracts the host name", func(t *testing.T) {
		got, ok := parseTLSClientHelloSNI(buildClientHello("github.com"))
		if !ok || got != "github.com" {
			t.Fatalf("got (%q, %v), want (github.com, true)", got, ok)
		}
	})

	t.Run("a ClientHello with no SNI extension yields nothing", func(t *testing.T) {
		if got, ok := parseTLSClientHelloSNI(buildClientHello("")); ok {
			t.Fatalf("got (%q, true), want ok=false", got)
		}
	})

	t.Run("non-handshake and truncated records are rejected without panicking", func(t *testing.T) {
		full := buildClientHello("example.org")
		cases := map[string][]byte{
			"nil":                nil,
			"empty":              {},
			"too short":          {0x16, 0x03},
			"application data":   {0x17, 0x03, 0x01, 0x00, 0x05, 1, 2, 3, 4, 5},
			"not a ClientHello":  {0x16, 0x03, 0x01, 0x00, 0x04, 0x02, 0x00, 0x00, 0x00},
			"truncated mid-body": full[:len(full)-4],
			"truncated header":   full[:6],
			"all zeroes":         make([]byte, 64),
		}
		for name, b := range cases {
			if got, ok := parseTLSClientHelloSNI(b); ok {
				t.Errorf("%s: got (%q, true), want ok=false", name, got)
			}
		}
	})

	t.Run("every prefix of a valid record is safe", func(t *testing.T) {
		// Guards the whole bounds-checking surface against a snaplen-truncated
		// capture, which is a routine occurrence, not an edge case.
		full := buildClientHello("truncation.test")
		for i := 0; i < len(full); i++ {
			_, _ = parseTLSClientHelloSNI(full[:i]) // must not panic
		}
		if got, ok := parseTLSClientHelloSNI(full); !ok || got != "truncation.test" {
			t.Fatalf("full record still parses? got (%q, %v)", got, ok)
		}
	})
}

func TestParseSNIFromIPPacket(t *testing.T) {
	t.Run("IPv4 TCP ClientHello", func(t *testing.T) {
		ip, port, name, ok := parseSNIFromIPPacket(tlsPacketV4("172.17.0.4", "140.82.121.4", 443, "github.com"))
		if !ok {
			t.Fatal("expected a parse")
		}
		if ip != "140.82.121.4" || port != 443 || name != "github.com" {
			t.Fatalf("got (%s, %d, %s), want (140.82.121.4, 443, github.com)", ip, port, name)
		}
	})

	t.Run("IPv6 TCP ClientHello", func(t *testing.T) {
		pkt := buildIPv6("2001:db8::2", "2606:50c0::1", ipProtoTCP, buildTCP(45000, 8443, buildClientHello("example.net")))
		ip, port, name, ok := parseSNIFromIPPacket(pkt)
		if !ok {
			t.Fatal("expected a parse")
		}
		if ip != "2606:50c0::1" || port != 8443 || name != "example.net" {
			t.Fatalf("got (%s, %d, %s)", ip, port, name)
		}
	})

	t.Run("non-TLS traffic is ignored", func(t *testing.T) {
		cases := map[string][]byte{
			"UDP":                     buildIPv4("10.0.0.1", "8.8.8.8", 17, buildTCP(1, 53, []byte("dns"))),
			"TCP without a handshake": buildIPv4("10.0.0.1", "1.1.1.1", ipProtoTCP, buildTCP(1, 80, []byte("GET / HTTP/1.1\r\n"))),
			"not IP at all":           {0xff, 0xff, 0xff, 0xff, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			"too short":               {0x45, 0x00},
		}
		for name, pkt := range cases {
			if _, _, _, ok := parseSNIFromIPPacket(pkt); ok {
				t.Errorf("%s: expected no parse", name)
			}
		}
	})

	t.Run("a non-first IP fragment is skipped", func(t *testing.T) {
		pkt := tlsPacketV4("10.0.0.1", "1.1.1.1", 443, "frag.test")
		binary.BigEndian.PutUint16(pkt[6:8], 0x0025) // non-zero fragment offset
		if _, _, _, ok := parseSNIFromIPPacket(pkt); ok {
			t.Fatal("expected a non-first fragment to be skipped")
		}
	})
}

// ---- Merge tests ----

func TestMergeServerNames(t *testing.T) {
	names := map[string]string{
		"140.82.121.4:443":   "github.com",
		"1.1.1.1:443":        "cloudflare-dns.com",
		"[2606:50c0::1]:443": "example.net",
	}

	talkers := []NetworkFlowTalker{
		{RemoteIP: "140.82.121.4", RemotePort: 443, Direction: "outbound"},
		{RemoteIP: "2606:50c0::1", RemotePort: 443, Direction: "outbound"},
		{RemoteIP: "1.1.1.1", RemotePort: 443, Direction: "inbound"},      // inbound: never labelled
		{RemoteIP: "9.9.9.9", RemotePort: 443, Direction: "outbound"},     // not observed
		{RemoteIP: "140.82.121.4", RemotePort: 22, Direction: "outbound"}, // different port
	}
	mergeServerNames(talkers, names)

	want := []string{"github.com", "example.net", "", "", ""}
	for i, w := range want {
		if talkers[i].ServerName != w {
			t.Errorf("talkers[%d].ServerName = %q, want %q", i, talkers[i].ServerName, w)
		}
	}
}

func TestMergeServerNamesNoOps(t *testing.T) {
	talkers := []NetworkFlowTalker{{RemoteIP: "1.1.1.1", RemotePort: 443, Direction: "outbound", ServerName: "already.set"}}

	mergeServerNames(talkers, nil)
	if talkers[0].ServerName != "already.set" {
		t.Fatal("a nil name map must be a no-op")
	}
	mergeServerNames(talkers, map[string]string{"1.1.1.1:443": "overwrite.me"})
	if talkers[0].ServerName != "already.set" {
		t.Errorf("an existing ServerName must not be overwritten, got %q", talkers[0].ServerName)
	}
	mergeServerNames(nil, map[string]string{"1.1.1.1:443": "x"}) // must not panic
}

// ---- Capture-loop test through the packetSource seam ----

// fakeSource replays a fixed list of frames, then blocks on "nothing to read"
// the way a live socket does between packets.
type fakeSource struct {
	packets [][]byte
	idx     int
	closed  bool
}

func (f *fakeSource) ReadPacket() ([]byte, error) {
	if f.idx >= len(f.packets) {
		time.Sleep(time.Millisecond)
		return nil, nil // idle, same shape as a socket read timeout
	}
	p := f.packets[f.idx]
	f.idx++
	return p, nil
}

func (f *fakeSource) Close() error { f.closed = true; return nil }

func TestSniSnifferCaptureCollectsNames(t *testing.T) {
	src := &fakeSource{packets: [][]byte{
		tlsPacketV4("172.17.0.4", "140.82.121.4", 443, "github.com"),
		buildIPv4("10.0.0.1", "8.8.8.8", 17, buildTCP(1, 53, []byte("not tls"))),
		tlsPacketV4("172.17.0.4", "1.1.1.1", 443, "cloudflare-dns.com"),
		// A second hello to an endpoint already seen must not re-key it.
		tlsPacketV4("172.17.0.4", "140.82.121.4", 443, "other.example"),
	}}

	s := &sniSniffer{names: map[string]string{}, done: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		defer close(s.done)
		s.capture(ctx, src)
	}()

	deadline := time.After(2 * time.Second)
	for {
		s.mu.Lock()
		n := len(s.names)
		s.mu.Unlock()
		if n >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for captured names (got %d)", n)
		case <-time.After(5 * time.Millisecond):
		}
	}

	cancel()
	<-s.done

	s.mu.Lock()
	defer s.mu.Unlock()
	if got := s.names["140.82.121.4:443"]; got != "github.com" {
		t.Errorf("first observation should win, got %q", got)
	}
	if got := s.names["1.1.1.1:443"]; got != "cloudflare-dns.com" {
		t.Errorf("names[1.1.1.1:443] = %q", got)
	}
	if len(s.names) != 2 {
		t.Errorf("expected exactly 2 endpoints, got %v", s.names)
	}
}

func TestStartSNISnifferDisabledIsNil(t *testing.T) {
	if s := startSNISniffer(context.Background(), false); s != nil {
		t.Fatal("capture must not start when the config flag is off")
	}
	// The nil sniffer must flow through the rest of the pipeline harmlessly —
	// this is the path every default install takes.
	if got := stopSNISniffer(nil); len(got) != 0 {
		t.Fatalf("stopSNISniffer(nil) = %v, want empty", got)
	}
}
