//go:build linux

// Package collector's network_flows.go aggregates conntrack accounting into a
// bounded "top talkers" snapshot per report cycle: this host's busiest remote
// peers (IP/port/protocol), with best-effort process attribution. Zero
// external binary — reads netlink conntrack directly (github.com/vishvananda/
// netlink) and /proc for process attribution, same "poor man's netstat" style
// as the rest of this package (see system.go's /proc/net/dev network byte
// counters).
//
// Forwarded/NAT traffic (neither side of a connection is a local IP — e.g.
// docker0 container-to-container NAT) is deliberately excluded: this
// collector answers "who is this host talking to", not general routed
// traffic, and there is no single "remote peer" to attribute it to without
// risking a wrong classification.
package collector

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// NetworkFlowsReport is the agent-collected "top talkers" block for one
// report cycle. Mirrors server/internal/models.NetworkFlowsReport — kept in
// sync manually per the agent↔server golden-fixture contract (see
// protocol/README.md), not shared code.
type NetworkFlowsReport struct {
	// Available is false when this host can't provide per-connection byte
	// counters right now (nf_conntrack not loaded, accounting disabled, or
	// collection disabled in agent.yaml) — never an error, a capability flag.
	Available   bool                `json:"available"`
	Reason      string              `json:"reason,omitempty"`
	TopTalkers  []NetworkFlowTalker `json:"top_talkers,omitempty"`
	Others      *NetworkFlowBucket  `json:"others,omitempty"`
	TotalFlows  int                 `json:"total_flows"`
	CollectedAt time.Time           `json:"collected_at"`
}

// NetworkFlowTalker is one aggregated remote peer for this cycle.
// RxBytes/TxBytes/Packets are deltas since the previous cycle (see
// deltaFlows), not conntrack's cumulative counters.
type NetworkFlowTalker struct {
	RemoteIP    string `json:"remote_ip"`
	RemotePort  int    `json:"remote_port"`
	Protocol    string `json:"protocol"`  // "tcp" | "udp"
	Direction   string `json:"direction"` // "inbound" | "outbound"
	ProcessName string `json:"process_name,omitempty"`
	PID         int    `json:"pid,omitempty"`
	RxBytes     uint64 `json:"rx_bytes"`
	TxBytes     uint64 `json:"tx_bytes"`
	Packets     uint64 `json:"packets"`
	Connections int    `json:"connections"`
}

// NetworkFlowBucket aggregates every talker beyond the top-N cutoff.
type NetworkFlowBucket struct {
	Connections int    `json:"connections"`
	RxBytes     uint64 `json:"rx_bytes"`
	TxBytes     uint64 `json:"tx_bytes"`
}

// conntrackTimeout bounds the netlink conntrack table read, as a sub-budget
// of reporter.go's collectionTimeout (25s) — this collector runs in its own
// goroutine in that same WaitGroup, so it must never be the one that hangs it.
const conntrackTimeout = 5 * time.Second

// flowKey identifies one conntrack connection across report cycles by its
// original-direction (Forward) tuple, which is stable for the connection's
// lifetime regardless of which side initiated it.
type flowKey struct {
	protocol uint8
	srcIP    string
	dstIP    string
	srcPort  uint16
	dstPort  uint16
}

type flowCounters struct {
	bytesFwd uint64
	bytesRev uint64
	packets  uint64
}

// prevFlows holds the last cycle's cumulative conntrack counters so
// deltaFlows can report per-cycle bytes instead of connection-lifetime
// totals. Package-level (one agent process, one collection at a time —
// reporter.Send never runs two report cycles concurrently).
var (
	prevFlowsMu sync.Mutex
	prevFlows   = map[flowKey]flowCounters{}
)

