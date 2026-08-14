//go:build linux

// network_flows_l7.go adds an *optional*, off-by-default application-layer
// signal to the conntrack-derived top-talkers collector: the TLS SNI hostname a
// flow's ClientHello asked for. Enabled with the agent's
// `network_flows_l7_capture` config key.
//
// Scope, deliberately narrow — see the "maturity" note in the feature's report:
// TLS ClientHello SNI only. DNS query names and HTTP Host headers are the
// obvious next signals and are explicitly *not* implemented here; they would
// each need their own parser plus (for HTTP) cross-segment reassembly, and
// shipping one well-tested signal beats three shaky ones.
//
// # Why no new dependency
//
// The obvious candidate was github.com/google/gopacket (or its maintained fork
// github.com/gopacket/gopacket) with libpcap. That was rejected on a concrete,
// checkable blocker rather than taste: gopacket/pcap needs cgo + libpcap
// headers, and this agent cross-compiles to four targets with CGO_ENABLED=0
// (see build.sh / .github/workflows/release.yml), so adding it would break the
// arm64/armv7/armv6 builds outright. gopacket/afpacket is cgo-free but pulls in
// the whole gopacket tree to do what this narrow case needs, which is: open one
// AF_PACKET socket and read a ClientHello. golang.org/x/sys/unix is already a
// direct dependency of this very file's package (network_flows.go uses it for
// the conntrack protocol constants), so the capture is ~40 lines of syscall on
// top of what's already vendored. Net new dependencies: zero.
//
// # Safety posture
//
// AF_PACKET needs CAP_NET_RAW. This mirrors internal/synthetic's ICMP prober:
// try, and on failure record a clear "CAP_NET_RAW manquant" condition and
// degrade — never crash, never block the collector loop, never go silently
// dark. The regular conntrack collection and the UI's own well-known-port
// heuristic are unaffected when capture is unavailable.
package collector

import (
	"context"
	"encoding/binary"
	"errors"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const (
	// sniCaptureWindow is how long a cycle samples packets. It runs
	// concurrently with the conntrack + /proc work, so it does not extend the
	// collection budget — but it is still deliberately short: this is a
	// sampling probe, not a full-fidelity capture. A flow whose ClientHello
	// happened outside the window simply keeps its port-heuristic label.
	sniCaptureWindow = 3 * time.Second
	// sniMaxPackets bounds CPU on a busy host: the socket is unfiltered (see
	// the BPF note in newAFPacketSource), so this cap — not the traffic rate —
	// is what determines the worst-case cost of a cycle.
	sniMaxPackets = 4000
	// sniMaxNames bounds memory/report growth independently of packet count.
	sniMaxNames = 500
	// sniSnapLen only needs to cover a ClientHello's SNI extension, which sits
	// within the first few hundred bytes of the first TCP segment.
	sniSnapLen      = 1600
	sniReadTimeout  = 250 * time.Millisecond
	tlsHandshake    = 0x16
	tlsClientHello  = 0x01
	tlsExtServerNam = 0x0000
	ipProtoTCP      = 6
)

// packetSource is the seam that keeps everything except the ~40 lines of actual
// syscall unit-testable: the parse/collect/merge logic below is driven entirely
// through this interface, so a test can feed it hand-built frames with no
// CAP_NET_RAW, no NIC and no root (see network_flows_l7_test.go's fakeSource).
type packetSource interface {
	// ReadPacket returns the next network-layer packet (IP header first — the
	// link-layer header is already stripped, see newAFPacketSource). A nil
	// error with a nil packet means "nothing this time, try again".
	ReadPacket() ([]byte, error)
	Close() error
}

// sniSniffer accumulates observed "remote endpoint → SNI hostname" pairs for
// one collection cycle.
type sniSniffer struct {
	mu     sync.Mutex
	names  map[string]string
	done   chan struct{}
	cancel context.CancelFunc
}

var (
	// l7StatusMu guards the last capture-availability error, surfaced to the
	// user through diagnostics.go's CheckConfig rather than only a log line.
	l7StatusMu   sync.RWMutex
	l7LastErr    string
	l7WarnedOnce sync.Once
)

// L7CaptureError returns the last reason the optional L7 capture could not run,
// or "" if it has never failed (or was never enabled). Read by CheckConfig.
func L7CaptureError() string {
	l7StatusMu.RLock()
	defer l7StatusMu.RUnlock()
	return l7LastErr
}

func setL7Error(msg string) {
	l7StatusMu.Lock()
	l7LastErr = msg
	l7StatusMu.Unlock()
}

// startSNISniffer begins a bounded background capture, or returns nil when the
// feature is disabled or the capability is missing. A nil *sniSniffer is a
// valid value everywhere below — callers never need to nil-check.
func startSNISniffer(ctx context.Context, enabled bool) *sniSniffer {
	if !enabled {
		return nil
	}
	src, err := newAFPacketSource()
	if err != nil {
		msg := "capture L7 indisponible : " + err.Error()
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			msg = "capture L7 indisponible : CAP_NET_RAW manquant sur le binaire de l'agent " +
				"(setcap cap_net_raw+ep /usr/local/bin/serversupervisor-agent, ou désactiver network_flows_l7_capture)"
		}
		setL7Error(msg)
		// Once per process: this condition is static (a capability, not a
		// transient), so logging it every report cycle would be pure noise. The
		// diagnostics banner carries the persistent signal instead.
		l7WarnedOnce.Do(func() { slog.Warn("network flows L7 capture disabled", "reason", msg) })
		return nil
	}
	setL7Error("")

	captureCtx, cancel := context.WithTimeout(ctx, sniCaptureWindow)
	s := &sniSniffer{
		names:  map[string]string{},
		done:   make(chan struct{}),
		cancel: cancel,
	}
	go func() {
		defer close(s.done)
		defer cancel()
		defer func() { _ = src.Close() }()
		s.capture(captureCtx, src)
	}()
	return s
}

