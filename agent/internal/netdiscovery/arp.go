package netdiscovery

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"sort"
	"strconv"
	"strings"
)

// procNetARP is the kernel's IPv4 neighbour table. A var so tests can point
// it at a fixture.
var procNetARP = "/proc/net/arp"

// maxARPEntries bounds the reported table: a host on a flat /16 can hold far
// more neighbours than any UI should render in one go.
const maxARPEntries = 4096

// atfComplete is the ATF_COM flag: the entry has a resolved hardware address.
const atfComplete = 0x2

// ARPEntry is one resolved neighbour. JSON tags mirror the server's
// models.ARPNeighbor fields that the agent fills.
type ARPEntry struct {
	IP        string `json:"ip"`
	MAC       string `json:"mac"`
	Interface string `json:"interface"`
}

// ARPResult is the command output of discovery/arp.
type ARPResult struct {
	Entries   []ARPEntry `json:"entries"`
	Truncated bool       `json:"truncated,omitempty"`
}

// ReadARPTable reads the kernel neighbour table. It only reads what the
// kernel already learnt: no ARP request is sent.
func ReadARPTable() (*ARPResult, error) {
	f, err := os.Open(procNetARP)
	if err != nil {
		return nil, fmt.Errorf("cannot read the ARP table: %w", err)
	}
	defer func() { _ = f.Close() }()
	return ParseProcNetARP(f)
}

// ParseProcNetARP parses the /proc/net/arp format. Incomplete entries (no
// resolved MAC yet, or a failed resolution) are skipped; IPv4 addresses only.
func ParseProcNetARP(r io.Reader) (*ARPResult, error) {
	res := &ARPResult{Entries: []ARPEntry{}}
	sc := bufio.NewScanner(r)
	first := true
	seen := map[string]bool{}
	for sc.Scan() {
		if first {
			first = false
			continue // header line
		}
		fields := strings.Fields(sc.Text())
		if len(fields) < 6 {
			continue
		}
		entry, ok := parseARPFields(fields)
		if !ok {
			continue
		}
		key := entry.IP + "|" + entry.Interface
		if seen[key] {
			continue
		}
		seen[key] = true
		if len(res.Entries) >= maxARPEntries {
			res.Truncated = true
			break
		}
		res.Entries = append(res.Entries, entry)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("cannot parse the ARP table: %w", err)
	}
	sort.Slice(res.Entries, func(i, j int) bool {
		a, _ := netip.ParseAddr(res.Entries[i].IP)
		b, _ := netip.ParseAddr(res.Entries[j].IP)
		if c := a.Compare(b); c != 0 {
			return c < 0
		}
		return res.Entries[i].Interface < res.Entries[j].Interface
	})
	return res, nil
}

func parseARPFields(fields []string) (ARPEntry, bool) {
	addr, err := netip.ParseAddr(fields[0])
	if err != nil || !addr.Is4() {
		return ARPEntry{}, false
	}
	flags, err := strconv.ParseUint(strings.TrimPrefix(fields[2], "0x"), 16, 32)
	if err != nil || flags&atfComplete == 0 {
		return ARPEntry{}, false
	}
	hw, err := net.ParseMAC(fields[3])
	if err != nil || isZeroMAC(hw) {
		return ARPEntry{}, false
	}
	return ARPEntry{IP: addr.String(), MAC: hw.String(), Interface: fields[5]}, true
}

func isZeroMAC(hw net.HardwareAddr) bool {
	for _, b := range hw {
		if b != 0 {
			return false
		}
	}
	return true
}