// checkConntrackAcct reports whether the kernel can provide per-connection
// byte accounting. Missing module or disabled accounting are both common,
// legitimate states (VPS/LXC/hardened kernels) — never an error.
func checkConntrackAcct() (bool, string) {
	data, err := os.ReadFile("/proc/sys/net/netfilter/nf_conntrack_acct")
	if err != nil {
		return false, "nf_conntrack module not loaded (missing /proc/sys/net/netfilter/nf_conntrack_acct)"
	}
	if strings.TrimSpace(string(data)) != "1" {
		return false, "nf_conntrack_acct disabled — enable with sysctl -w net.netfilter.nf_conntrack_acct=1 (persist in /etc/sysctl.d) to collect per-connection byte counters"
	}
	return true, ""
}

// CollectNetworkFlows aggregates conntrack accounting into a bounded top-N +
// "others" snapshot of this host's busiest remote peers this cycle. Never
// returns an error for a degraded/unavailable kernel — that's reported via
// Available/Reason instead, so one host without conntrack never breaks its
// report cycle.
func CollectNetworkFlows(ctx context.Context, topN int) (*NetworkFlowsReport, error) {
	if topN <= 0 {
		topN = 50
	}
	now := time.Now()

	if ok, reason := checkConntrackAcct(); !ok {
		return &NetworkFlowsReport{Available: false, Reason: reason, CollectedAt: now}, nil
	}

	flows, err := listConntrackFlows()
	if err != nil {
		return &NetworkFlowsReport{Available: false, Reason: err.Error(), CollectedAt: now}, nil
	}
	if flows == nil {
		// conntrackTimeout or ctx cancellation — listConntrackFlows already
		// logged nothing (no error to report either), just degrade quietly.
		return &NetworkFlowsReport{Available: false, Reason: "conntrack table read exceeded its collection budget", CollectedAt: now}, nil
	}

	raw := deltaFlows(flows)
	classified := classifyFlows(raw, localIPSet())
	if len(classified) == 0 {
		return &NetworkFlowsReport{Available: true, TopTalkers: []NetworkFlowTalker{}, CollectedAt: now}, nil
	}

	pa := newProcessAttribution(ctx)
	talkers := aggregateTalkers(classified, pa)

	sort.Slice(talkers, func(i, j int) bool {
		return talkers[i].RxBytes+talkers[i].TxBytes > talkers[j].RxBytes+talkers[j].TxBytes
	})

	total := len(talkers)
	var others *NetworkFlowBucket
	if total > topN {
		rest := talkers[topN:]
		talkers = talkers[:topN]
		b := &NetworkFlowBucket{}
		for _, t := range rest {
			b.Connections += t.Connections
			b.RxBytes += t.RxBytes
			b.TxBytes += t.TxBytes
		}
		others = b
	}

	return &NetworkFlowsReport{
		Available:   true,
		TopTalkers:  talkers,
		Others:      others,
		TotalFlows:  total,
		CollectedAt: now,
	}, nil
}

// listConntrackFlows reads both address families with a hard time budget.
// Returns (nil, nil) on timeout/cancellation — the caller treats that the
// same as an explicit "unavailable" rather than a Go error, since it isn't
// one: the kernel/agent are fine, this cycle just ran out of budget.
func listConntrackFlows() ([]*netlink.ConntrackFlow, error) {
	type result struct {
		flows []*netlink.ConntrackFlow
		err   error
	}
	resCh := make(chan result, 1)
	go func() {
		var all []*netlink.ConntrackFlow
		for _, family := range []netlink.InetFamily{unix.AF_INET, unix.AF_INET6} {
			flows, err := netlink.ConntrackTableList(netlink.ConntrackTable, family)
			if err != nil {
				resCh <- result{err: fmt.Errorf("conntrack table list (family %d): %w", family, err)}
				return
			}
			all = append(all, flows...)
		}
		resCh <- result{flows: all}
	}()

	select {
	case r := <-resCh:
		return r.flows, r.err
	case <-time.After(conntrackTimeout):
		return nil, nil
	}
}

