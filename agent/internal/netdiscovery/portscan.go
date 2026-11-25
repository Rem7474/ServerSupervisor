package netdiscovery

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Port-scan bounds enforced on the agent. Requests outside them are clamped
// (timeout, concurrency, rate) or refused (range, ports).
const (
	DefaultProbeTimeout = 800 * time.Millisecond
	MinProbeTimeout     = 100 * time.Millisecond
	MaxProbeTimeout     = 5 * time.Second
	DefaultConcurrency  = 64
	MaxConcurrency      = 256
	DefaultRatePerSec   = 500
	MaxRatePerSec       = 2000
)

// Vars rather than consts so tests can shorten them.
var (
	// MaxScanDuration stops a scan that would otherwise run for too long;
	// the result is reported as partial (timed_out).
	MaxScanDuration = 20 * time.Minute
	// ProgressInterval is how often Scan reports progress.
	ProgressInterval = 2 * time.Second
)

// ScanRequest is the discovery/port_scan command payload.
type ScanRequest struct {
	CIDR          string `json:"cidr"`
	Ports         []int  `json:"ports"`
	TimeoutMs     int    `json:"timeout_ms,omitempty"`
	Concurrency   int    `json:"concurrency,omitempty"`
	RatePerSecond int    `json:"rate_per_second,omitempty"`
}

// ScanHost is one address with at least one open port.
type ScanHost struct {
	IP        string `json:"ip"`
	OpenPorts []int  `json:"open_ports"`
}

// ScanResult is the discovery/port_scan command output. Hosts lists only
// addresses with at least one open port.
type ScanResult struct {
	CIDR         string     `json:"cidr"`
	HostsScanned int        `json:"hosts_scanned"`
	PortsScanned int        `json:"ports_scanned"`
	ProbesDone   int        `json:"probes_done"`
	ProbesTotal  int        `json:"probes_total"`
	Cancelled    bool       `json:"cancelled"`
	TimedOut     bool       `json:"timed_out"`
	DurationMs   int64      `json:"duration_ms"`
	Hosts        []ScanHost `json:"hosts"`
}

// Progress is a periodic scan progress snapshot.
type Progress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
	Open  int `json:"open"`
}

// DialFunc opens a TCP connection; net.Dialer.DialContext satisfies it.
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

type scanPlan struct {
	addrs       []netip.Addr
	ports       []int
	timeout     time.Duration
	concurrency int
	rate        int
}

func (r ScanRequest) plan() (*scanPlan, error) {
	addrs, err := ExpandCIDR(r.CIDR)
	if err != nil {
		return nil, err
	}
	ports, err := NormalizePorts(r.Ports)
	if err != nil {
		return nil, err
	}
	p := &scanPlan{addrs: addrs, ports: ports}
	p.timeout = clampDuration(time.Duration(r.TimeoutMs)*time.Millisecond, DefaultProbeTimeout, MinProbeTimeout, MaxProbeTimeout)
	p.concurrency = clampInt(r.Concurrency, DefaultConcurrency, MaxConcurrency)
	p.rate = clampInt(r.RatePerSecond, DefaultRatePerSec, MaxRatePerSec)
	return p, nil
}

func clampInt(v, def, maxV int) int {
	if v <= 0 {
		return def
	}
	return min(v, maxV)
}

func clampDuration(v, def, minV, maxV time.Duration) time.Duration {
	if v <= 0 {
		return def
	}
	return max(minV, min(v, maxV))
}

type probe struct {
	addr netip.Addr
	port int
}

// Scan runs a TCP connect scan (full handshake, no raw socket) over the
// request's range and ports, bounded in concurrency, rate and duration.
// Cancelling ctx stops it early: the partial result is returned with
// Cancelled set. progress, when non-nil, is called every ProgressInterval
// from a single goroutine.
func Scan(ctx context.Context, req ScanRequest, dial DialFunc, progress func(Progress)) (*ScanResult, error) {
	p, err := req.plan()
	if err != nil {
		return nil, err
	}
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	start := time.Now()
	scanCtx, cancel := context.WithTimeout(ctx, MaxScanDuration)
	defer cancel()

	total := len(p.addrs) * len(p.ports)
	var done, openCount atomic.Int64
	var mu sync.Mutex
	open := map[netip.Addr][]int{}

	probes := make(chan probe)
	go produceProbes(scanCtx, p, probes)

	var wg sync.WaitGroup
	for range p.concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pr := range probes {
				if probeOpen(scanCtx, dial, pr, p.timeout) {
					mu.Lock()
					open[pr.addr] = append(open[pr.addr], pr.port)
					mu.Unlock()
					openCount.Add(1)
				}
				done.Add(1)
			}
		}()
	}

	stopProgress := make(chan struct{})
	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		if progress == nil {
			return
		}
		t := time.NewTicker(ProgressInterval)
		defer t.Stop()
		for {
			select {
			case <-stopProgress:
				return
			case <-t.C:
				progress(Progress{Done: int(done.Load()), Total: total, Open: int(openCount.Load())})
			}
		}
	}()

	wg.Wait()
	close(stopProgress)
	<-progressDone

	res := &ScanResult{
		CIDR:         req.CIDR,
		HostsScanned: len(p.addrs),
		PortsScanned: len(p.ports),
		ProbesDone:   int(done.Load()),
		ProbesTotal:  total,
		DurationMs:   time.Since(start).Milliseconds(),
		Hosts:        sortedHosts(open),
	}
	if res.ProbesDone < total {
		if ctx.Err() != nil {
			res.Cancelled = true
		} else {
			res.TimedOut = true
		}
	}
	return res, nil
}

// produceProbes feeds every (address, port) pair at the configured rate and
// stops as soon as ctx ends.
func produceProbes(ctx context.Context, p *scanPlan, out chan<- probe) {
	defer close(out)
	tick := time.NewTicker(time.Second / time.Duration(p.rate))
	defer tick.Stop()
	for _, addr := range p.addrs {
		for _, port := range p.ports {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
			select {
			case <-ctx.Done():
				return
			case out <- probe{addr: addr, port: port}:
			}
		}
	}
}

func probeOpen(ctx context.Context, dial DialFunc, pr probe, timeout time.Duration) bool {
	if ctx.Err() != nil {
		return false
	}
	dctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := dial(dctx, "tcp", net.JoinHostPort(pr.addr.String(), strconv.Itoa(pr.port)))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func sortedHosts(open map[netip.Addr][]int) []ScanHost {
	addrs := make([]netip.Addr, 0, len(open))
	for a := range open {
		addrs = append(addrs, a)
	}
	sort.Slice(addrs, func(i, j int) bool { return addrs[i].Less(addrs[j]) })
	hosts := make([]ScanHost, 0, len(addrs))
	for _, a := range addrs {
		ports := open[a]
		sort.Ints(ports)
		hosts = append(hosts, ScanHost{IP: a.String(), OpenPorts: ports})
	}
	return hosts
}

// ErrScanDisabled is returned when the agent's configuration does not allow
// active probes (port scan, SNMP).
var ErrScanDisabled = errors.New("active network scanning is disabled on this agent (set allow_network_scan: true in its configuration)")
