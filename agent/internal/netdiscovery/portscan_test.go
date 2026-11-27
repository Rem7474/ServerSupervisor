package netdiscovery

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeDialer answers "open" for the addresses in open and refuses the rest,
// recording every address it was asked to dial.
type fakeDialer struct {
	mu     sync.Mutex
	open   map[string]bool
	dialed []string
}

func (f *fakeDialer) dial(_ context.Context, network, address string) (net.Conn, error) {
	f.mu.Lock()
	f.dialed = append(f.dialed, address)
	f.mu.Unlock()
	if network != "tcp" {
		return nil, errors.New("unexpected network " + network)
	}
	if f.open[address] {
		a, b := net.Pipe()
		_ = b.Close()
		return a, nil
	}
	return nil, errors.New("connection refused")
}

func TestScanFindsOpenPorts(t *testing.T) {
	fd := &fakeDialer{open: map[string]bool{
		"192.168.10.2:443": true,
		"192.168.10.1:22":  true,
		"192.168.10.2:22":  true,
	}}
	res, err := Scan(context.Background(), ScanRequest{
		CIDR: "192.168.10.0/30", Ports: []int{443, 22, 80}, RatePerSecond: MaxRatePerSec,
	}, fd.dial, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.HostsScanned != 2 || res.PortsScanned != 3 || res.ProbesTotal != 6 || res.ProbesDone != 6 {
		t.Fatalf("counters = %+v", res)
	}
	if res.Cancelled || res.TimedOut {
		t.Fatalf("unexpected partial result: %+v", res)
	}
	if len(res.Hosts) != 2 ||
		res.Hosts[0].IP != "192.168.10.1" || len(res.Hosts[0].OpenPorts) != 1 || res.Hosts[0].OpenPorts[0] != 22 ||
		res.Hosts[1].IP != "192.168.10.2" || len(res.Hosts[1].OpenPorts) != 2 || res.Hosts[1].OpenPorts[0] != 22 || res.Hosts[1].OpenPorts[1] != 443 {
		t.Fatalf("hosts = %+v", res.Hosts)
	}
	if len(fd.dialed) != 6 {
		t.Fatalf("dialed %d addresses, want 6", len(fd.dialed))
	}
}

func TestScanRejectsInvalidRequests(t *testing.T) {
	for _, req := range []ScanRequest{
		{CIDR: "8.8.8.0/24", Ports: []int{22}},
		{CIDR: "192.168.0.0/16", Ports: []int{22}},
		{CIDR: "192.168.1.0/24", Ports: nil},
		{CIDR: "192.168.1.0/24", Ports: []int{70000}},
	} {
		if _, err := Scan(context.Background(), req, func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("must not dial for an invalid request")
			return nil, nil
		}, nil); err == nil {
			t.Errorf("Scan(%+v) should fail", req)
		}
	}
}