// deltaFlows converts this cycle's cumulative conntrack counters into
// per-cycle deltas, using prevFlows as the previous baseline. A counter lower
// than its previous value (conntrack entry reused/reset) is treated as a
// fresh delta rather than producing a negative number — same guard shape as
// system.go's getCPUUsage(). Connections no longer present this cycle are
// dropped from prevFlows so it doesn't grow unbounded.
func deltaFlows(flows []*netlink.ConntrackFlow) []rawDelta {
	prevFlowsMu.Lock()
	defer prevFlowsMu.Unlock()

	seen := make(map[flowKey]struct{}, len(flows))
	result := make([]rawDelta, 0, len(flows))
	for _, f := range flows {
		key := flowKey{
			protocol: f.Forward.Protocol,
			srcIP:    f.Forward.SrcIP.String(),
			dstIP:    f.Forward.DstIP.String(),
			srcPort:  f.Forward.SrcPort,
			dstPort:  f.Forward.DstPort,
		}
		seen[key] = struct{}{}

		curFwd, curRev, curPkt := f.Forward.Bytes, f.Reverse.Bytes, f.Forward.Packets+f.Reverse.Packets

		var dFwd, dRev, dPkt uint64
		if prev, ok := prevFlows[key]; ok {
			dFwd = deltaOrReset(curFwd, prev.bytesFwd)
			dRev = deltaOrReset(curRev, prev.bytesRev)
			dPkt = deltaOrReset(curPkt, prev.packets)
		} else {
			dFwd, dRev, dPkt = curFwd, curRev, curPkt
		}
		prevFlows[key] = flowCounters{bytesFwd: curFwd, bytesRev: curRev, packets: curPkt}

		if dFwd == 0 && dRev == 0 {
			continue
		}
		result = append(result, rawDelta{
			protocol: f.Forward.Protocol,
			srcIP:    f.Forward.SrcIP,
			dstIP:    f.Forward.DstIP,
			srcPort:  f.Forward.SrcPort,
			dstPort:  f.Forward.DstPort,
			fwdBytes: dFwd,
			revBytes: dRev,
			packets:  dPkt,
		})
	}

	for k := range prevFlows {
		if _, ok := seen[k]; !ok {
			delete(prevFlows, k)
		}
	}
	return result
}

func deltaOrReset(cur, prev uint64) uint64 {
	if cur >= prev {
		return cur - prev
	}
	return cur
}

// rawDelta is one conntrack connection's per-cycle delta, still in its raw
// (unclassified) Forward-tuple orientation — classifyFlows resolves which
// side is local before this becomes rx/tx.
type rawDelta struct {
	protocol         uint8
	srcIP, dstIP     net.IP
	srcPort, dstPort uint16
	fwdBytes         uint64
	revBytes         uint64
	packets          uint64
}

type classifiedFlow struct {
	protocol   string
	localIP    string
	localPort  int
	remoteIP   string
	remotePort int
	direction  string // inbound|outbound
	rxBytes    uint64
	txBytes    uint64
	packets    uint64
}

// classifyFlows resolves each raw delta into a local/remote + direction,
// dropping anything where neither (or both) side of the connection is a
// local IP — forwarded/NAT traffic, out of scope for v1 (see package doc).
func classifyFlows(raws []rawDelta, localIPs map[string]struct{}) []classifiedFlow {
	result := make([]classifiedFlow, 0, len(raws))
	for _, r := range raws {
		_, srcLocal := localIPs[r.srcIP.String()]
		_, dstLocal := localIPs[r.dstIP.String()]

		var cf classifiedFlow
		switch {
		case srcLocal && !dstLocal:
			cf = classifiedFlow{
				localIP: r.srcIP.String(), localPort: int(r.srcPort),
				remoteIP: r.dstIP.String(), remotePort: int(r.dstPort),
				direction: "outbound",
				txBytes:   r.fwdBytes, rxBytes: r.revBytes,
			}
		case dstLocal && !srcLocal:
			cf = classifiedFlow{
				localIP: r.dstIP.String(), localPort: int(r.dstPort),
				remoteIP: r.srcIP.String(), remotePort: int(r.srcPort),
				direction: "inbound",
				rxBytes:   r.fwdBytes, txBytes: r.revBytes,
			}
		default:
			continue
		}
		cf.protocol = protocolName(r.protocol)
		cf.packets = r.packets
		result = append(result, cf)
	}
	return result
}

