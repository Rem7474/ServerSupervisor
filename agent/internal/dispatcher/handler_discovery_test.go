package dispatcher

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/serversupervisor/agent/internal/config"
	"github.com/serversupervisor/agent/internal/netdiscovery"
	"github.com/serversupervisor/agent/internal/sender"
)

func discoveryFixture(t *testing.T, allowScan bool) (*Dispatcher, *sender.Sender, func() []map[string]any) {
	t.Helper()
	srv, results := fakeAgentServer(t)
	cfg := &config.Config{ServerURL: srv.URL, APIKey: "k", AllowNetworkScan: allowScan}
	return &Dispatcher{cfg: cfg}, sender.New(cfg), results
}

func runDiscovery(t *testing.T, d *Dispatcher, s *sender.Sender, action, payload string, results func() []map[string]any) map[string]any {
	t.Helper()
	handleDiscovery(context.Background(), d, s, sender.PendingCommand{ID: "cmd-1", Module: "discovery", Action: action, Payload: payload})
	return lastNonRunningResult(t, results())
}

func TestHandleDiscoveryARP(t *testing.T) {
	orig := readARPTable
	t.Cleanup(func() { readARPTable = orig })
	readARPTable = func() (*netdiscovery.ARPResult, error) {
		return &netdiscovery.ARPResult{Entries: []netdiscovery.ARPEntry{{IP: "192.168.1.1", MAC: "aa:bb:cc:dd:ee:ff", Interface: "eth0"}}}, nil
	}
	// ARP is passive: it works even when active scanning is not allowed.
	d, s, results := discoveryFixture(t, false)
	res := runDiscovery(t, d, s, "arp", "{}", results)
	if res["status"] != "completed" {
		t.Fatalf("result = %+v", res)
	}
	var out netdiscovery.ARPResult
	if err := json.Unmarshal([]byte(res["output"].(string)), &out); err != nil || len(out.Entries) != 1 || out.Entries[0].MAC != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("output = %v (%v)", res["output"], err)
	}

	readARPTable = func() (*netdiscovery.ARPResult, error) { return nil, errors.New("cannot read the ARP table: denied") }
	res = runDiscovery(t, d, s, "arp", "{}", results)
	if res["status"] != "failed" || !strings.Contains(res["output"].(string), "denied") {
		t.Fatalf("failure result = %+v", res)
	}
}

func TestHandleDiscoveryActiveProbesRequireOptIn(t *testing.T) {
	d, s, results := discoveryFixture(t, false)
	for _, action := range []string{"port_scan", "snmp"} {
		res := runDiscovery(t, d, s, action, `{"cidr":"192.168.1.0/30","ports":[22]}`, results)
		if res["status"] != "failed" || !strings.Contains(res["output"].(string), "allow_network_scan") {
			t.Fatalf("%s result = %+v", action, res)
		}
	}
}

func TestHandleDiscoveryUnknownActionAndBadPayloads(t *testing.T) {
	d, s, results := discoveryFixture(t, true)
	cases := map[string][2]string{
		"unknown":   {"whatever", "{}"},
		"port_scan": {"port_scan", "not json"},
		"snmp":      {"snmp", "not json"},
		"public":    {"port_scan", `{"cidr":"8.8.8.0/24","ports":[22]}`},
	}
	for name, c := range cases {
		res := runDiscovery(t, d, s, c[0], c[1], results)
		if res["status"] != "failed" || !strings.HasPrefix(res["output"].(string), "ERROR: ") {
			t.Errorf("%s: result = %+v", name, res)
		}
	}
}