func TestScanCancellationReturnsPartialResult(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var dials atomic.Int64
	dial := func(dctx context.Context, _, _ string) (net.Conn, error) {
		if dials.Add(1) == 3 {
			cancel()
		}
		<-dctx.Done()
		return nil, dctx.Err()
	}
	res, err := Scan(ctx, ScanRequest{
		CIDR: "192.168.20.0/24", Ports: []int{22, 80, 443}, Concurrency: 2, RatePerSecond: MaxRatePerSec, TimeoutMs: 5000,
	}, dial, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Cancelled || res.TimedOut {
		t.Fatalf("expected a cancelled result, got %+v", res)
	}
	if res.ProbesDone >= res.ProbesTotal || res.ProbesTotal != 254*3 {
		t.Fatalf("probes %d/%d", res.ProbesDone, res.ProbesTotal)
	}
	if len(res.Hosts) != 0 {
		t.Fatalf("no port should be open: %+v", res.Hosts)
	}
}

func TestScanStopsAtMaxDuration(t *testing.T) {
	orig := MaxScanDuration
	MaxScanDuration = 150 * time.Millisecond
	t.Cleanup(func() { MaxScanDuration = orig })

	dial := func(dctx context.Context, _, _ string) (net.Conn, error) {
		<-dctx.Done()
		return nil, dctx.Err()
	}
	res, err := Scan(context.Background(), ScanRequest{
		CIDR: "10.0.0.0/24", Ports: []int{1, 2, 3}, Concurrency: 1, TimeoutMs: 5000,
	}, dial, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.TimedOut || res.Cancelled {
		t.Fatalf("expected timed_out, got %+v", res)
	}
}

func TestScanReportsProgress(t *testing.T) {
	orig := ProgressInterval
	ProgressInterval = 10 * time.Millisecond
	t.Cleanup(func() { ProgressInterval = orig })

	var mu sync.Mutex
	var seen []Progress
	dial := func(context.Context, string, string) (net.Conn, error) {
		time.Sleep(5 * time.Millisecond)
		return nil, errors.New("refused")
	}
	_, err := Scan(context.Background(), ScanRequest{
		CIDR: "172.16.0.0/28", Ports: []int{22, 80}, Concurrency: 1, RatePerSecond: MaxRatePerSec,
	}, dial, func(p Progress) {
		mu.Lock()
		seen = append(seen, p)
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) == 0 {
		t.Fatal("expected at least one progress report")
	}
	for _, p := range seen {
		if p.Total != 14*2 || p.Done > p.Total {
			t.Fatalf("bad progress %+v", p)
		}
	}
}

func TestScanPlanClampsKnobs(t *testing.T) {
	p, err := ScanRequest{CIDR: "10.0.0.1/32", Ports: []int{22}}.plan()
	if err != nil {
		t.Fatal(err)
	}
	if p.timeout != DefaultProbeTimeout || p.concurrency != DefaultConcurrency || p.rate != DefaultRatePerSec {
		t.Fatalf("defaults = %+v", p)
	}
	p, _ = ScanRequest{CIDR: "10.0.0.1/32", Ports: []int{22}, TimeoutMs: 1, Concurrency: 100000, RatePerSecond: 100000}.plan()
	if p.timeout != MinProbeTimeout || p.concurrency != MaxConcurrency || p.rate != MaxRatePerSec {
		t.Fatalf("clamped = %+v", p)
	}
	p, _ = ScanRequest{CIDR: "10.0.0.1/32", Ports: []int{22}, TimeoutMs: 60000}.plan()
	if p.timeout != MaxProbeTimeout {
		t.Fatalf("max timeout = %v", p.timeout)
	}
}

func TestScanUsesRealDialerByDefault(t *testing.T) {
	// A /32 on a private address nobody listens on, with the shortest
	// timeout: exercises the nil-dialer default without depending on the LAN.
	res, err := Scan(context.Background(), ScanRequest{CIDR: "10.255.255.1/32", Ports: []int{9}, TimeoutMs: 100}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.ProbesDone != 1 {
		t.Fatalf("probes done = %d", res.ProbesDone)
	}
}

func TestSortedHostsBoundsResult(t *testing.T) {
	many := func(n int) []int {
		out := make([]int, n)
		for i := range out {
			out[i] = n - i // reversed: sortedHosts sorts before cutting
		}
		return out
	}
	hosts, truncated := sortedHosts(map[netip.Addr][]int{
		netip.MustParseAddr("10.0.0.2"): {443, 22},
		netip.MustParseAddr("10.0.0.1"): {80},
	})
	if truncated || len(hosts) != 2 || hosts[0].IP != "10.0.0.1" || hosts[1].OpenPorts[0] != 22 {
		t.Fatalf("small result = %+v %v", hosts, truncated)
	}

	// Per-host cap: lowest ports kept.
	hosts, truncated = sortedHosts(map[netip.Addr][]int{netip.MustParseAddr("10.0.0.1"): many(MaxOpenPortsPerHost + 10)})
	if !truncated || len(hosts[0].OpenPorts) != MaxOpenPortsPerHost || hosts[0].OpenPorts[0] != 1 {
		t.Fatalf("per-host cap: %d ports, truncated=%v", len(hosts[0].OpenPorts), truncated)
	}

	// Total cap: lowest addresses kept, the rest dropped.
	open := map[netip.Addr][]int{}
	nHosts := MaxOpenPortsTotal/MaxOpenPortsPerHost + 2
	for i := range nHosts {
		open[netip.AddrFrom4([4]byte{10, 0, 1, byte(i + 1)})] = many(MaxOpenPortsPerHost)
	}
	hosts, truncated = sortedHosts(open)
	total := 0
	for _, h := range hosts {
		total += len(h.OpenPorts)
	}
	if !truncated || total != MaxOpenPortsTotal || hosts[0].IP != "10.0.1.1" || len(hosts) != MaxOpenPortsTotal/MaxOpenPortsPerHost {
		t.Fatalf("total cap: %d ports over %d hosts, truncated=%v", total, len(hosts), truncated)
	}
}