func protocolName(p uint8) string {
	switch p {
	case unix.IPPROTO_TCP:
		return "tcp"
	case unix.IPPROTO_UDP:
		return "udp"
	default:
		return fmt.Sprintf("proto-%d", p)
	}
}

// localIPSet returns every IP address currently assigned to this host's
// interfaces, used to tell "local" from "remote" in a conntrack tuple.
func localIPSet() map[string]struct{} {
	set := map[string]struct{}{}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return set
	}
	for _, a := range addrs {
		var ip net.IP
		switch v := a.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip != nil {
			set[ip.String()] = struct{}{}
		}
	}
	return set
}

type talkerKey struct {
	remoteIP   string
	remotePort int
	protocol   string
	direction  string
}

// aggregateTalkers groups classified flows by remote peer + protocol +
// direction, summing bytes/packets/connections and attributing a best-effort
// process name from the first connection in the group that resolves one.
func aggregateTalkers(flows []classifiedFlow, pa processAttribution) []NetworkFlowTalker {
	agg := map[talkerKey]*NetworkFlowTalker{}
	order := make([]talkerKey, 0, len(flows))

	for _, f := range flows {
		k := talkerKey{remoteIP: f.remoteIP, remotePort: f.remotePort, protocol: f.protocol, direction: f.direction}
		t, ok := agg[k]
		if !ok {
			t = &NetworkFlowTalker{RemoteIP: f.remoteIP, RemotePort: f.remotePort, Protocol: f.protocol, Direction: f.direction}
			agg[k] = t
			order = append(order, k)
		}
		t.RxBytes += f.rxBytes
		t.TxBytes += f.txBytes
		t.Packets += f.packets
		t.Connections++
		if t.ProcessName == "" {
			if name, pid := pa.attribute(f.protocol, f.localIP, f.localPort, f.remoteIP, f.remotePort); name != "" {
				t.ProcessName = name
				t.PID = pid
			}
		}
	}

	talkers := make([]NetworkFlowTalker, 0, len(order))
	for _, k := range order {
		talkers = append(talkers, *agg[k])
	}
	return talkers
}

// ---- Process attribution ("poor man's netstat"): /proc/net/{tcp,udp}[6] for
// the local+remote 4-tuple → inode, /proc/*/fd for inode → pid, /proc/<pid>/comm
// for pid → name. Every step is best-effort: a failure at any point leaves the
// talker's ProcessName empty rather than failing the whole collection cycle —
// ephemeral/closed sockets routinely miss this window between the conntrack
// read and the /proc read.

type socketKey struct {
	protocol              string
	localIP, remoteIP     string
	localPort, remotePort int
}

type processAttribution struct {
	tcpSockets map[socketKey]uint64
	udpSockets map[socketKey]uint64
	inodeToPID map[uint64]int
}

func newProcessAttribution(ctx context.Context) processAttribution {
	pa := processAttribution{
		tcpSockets: map[socketKey]uint64{},
		udpSockets: map[socketKey]uint64{},
	}
	mergeSocketTable(pa.tcpSockets, "/proc/net/tcp", "tcp")
	mergeSocketTable(pa.tcpSockets, "/proc/net/tcp6", "tcp")
	mergeSocketTable(pa.udpSockets, "/proc/net/udp", "udp")
	mergeSocketTable(pa.udpSockets, "/proc/net/udp6", "udp")
	pa.inodeToPID = buildInodeToPID(ctx)
	return pa
}