func TestHandleDiscoveryPortScanCancelledByServer(t *testing.T) {
	origDial, origProgress, origInterval := scanDialer, progressReporter, netdiscovery.ProgressInterval
	t.Cleanup(func() {
		scanDialer, progressReporter, netdiscovery.ProgressInterval = origDial, origProgress, origInterval
	})
	netdiscovery.ProgressInterval = 10 * time.Millisecond
	scanDialer = func(ctx context.Context, _, _ string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	var reports atomic.Int64
	progressReporter = func(context.Context, *sender.Sender, string, string) bool {
		return reports.Add(1) >= 2 // the server answers "cancel" on the second chunk
	}

	d, s, results := discoveryFixture(t, true)
	res := runDiscovery(t, d, s, "port_scan", `{"cidr":"192.168.1.0/24","ports":[22,80],"concurrency":1,"timeout_ms":5000}`, results)
	if res["status"] != "completed" {
		t.Fatalf("result = %+v", res)
	}
	var out netdiscovery.ScanResult
	if err := json.Unmarshal([]byte(res["output"].(string)), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Cancelled || out.ProbesDone >= out.ProbesTotal {
		t.Fatalf("expected a cancelled partial scan, got %+v", out)
	}
}

type stubSNMP struct{}

func (stubSNMP) Connect() error { return nil }
func (stubSNMP) Get([]string) (*gosnmp.SnmpPacket, error) {
	return &gosnmp.SnmpPacket{Variables: []gosnmp.SnmpPDU{{Name: ".1.3.6.1.2.1.1.5.0", Type: gosnmp.OctetString, Value: []byte("nas")}}}, nil
}
func (stubSNMP) BulkWalkAll(string) ([]gosnmp.SnmpPDU, error) { return nil, nil }
func (stubSNMP) Close() error                                 { return nil }

func TestHandleDiscoverySNMP(t *testing.T) {
	orig := snmpFactory
	t.Cleanup(func() { snmpFactory = orig })
	snmpFactory = func(*gosnmp.GoSNMP) netdiscovery.SNMPSession { return stubSNMP{} }

	d, s, results := discoveryFixture(t, true)
	res := runDiscovery(t, d, s, "snmp", `{"target":"192.168.1.9","version":"2c","secrets":{"community":"hidden-value"}}`, results)
	if res["status"] != "completed" {
		t.Fatalf("result = %+v", res)
	}
	output := res["output"].(string)
	if !strings.Contains(output, `"sys_name":"nas"`) || strings.Contains(output, "hidden-value") {
		t.Fatalf("output = %s", output)
	}
}

// Discovery commands run on their own worker, one at a time: Process returns
// without waiting for them (the agent's other commands are not held back),
// and a command beyond the queue bound is failed instead of waiting.
func TestProcessRunsDiscoveryOffTheCommandLoop(t *testing.T) {
	orig := readARPTable
	t.Cleanup(func() { readARPTable = orig })
	release := make(chan struct{})
	started := make(chan struct{}, 8)
	var running, maxRunning atomic.Int32
	readARPTable = func() (*netdiscovery.ARPResult, error) {
		n := running.Add(1)
		if n > maxRunning.Load() {
			maxRunning.Store(n)
		}
		started <- struct{}{}
		<-release
		running.Add(-1)
		return &netdiscovery.ARPResult{}, nil
	}
	d, s, results := discoveryFixture(t, false)

	done := make(chan struct{})
	go func() {
		cmds := []sender.PendingCommand{}
		for i := range discoveryQueueSize + 2 { // 1 running + queue + 1 refused
			cmds = append(cmds, sender.PendingCommand{ID: "d" + string(rune('0'+i)), Module: "discovery", Action: "arp"})
		}
		d.Process(s, cmds)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Process waited for the discovery job")
	}
	<-started

	refused := func() int {
		n := 0
		for _, r := range results() {
			if r["status"] == "failed" && strings.Contains(r["output"].(string), "already running") {
				n++
			}
		}
		return n
	}
	deadline := time.Now().Add(5 * time.Second)
	for refused() != 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if refused() != 1 {
		t.Fatalf("results = %v", results())
	}

	close(release)
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		completed := 0
		for _, r := range results() {
			if r["status"] == "completed" {
				completed++
			}
		}
		if completed == discoveryQueueSize+1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if maxRunning.Load() != 1 {
		t.Fatalf("%d discovery jobs ran at once", maxRunning.Load())
	}
}
