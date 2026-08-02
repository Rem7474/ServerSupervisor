package networkview

import (
	"context"
	"net"
	"regexp"
	"strings"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/proxmoxclient"
)

// ethInterfaceName matches the standard "ethX" guest interface (eth0, eth1, …).
// Other interfaces the guest agent reports — docker0, veth*, br-*, tailscale0,
// lo, ens18, ... — are noise for this correlation feature and are excluded.
var ethInterfaceName = regexp.MustCompile(`^eth\d+$`)

// GuestNetworksProvider is the subset of proxmox.Service's live-fetch
// capability BuildIPInventory needs. Defined here (consumer side) so it can
// be satisfied structurally by *proxmox.Service without networkview
// importing the proxmox service package for anything else, and so tests can
// pass a fake.
type GuestNetworksProvider interface {
	AllGuestNetworks(ctx context.Context) (map[string]map[int][]proxmoxclient.GuestNetworkIface, error)
}

// ProxyHostLister is the subset of npm.Service.ListAllProxyHosts BuildIPInventory needs.
type ProxyHostLister interface {
	ListAllProxyHosts(ctx context.Context) ([]models.NPMProxyHostEnriched, error)
}

// BuildIPInventory assembles a real-time, non-persisted view of every known
// IP address coming from Proxmox guests (correlated with a ServerSupervisor
// Host when a confirmed proxmox_guest_links entry exists) and every Nginx
// Proxy Manager proxy host (correlated by IP with a Host or a Proxmox guest
// when possible). Nothing here is cached or written to the database — every
// call re-fetches guest network interfaces live from Proxmox.
func BuildIPInventory(ctx context.Context, db *database.DB, proxmoxSvc GuestNetworksProvider, npmSvc ProxyHostLister) (*models.NetworkIPInventory, error) {
	guests, err := db.ListProxmoxGuests(ctx, "", "", "")
	if err != nil {
		return nil, err
	}
	links, err := db.ListProxmoxGuestLinks(ctx, "confirmed")
	if err != nil {
		return nil, err
	}
	hosts, err := db.GetAllHosts(ctx)
	if err != nil {
		return nil, err
	}
	nodes, err := db.ListProxmoxNodes(ctx)
	if err != nil {
		return nil, err
	}
	netsByNode, err := proxmoxSvc.AllGuestNetworks(ctx)
	if err != nil {
		return nil, err
	}

	hostIDToName := make(map[string]string, len(hosts))
	ipToHost := make(map[string]*models.Host, len(hosts))
	for i := range hosts {
		h := &hosts[i]
		hostIDToName[h.ID] = displayHostName(h.Name, h.Hostname)
		if h.IPAddress != "" {
			ipToHost[h.IPAddress] = h
		}
	}

	linkByGuestID := make(map[string]models.ProxmoxGuestLink, len(links))
	for _, l := range links {
		linkByGuestID[l.GuestID] = l
	}

	// Composite key (connection + node name) rather than node name alone,
	// since two different Proxmox connections could coincidentally use the
	// same node name.
	nodeKeyByID := make(map[string]string, len(nodes))
	for _, n := range nodes {
		nodeKeyByID[n.ID] = n.ConnectionID + "|" + n.NodeName
	}

	proxmoxGuests := make([]models.NetworkProxmoxGuestIP, 0, len(guests))
	ipToGuest := make(map[string]*models.NetworkProxmoxGuestIP)
	for _, g := range guests {
		entry := models.NetworkProxmoxGuestIP{
			GuestID:     g.ID,
			Name:        g.Name,
			Node:        g.NodeName,
			GuestType:   g.GuestType,
			VMID:        g.VMID,
			Status:      g.Status,
			IPAddresses: []string{},
		}
		if link, ok := linkByGuestID[g.ID]; ok {
			entry.HostID = link.HostID
			entry.HostName = hostIDToName[link.HostID]
		}
		guestNodeKey := g.ConnectionID + "|" + g.NodeName
		for nodeID, nodeNets := range netsByNode {
			if nodeKeyByID[nodeID] != guestNodeKey {
				continue
			}
			ifaces, ok := nodeNets[g.VMID]
			if !ok {
				continue
			}
			entry.IPAddresses = extractRoutableIPs(ifaces)
		}
		proxmoxGuests = append(proxmoxGuests, entry)
		idx := len(proxmoxGuests) - 1
		for _, ip := range entry.IPAddresses {
			ipToGuest[ip] = &proxmoxGuests[idx]
		}
	}

	npmProxyHosts, err := npmSvc.ListAllProxyHosts(ctx)
	if err != nil {
		return nil, err
	}
	npmEntries := make([]models.NetworkNPMEntry, 0, len(npmProxyHosts))
	for _, p := range npmProxyHosts {
		entry := models.NetworkNPMEntry{
			ProxyHostID: p.NPMID,
			DomainNames: p.DomainNames,
			ForwardHost: p.ForwardHost,
			ForwardPort: p.ForwardPort,
		}
		if h, ok := ipToHost[p.ForwardHost]; ok {
			entry.MatchedType = "host"
			entry.MatchedID = h.ID
			entry.MatchedName = displayHostName(h.Name, h.Hostname)
		} else if g, ok := ipToGuest[p.ForwardHost]; ok {
			entry.MatchedType = "proxmox_guest"
			entry.MatchedID = g.GuestID
			entry.MatchedName = g.Name
		}
		npmEntries = append(npmEntries, entry)
	}

	return &models.NetworkIPInventory{
		ProxmoxGuests: proxmoxGuests,
		NPMHosts:      npmEntries,
	}, nil
}

// extractRoutableIPs keeps only the "ethX" interface(s), strips the CIDR
// mask from their IPs, and drops loopback/link-local addresses — none of
// which are useful correlation targets on the Network page.
func extractRoutableIPs(ifaces []proxmoxclient.GuestNetworkIface) []string {
	var ips []string
	for _, iface := range ifaces {
		if !ethInterfaceName.MatchString(iface.Name) {
			continue
		}
		for _, cidr := range iface.IPs {
			addr := cidr
			if idx := strings.IndexByte(cidr, '/'); idx >= 0 {
				addr = cidr[:idx]
			}
			ip := net.ParseIP(addr)
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			ips = append(ips, addr)
		}
	}
	return ips
}

func displayHostName(name, hostname string) string {
	if name != "" {
		return name
	}
	return hostname
}