// capture reads until the window closes, the packet cap is hit, or the source
// fails. Every parse failure is silent by design — an unfiltered socket sees
// ARP, DNS, VXLAN and every other non-TLS packet on the wire, so "this isn't a
// ClientHello" is the overwhelmingly common case, not an error.
func (s *sniSniffer) capture(ctx context.Context, src packetSource) {
	for i := 0; i < sniMaxPackets; i++ {
		if ctx.Err() != nil {
			return
		}
		pkt, err := src.ReadPacket()
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EINTR) {
				continue // read timeout — expected, keep sampling until the window closes
			}
			return
		}
		if len(pkt) == 0 {
			continue
		}
		ip, port, name, ok := parseSNIFromIPPacket(pkt)
		if !ok {
			continue
		}
		s.mu.Lock()
		if len(s.names) < sniMaxNames {
			// First observation wins: a later ClientHello to the same endpoint
			// is the same server, and re-keying on every packet would let a
			// noisy multiplexed endpoint flap the reported label.
			key := net.JoinHostPort(ip, strconv.Itoa(port))
			if _, exists := s.names[key]; !exists {
				s.names[key] = name
			}
		}
		s.mu.Unlock()
	}
}

// stopSNISniffer ends the capture and returns what it observed, keyed by
// "ip:port". Nil-safe: a nil sniffer (feature off, or capability missing)
// yields an empty map, which mergeServerNames then treats as a no-op.
func stopSNISniffer(s *sniSniffer) map[string]string {
	if s == nil {
		return nil
	}
	s.cancel()
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string, len(s.names))
	for k, v := range s.names {
		out[k] = v
	}
	return out
}

// mergeServerNames stamps observed SNI hostnames onto matching talkers, in
// place. Outbound only: SNI travels client→server, so the ClientHello's
// destination is the *remote peer* only for a connection this host initiated.
// For an inbound connection the same packet's destination is this host's own
// address, which would be a wrong label to hang on the remote client.
func mergeServerNames(talkers []NetworkFlowTalker, names map[string]string) {
	if len(names) == 0 {
		return
	}
	for i := range talkers {
		if talkers[i].Direction != "outbound" || talkers[i].ServerName != "" {
			continue
		}
		key := net.JoinHostPort(talkers[i].RemoteIP, strconv.Itoa(talkers[i].RemotePort))
		if name, ok := names[key]; ok {
			talkers[i].ServerName = name
		}
	}
}

// ---- Pure parsers (no syscalls, no clock) ----

// parseSNIFromIPPacket walks IPv4/IPv6 → TCP → TLS ClientHello → SNI and
// returns the packet's *destination* endpoint plus the requested hostname.
// ok is false for anything that isn't a ClientHello carrying an SNI extension.
func parseSNIFromIPPacket(pkt []byte) (dstIP string, dstPort int, serverName string, ok bool) {
	if len(pkt) < 20 {
		return "", 0, "", false
	}

	var dst net.IP
	var proto byte
	var rest []byte

	switch pkt[0] >> 4 {
	case 4:
		ihl := int(pkt[0]&0x0f) * 4
		if ihl < 20 || len(pkt) < ihl {
			return "", 0, "", false
		}
		// A non-first fragment carries no TCP header, and reassembly is out of
		// scope for a sampling probe — a ClientHello virtually never fragments.
		if binary.BigEndian.Uint16(pkt[6:8])&0x1fff != 0 {
			return "", 0, "", false
		}
		proto = pkt[9]
		dst = net.IP(pkt[16:20])
		rest = pkt[ihl:]
	case 6:
		if len(pkt) < 40 {
			return "", 0, "", false
		}
		// Extension headers (fragment, routing, …) are not walked: they're
		// vanishingly rare on a TLS handshake and would need a full chain
		// parser for no practical gain here.
		proto = pkt[6]
		dst = net.IP(pkt[24:40])
		rest = pkt[40:]
	default:
		return "", 0, "", false
	}

	if proto != ipProtoTCP || len(rest) < 20 {
		return "", 0, "", false
	}
	dataOff := int(rest[12]>>4) * 4
	if dataOff < 20 || len(rest) < dataOff {
		return "", 0, "", false
	}
	name, found := parseTLSClientHelloSNI(rest[dataOff:])
	if !found {
		return "", 0, "", false
	}
	return dst.String(), int(binary.BigEndian.Uint16(rest[2:4])), name, true
}

