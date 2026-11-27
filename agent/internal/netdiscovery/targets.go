// Package netdiscovery implements the agent side of extended network
// discovery: reading the kernel's ARP/neighbour table, a bounded TCP connect
// port scan and a basic SNMP GET probe. Everything here runs from the
// supervised host's own LAN position, which is why it lives in the agent
// rather than the server (the server is often not on the same L2 segment).
//
// The server validates every request before dispatching it; this package
// re-validates the same bounds independently so a compromised or buggy
// server can never turn an agent into an Internet scanner: targets must be
// private IPv4 (RFC 1918), a range is at most a /24, and port lists, rate,
// concurrency and duration are all capped here.
package netdiscovery

import (
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strings"
)

const (
	// MinPrefixLen is the widest range a scan may target (/24, 256 addresses).
	MinPrefixLen = 24
	// MaxPorts is the hard per-scan port-list cap enforced by the agent,
	// whatever the server sends.
	MaxPorts = 4096
)

var privatePrefixes = []netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
}

// IsPrivateIPv4 reports whether addr is an RFC 1918 IPv4 address. Loopback,
// link-local, CGNAT and every public range are rejected.
func IsPrivateIPv4(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.Is4() {
		return false
	}
	for _, p := range privatePrefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// ParseTarget parses a single private IPv4 address (an SNMP target).
func ParseTarget(s string) (netip.Addr, error) {
	addr, err := netip.ParseAddr(strings.TrimSpace(s))
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid IP address %q", s)
	}
	if !IsPrivateIPv4(addr) {
		return netip.Addr{}, fmt.Errorf("%s is not a private IPv4 address", addr)
	}
	return addr.Unmap(), nil
}

// ExpandCIDR validates cidr (private IPv4, prefix between /24 and /32) and
// returns the addresses to probe. For /24 to /30 the network and broadcast
// addresses are skipped; /31 and /32 return every address.
func ExpandCIDR(cidr string) ([]netip.Addr, error) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR %q", cidr)
	}
	if !prefix.Addr().Is4() {
		return nil, errors.New("only IPv4 ranges can be scanned")
	}
	if prefix.Bits() < MinPrefixLen {
		return nil, fmt.Errorf("range /%d is too wide: /%d at most", prefix.Bits(), MinPrefixLen)
	}
	prefix = prefix.Masked()
	if !IsPrivateIPv4(prefix.Addr()) {
		return nil, fmt.Errorf("%s is not a private IPv4 range", prefix)
	}

	var out []netip.Addr
	for a := prefix.Addr(); prefix.Contains(a); a = a.Next() {
		out = append(out, a)
	}
	if prefix.Bits() <= 30 && len(out) > 2 {
		out = out[1 : len(out)-1]
	}
	return out, nil
}

// NormalizePorts validates a port list (1-65535, at most MaxPorts entries)
// and returns it de-duplicated and sorted.
func NormalizePorts(ports []int) ([]int, error) {
	if len(ports) == 0 {
		return nil, errors.New("no port to scan")
	}
	seen := make(map[int]bool, len(ports))
	out := make([]int, 0, len(ports))
	for _, p := range ports {
		if p < 1 || p > 65535 {
			return nil, fmt.Errorf("invalid port %d", p)
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	if len(out) > MaxPorts {
		return nil, fmt.Errorf("too many ports: %d (max %d)", len(out), MaxPorts)
	}
	sort.Ints(out)
	return out, nil
}
