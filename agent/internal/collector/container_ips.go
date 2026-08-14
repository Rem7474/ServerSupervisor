package collector

import (
	"net"
	"sync"
	"time"
)

// containerIPTTL bounds how long the last CollectDocker() container-IP snapshot
// stays usable by other collectors. Docker collection and network-flow
// collection run as two independent goroutines in the same report cycle (see
// reporter.Send), so the flow collector always reads the *previous* cycle's
// snapshot rather than racing for this one's — container IPs are near-static,
// so a one-cycle lag is irrelevant, while a shared lock between the two
// goroutines would serialize collection for no gain.
//
// The TTL is what makes this safe when Docker collection stops (daemon down,
// collect_docker turned off, container removed): the snapshot expires instead
// of pinning stale container IPs into the flow collector's "local" set forever.
const containerIPTTL = 10 * time.Minute

var (
	containerIPMu   sync.RWMutex
	containerIPs    map[string]string
	containerIPsAt  time.Time
	containerIPsNow = time.Now // swappable in tests
)

// buildContainerIPIndex maps each container-owned IP address to its container
// name. Pure (no Docker daemon, no clock) so the mapping rules stay unit
// testable — see container_ips_test.go.
//
// Only addresses Docker itself reported as belonging to a container on *this*
// host end up here: this index is what widens the network-flow collector's
// notion of "local" (see network_flows.go's classifyFlows), so anything looser
// would start re-classifying genuinely forwarded/routed traffic as this host's
// own.
func buildContainerIPIndex(containers []DockerContainer) map[string]string {
	index := make(map[string]string, len(containers))
	for _, c := range containers {
		if c.Name == "" {
			continue
		}
		for _, raw := range c.IPAddresses {
			parsed := net.ParseIP(raw)
			if parsed == nil || parsed.IsUnspecified() {
				continue
			}
			// Normalize through net.IP so the key matches what the conntrack
			// and /proc paths produce for the same address (e.g. an
			// IPv4-mapped or zero-padded IPv6 literal).
			key := parsed.String()
			if _, exists := index[key]; exists {
				continue // first container wins; a shared IP is a Docker anomaly, not a case to guess at
			}
			index[key] = c.Name
		}
	}
	return index
}

// publishContainerIPs records the container-IP index built from a successful
// CollectDocker() pass. Called only on success — a failed Docker collection
// leaves the previous snapshot in place to age out via containerIPTTL rather
// than blanking attribution on one transient daemon hiccup.
func publishContainerIPs(containers []DockerContainer) {
	index := buildContainerIPIndex(containers)
	containerIPMu.Lock()
	defer containerIPMu.Unlock()
	containerIPs = index
	containerIPsAt = containerIPsNow()
}

// ContainerIPIndex returns the most recent container IP → container name
// snapshot, or an empty map when there is none or it has aged past
// containerIPTTL. Never nil, so callers can index it unconditionally.
func ContainerIPIndex() map[string]string {
	containerIPMu.RLock()
	defer containerIPMu.RUnlock()
	if containerIPs == nil || containerIPsNow().Sub(containerIPsAt) > containerIPTTL {
		return map[string]string{}
	}
	out := make(map[string]string, len(containerIPs))
	for ip, name := range containerIPs {
		out[ip] = name
	}
	return out
}
