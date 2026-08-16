//go:build linux

package collector

import (
	"net"
	"testing"

	"golang.org/x/sys/unix"
)

func ipSet(ips ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(ips))
	for _, ip := range ips {
		set[ip] = struct{}{}
	}
	return set
}

// TestClassifyFlowsDockerLocalSet is the regression guard for the bug this
// feature fixes: with only host interface addresses in the "local" set, a
// container's SNAT'd outbound flow has neither side local and gets dropped, so
// Docker traffic is entirely invisible.
func TestClassifyFlowsDockerLocalSet(t *testing.T) {
	hostOnly := ipSet("192.168.1.10", "172.17.0.1")
	withContainers := ipSet("192.168.1.10", "172.17.0.1", "172.17.0.4", "172.18.0.2")

	containerOutbound := rawDelta{
		protocol: unix.IPPROTO_TCP,
		srcIP:    net.ParseIP("172.17.0.4"), srcPort: 51000,
		dstIP: net.ParseIP("140.82.121.4"), dstPort: 443,
		fwdBytes: 900, revBytes: 12000, packets: 30,
	}

	if got := classifyFlows([]rawDelta{containerOutbound}, hostOnly); len(got) != 0 {
		t.Fatalf("baseline: container flow should be dropped with a host-only local set, got %+v", got)
	}

	got := classifyFlows([]rawDelta{containerOutbound}, withContainers)
	if len(got) != 1 {
		t.Fatalf("expected the container flow to classify once container IPs are local, got %d", len(got))
	}
	f := got[0]
	if f.direction != "outbound" {
		t.Errorf("direction = %q, want outbound", f.direction)
	}
	if f.localIP != "172.17.0.4" || f.remoteIP != "140.82.121.4" || f.remotePort != 443 {
		t.Errorf("local/remote resolved wrong: %+v", f)
	}
	if f.txBytes != 900 || f.rxBytes != 12000 {
		t.Errorf("rx/tx = %d/%d, want 12000/900", f.rxBytes, f.txBytes)
	}
	if f.protocol != "tcp" {
		t.Errorf("protocol = %q, want tcp", f.protocol)
	}
}

func TestClassifyFlowsScopeIsNotOverBroadened(t *testing.T) {
	// Both sides local (container ↔ container on the same bridge) is not "this
	// host talking to a remote peer" — it must stay excluded, exactly like the
	// both-host-IPs loopback case always was.
	local := ipSet("192.168.1.10", "172.17.0.4", "172.17.0.5")

	cases := []struct {
		name string
		raw  rawDelta
	}{
		{
			name: "container to container on the same bridge",
			raw: rawDelta{
				protocol: unix.IPPROTO_TCP,
				srcIP:    net.ParseIP("172.17.0.4"), srcPort: 40000,
				dstIP: net.ParseIP("172.17.0.5"), dstPort: 5432,
				fwdBytes: 100, revBytes: 200,
			},
		},
		{
			name: "genuinely routed traffic with neither side on this host",
			raw: rawDelta{
				protocol: unix.IPPROTO_TCP,
				srcIP:    net.ParseIP("10.9.9.9"), srcPort: 40000,
				dstIP: net.ParseIP("1.1.1.1"), dstPort: 443,
				fwdBytes: 100, revBytes: 200,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyFlows([]rawDelta{tc.raw}, local); len(got) != 0 {
				t.Fatalf("expected the flow to be excluded, got %+v", got)
			}
		})
	}
}

func TestClassifyFlowsInboundUnaffected(t *testing.T) {
	local := ipSet("192.168.1.10", "172.17.0.4")
	raw := rawDelta{
		protocol: unix.IPPROTO_UDP,
		srcIP:    net.ParseIP("8.8.8.8"), srcPort: 53,
		dstIP: net.ParseIP("192.168.1.10"), dstPort: 44444,
		fwdBytes: 300, revBytes: 80, packets: 4,
	}
	got := classifyFlows([]rawDelta{raw}, local)
	if len(got) != 1 {
		t.Fatalf("expected 1 classified flow, got %d", len(got))
	}
	f := got[0]
	if f.direction != "inbound" || f.remoteIP != "8.8.8.8" || f.remotePort != 53 {
		t.Fatalf("inbound classification changed: %+v", f)
	}
	if f.rxBytes != 300 || f.txBytes != 80 {
		t.Errorf("rx/tx = %d/%d, want 300/80", f.rxBytes, f.txBytes)
	}
	if f.protocol != "udp" {
		t.Errorf("protocol = %q, want udp", f.protocol)
	}
}