func (pa processAttribution) attribute(protocol, localIP string, localPort int, remoteIP string, remotePort int) (name string, pid int) {
	table := pa.tcpSockets
	if protocol == "udp" {
		table = pa.udpSockets
	}
	inode, ok := table[socketKey{protocol, localIP, remoteIP, localPort, remotePort}]
	if !ok {
		return "", 0
	}
	p, ok := pa.inodeToPID[inode]
	if !ok {
		return "", 0
	}
	return processCommName(p), p
}

func mergeSocketTable(dst map[socketKey]uint64, path, protocol string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // header or trailing blank line
		}
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		localIP, localPort, ok1 := parseHexAddr(fields[1])
		remoteIP, remotePort, ok2 := parseHexAddr(fields[2])
		if !ok1 || !ok2 {
			continue
		}
		inode, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			continue
		}
		dst[socketKey{protocol, localIP, remoteIP, localPort, remotePort}] = inode
	}
}

// parseHexAddr parses a /proc/net/{tcp,udp}* "IP:PORT" field (hex-encoded,
// e.g. "0100007F:1770") into a normalized IP string and port.
func parseHexAddr(s string) (ip string, port int, ok bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return "", 0, false
	}
	parsedIP := hexToIP(parts[0])
	if parsedIP == nil {
		return "", 0, false
	}
	p, err := strconv.ParseUint(parts[1], 16, 16)
	if err != nil {
		return "", 0, false
	}
	return parsedIP.String(), int(p), true
}

// hexToIP decodes /proc/net/tcp*'s address encoding: 4 bytes for IPv4 stored
// as one little-endian 32-bit word, 16 bytes for IPv6 stored as four
// little-endian 32-bit words — each word's bytes are reversed independently.
func hexToIP(s string) net.IP {
	raw, err := decodeHex(s)
	if err != nil {
		return nil
	}
	switch len(raw) {
	case 4:
		return net.IPv4(raw[3], raw[2], raw[1], raw[0])
	case 16:
		ip := make(net.IP, 16)
		for word := 0; word < 4; word++ {
			ip[word*4+0] = raw[word*4+3]
			ip[word*4+1] = raw[word*4+2]
			ip[word*4+2] = raw[word*4+1]
			ip[word*4+3] = raw[word*4+0]
		}
		return ip
	default:
		return nil
	}
}

func decodeHex(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("odd-length hex string")
	}
	out := make([]byte, len(s)/2)
	for i := range out {
		v, err := strconv.ParseUint(s[i*2:i*2+2], 16, 8)
		if err != nil {
			return nil, err
		}
		out[i] = byte(v)
	}
	return out, nil
}

var socketFDRe = regexp.MustCompile(`^socket:\[(\d+)\]$`)

// buildInodeToPID walks /proc/*/fd to map socket inodes to owning PIDs.
// Bounded by ctx: on a host with very many processes/fds, it bails early with
// whatever it's found so far rather than risking the collector's time budget
// — a partial attribution table is always safe (see the caller's best-effort
// contract above), an unbounded /proc walk isn't.
func buildInodeToPID(ctx context.Context) map[uint64]int {
	result := map[uint64]int{}
	procEntries, err := os.ReadDir("/proc")
	if err != nil {
		return result
	}
	for i, entry := range procEntries {
		if i%50 == 0 && ctx.Err() != nil {
			return result
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || !entry.IsDir() {
			continue
		}
		fdDir := filepath.Join("/proc", entry.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue // process exited mid-scan, or no permission — skip, not fatal
		}
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			if m := socketFDRe.FindStringSubmatch(link); m != nil {
				if inode, err := strconv.ParseUint(m[1], 10, 64); err == nil {
					result[inode] = pid
				}
			}
		}
	}
	return result
}

func processCommName(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