// parseTLSClientHelloSNI extracts the first host_name entry from a TLS
// ClientHello's server_name extension. Every field read is length-checked
// against attacker-controlled data off the wire — a malformed or truncated
// record returns false, never panics.
func parseTLSClientHelloSNI(b []byte) (string, bool) {
	// TLS record header: type(1) version(2) length(2).
	if len(b) < 5 || b[0] != tlsHandshake {
		return "", false
	}
	body := clip(b[5:], int(binary.BigEndian.Uint16(b[3:5])))

	// Handshake header: type(1) length(3).
	if len(body) < 4 || body[0] != tlsClientHello {
		return "", false
	}
	h := clip(body[4:], int(body[1])<<16|int(body[2])<<8|int(body[3]))

	// client_version(2) + random(32).
	off := 34
	// legacy_session_id
	v, off, ok := readVector(h, off, 1)
	if !ok {
		return "", false
	}
	_ = v
	// cipher_suites
	if _, off, ok = readVector(h, off, 2); !ok {
		return "", false
	}
	// legacy_compression_methods
	if _, off, ok = readVector(h, off, 1); !ok {
		return "", false
	}
	// extensions
	exts, _, ok := readVector(h, off, 2)
	if !ok {
		return "", false
	}

	for len(exts) >= 4 {
		extType := binary.BigEndian.Uint16(exts[0:2])
		extLen := int(binary.BigEndian.Uint16(exts[2:4]))
		if len(exts) < 4+extLen {
			return "", false
		}
		data := exts[4 : 4+extLen]
		exts = exts[4+extLen:]
		if extType != tlsExtServerNam {
			continue
		}
		// ServerNameList: list_length(2), then entries of
		// name_type(1) + host_name length(2) + host_name.
		list, _, listOK := readVector(data, 0, 2)
		if !listOK {
			return "", false
		}
		for len(list) >= 3 {
			nameType := list[0]
			nameLen := int(binary.BigEndian.Uint16(list[1:3]))
			if len(list) < 3+nameLen {
				return "", false
			}
			if nameType == 0 && nameLen > 0 {
				return string(list[3 : 3+nameLen]), true
			}
			list = list[3+nameLen:]
		}
		return "", false
	}
	return "", false
}

// readVector reads a TLS variable-length vector at b[off:] whose length is
// encoded in lenBytes (1 or 2) bytes, returning its contents and the offset
// just past it.
func readVector(b []byte, off, lenBytes int) (data []byte, next int, ok bool) {
	if off < 0 || off+lenBytes > len(b) {
		return nil, 0, false
	}
	n := 0
	for i := 0; i < lenBytes; i++ {
		n = n<<8 | int(b[off+i])
	}
	start := off + lenBytes
	if start+n > len(b) {
		return nil, 0, false
	}
	return b[start : start+n], start + n, true
}

// clip truncates b to n when the declared length is shorter than what was
// captured (trailing records, padding), and leaves it alone when the capture is
// the shorter of the two (snaplen truncation) — the parsers above bounds-check
// every read either way.
func clip(b []byte, n int) []byte {
	if n >= 0 && n < len(b) {
		return b[:n]
	}
	return b
}

// ---- AF_PACKET capture ----

type afPacketSource struct {
	fd  int
	buf []byte
}

// newAFPacketSource opens a cooked (SOCK_DGRAM) AF_PACKET socket, which hands
// back packets with the link-layer header already stripped — so the parsers
// above start at the IP header and never need Ethernet/VLAN handling.
//
// No BPF filter is attached, deliberately: the correct filter offsets differ
// between cooked and raw AF_PACKET sockets, and a filter that's subtly wrong
// fails *closed* (captures nothing, feature silently dead) rather than open.
// The cost is instead bounded by sniCaptureWindow + sniMaxPackets. Attaching a
// verified `tcp dst port 443` BPF program is the natural optimisation once this
// has run on real hosts.
func newAFPacketSource() (packetSource, error) {
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, int(hostShort(unix.ETH_P_ALL)))
	if err != nil {
		return nil, err
	}
	// Bound reads so the capture goroutine reliably notices its context expiring
	// on a quiet interface instead of blocking in recv until the next packet.
	tv := unix.NsecToTimeval(int64(sniReadTimeout))
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	return &afPacketSource{fd: fd, buf: make([]byte, sniSnapLen)}, nil
}

func (s *afPacketSource) ReadPacket() ([]byte, error) {
	n, err := unix.Read(s.fd, s.buf)
	if err != nil {
		return nil, err
	}
	if n <= 0 {
		return nil, nil
	}
	return s.buf[:n], nil
}

func (s *afPacketSource) Close() error { return unix.Close(s.fd) }

// hostShort converts a big-endian (network order) protocol constant into the
// host byte order the AF_PACKET socket() call expects.
func hostShort(v uint16) uint16 {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], v)
	return binary.NativeEndian.Uint16(b[:])
}
