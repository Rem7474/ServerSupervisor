// Package discovery implements basic network discovery: an ICMP ping sweep
// over an IPv4 CIDR block, so onboarding doesn't require knowing every host's
// IP up front. It deliberately stops at "which addresses answer a ping" — no
// ARP scanning, no port scanning, no OS fingerprinting. Discovered addresses
// are handed to internal/services/host (Register / RegisterBulk) to actually
// become monitored hosts; this package never writes a host row itself.
package discovery

import (
	"context"
	"fmt"
	"net"
	"sort"
	"sync"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/safego"
	"github.com/serversupervisor/server/internal/synthetic"
)

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	GetAllHosts(ctx context.Context) ([]models.Host, error)
}

// Pinger abstracts the ICMP echo call so tests can fake network behavior.
// synthetic.PingICMP satisfies it.
type Pinger func(ctx context.Context, target string) (success bool, latencyMs int, err error)

const (
	// minPrefixLen is the smallest (largest-network) IPv4 prefix a scan may
	// target — /24 (254 usable addresses). Anything larger would make a
	// single HTTP request ping thousands of addresses.
	minPrefixLen = 24
	// maxPrefixLen is the largest (smallest-network) prefix — /30 (2 usable
	// addresses). Below that there's nothing meaningful to sweep.
	maxPrefixLen = 30
	// concurrency bounds how many pings run in parallel, same rationale as
	// the uptime worker's semaphore: a full /24 shouldn't fork 254 goroutines
	// at once.
	concurrency = 64
)

// Service holds the discovery use-case.
type Service struct {
	repo Repository
	ping Pinger
}

// NewService wires the service against the real ICMP pinger.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, ping: synthetic.PingICMP}
}

// Scan ping-sweeps every usable address in cidr and reports which ones
// answered, cross-referenced against already-registered hosts. Results are
// sorted by IP address for a stable, scannable table.
func (s *Service) Scan(ctx context.Context, cidr string) ([]models.DiscoveredHost, error) {
	ips, err := usableIPs(cidr)
	if err != nil {
		return nil, err
	}

	existingByIP := map[string]models.Host{}
	if hosts, err := s.repo.GetAllHosts(ctx); err == nil {
		for _, h := range hosts {
			existingByIP[h.IPAddress] = h
		}
	}

	results := make([]models.DiscoveredHost, len(ips))
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	for i, ip := range ips {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, ip string) {
			defer wg.Done()
			defer func() { <-sem }()
			defer safego.Recover(ctx, "discovery.Scan.ping")
			success, latencyMs, _ := s.ping(ctx, ip)
			dh := models.DiscoveredHost{IPAddress: ip, Responded: success, LatencyMs: latencyMs}
			if h, ok := existingByIP[ip]; ok {
				dh.AlreadyRegistered = true
				dh.ExistingHostID = h.ID
				dh.ExistingHostName = h.Name
			}
			results[i] = dh
		}(i, ip)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return ipLess(results[i].IPAddress, results[j].IPAddress)
	})
	return results, nil
}

// usableIPs parses and bounds cidr, then enumerates every address excluding
// the network and broadcast addresses (for prefixes where those apply).
func usableIPs(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, apperr.Validation("adresse CIDR invalide")
	}
	if ip.To4() == nil {
		return nil, apperr.Validation("seul IPv4 est pris en charge pour la découverte réseau")
	}
	ones, bits := ipnet.Mask.Size()
	if bits != 32 {
		return nil, apperr.Validation("seul IPv4 est pris en charge pour la découverte réseau")
	}
	if ones < minPrefixLen || ones > maxPrefixLen {
		return nil, apperr.Validation(fmt.Sprintf("le préfixe doit être compris entre /%d et /%d (254 adresses max)", minPrefixLen, maxPrefixLen))
	}

	base := ipnet.IP.To4()
	broadcast := make(net.IP, 4)
	for i := range base {
		broadcast[i] = base[i] | ^ipnet.Mask[i]
	}

	var ips []string
	for cur := cloneIP(base); ipnet.Contains(cur); incIP(cur) {
		if cur.Equal(base) || cur.Equal(broadcast) {
			continue // skip network/broadcast addresses, not usable hosts
		}
		ips = append(ips, cur.String())
	}
	return ips, nil
}

func cloneIP(ip net.IP) net.IP {
	out := make(net.IP, len(ip))
	copy(out, ip)
	return out
}

func incIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			return
		}
	}
}

func ipLess(a, b string) bool {
	ipA, ipB := net.ParseIP(a).To4(), net.ParseIP(b).To4()
	if ipA == nil || ipB == nil {
		return a < b
	}
	for i := range ipA {
		if ipA[i] != ipB[i] {
			return ipA[i] < ipB[i]
		}
	}
	return false
}