// emptyAttribution stands in for a host whose /proc lookup resolves nothing —
// the normal outcome for a process living inside a container's namespace.
func emptyAttribution() processAttribution {
	return processAttribution{
		tcpSockets: map[socketKey]uint64{},
		udpSockets: map[socketKey]uint64{},
		inodeToPID: map[uint64]int{},
	}
}

func TestAggregateTalkersContainerFallback(t *testing.T) {
	flows := []classifiedFlow{{
		protocol: "tcp",
		localIP:  "172.17.0.4", localPort: 51000,
		remoteIP: "140.82.121.4", remotePort: 443,
		direction: "outbound", txBytes: 900, rxBytes: 12000, packets: 30,
	}}
	containerIPs := map[string]string{"172.17.0.4": "gitea"}

	talkers := aggregateTalkers(flows, emptyAttribution(), containerIPs)
	if len(talkers) != 1 {
		t.Fatalf("expected 1 talker, got %d", len(talkers))
	}
	if want := containerLabelPrefix + "gitea"; talkers[0].ProcessName != want {
		t.Errorf("ProcessName = %q, want %q", talkers[0].ProcessName, want)
	}
	// A container's PID is only meaningful inside its own namespace — reporting
	// one here would point at the wrong process on the host.
	if talkers[0].PID != 0 {
		t.Errorf("PID = %d, want 0 for a container-attributed talker", talkers[0].PID)
	}
}

func TestAggregateTalkersHostProcessWinsOverContainerFallback(t *testing.T) {
	const localIP, remoteIP = "192.168.1.10", "1.1.1.1"
	pa := emptyAttribution()
	pa.tcpSockets[socketKey{"tcp", localIP, remoteIP, 40000, 443}] = 4242
	pa.inodeToPID[4242] = 1
	// processCommName reads /proc/1/comm, which exists on any Linux CI runner;
	// the assertion below only needs "not the container label".

	flows := []classifiedFlow{{
		protocol: "tcp",
		localIP:  localIP, localPort: 40000,
		remoteIP: remoteIP, remotePort: 443,
		direction: "outbound", txBytes: 10, rxBytes: 20,
	}}
	// Deliberately claim the *same* local IP is a container's, to prove the
	// fallback never overrides a real host-process match.
	containerIPs := map[string]string{localIP: "should-not-win"}

	talkers := aggregateTalkers(flows, pa, containerIPs)
	if len(talkers) != 1 {
		t.Fatalf("expected 1 talker, got %d", len(talkers))
	}
	if talkers[0].ProcessName == containerLabelPrefix+"should-not-win" {
		t.Fatal("container fallback overrode a real /proc process match")
	}
	if talkers[0].PID != 1 {
		t.Errorf("PID = %d, want 1 (the real host-process match)", talkers[0].PID)
	}
}

func TestAggregateTalkersNoAttributionAtAll(t *testing.T) {
	flows := []classifiedFlow{{
		protocol: "tcp",
		localIP:  "192.168.1.10", localPort: 40000,
		remoteIP: "1.1.1.1", remotePort: 443,
		direction: "outbound", txBytes: 10, rxBytes: 20,
	}}
	talkers := aggregateTalkers(flows, emptyAttribution(), map[string]string{})
	if len(talkers) != 1 {
		t.Fatalf("expected 1 talker, got %d", len(talkers))
	}
	if talkers[0].ProcessName != "" {
		t.Errorf("ProcessName = %q, want empty when nothing resolves", talkers[0].ProcessName)
	}
}

func TestAggregateTalkersGroupsAndSums(t *testing.T) {
	mk := func(localIP string, localPort int) classifiedFlow {
		return classifiedFlow{
			protocol: "tcp",
			localIP:  localIP, localPort: localPort,
			remoteIP: "140.82.121.4", remotePort: 443,
			direction: "outbound", txBytes: 100, rxBytes: 1000, packets: 5,
		}
	}
	// Two connections from the same container to the same peer collapse into
	// one talker, and the container label is still applied.
	flows := []classifiedFlow{mk("172.17.0.4", 51000), mk("172.17.0.4", 51001)}
	talkers := aggregateTalkers(flows, emptyAttribution(), map[string]string{"172.17.0.4": "gitea"})

	if len(talkers) != 1 {
		t.Fatalf("expected the two connections to aggregate into 1 talker, got %d", len(talkers))
	}
	got := talkers[0]
	if got.Connections != 2 || got.RxBytes != 2000 || got.TxBytes != 200 || got.Packets != 10 {
		t.Errorf("aggregation wrong: %+v", got)
	}
	if want := containerLabelPrefix + "gitea"; got.ProcessName != want {
		t.Errorf("ProcessName = %q, want %q", got.ProcessName, want)
	}
}
