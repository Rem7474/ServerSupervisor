package discovery

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
)

// Agent-run discovery. The ping sweep above runs on the server; ARP, port
// scan and SNMP run on an agent instead, because only a host sitting on the
// target LAN sees its neighbour table and reaches its private addresses (the
// server is often in another network, a VPS or a container). They reuse the
// regular remote-command pipeline (module "discovery"): dispatch, claim,
// stream, result, audit log. The agent re-checks every bound below on its
// own and refuses active probes unless its local config opts in.

const (
	discoveryModule = "discovery"
	// scanMinPrefixLen is the widest range a port scan may target.
	scanMinPrefixLen = 24
	// hardMaxPorts mirrors the agent's own cap; the configurable maximum
	// (DISCOVERY_PORT_SCAN_MAX_PORTS) is clamped to it.
	hardMaxPorts       = 4096
	defaultMaxPorts    = 1024
	secretTTL          = 15 * time.Minute
	cancelTTL          = time.Hour
	maxJobErrorLen     = 500
	snmpMinPassphrase  = 8
	snmpMaxCredential  = 255
	cancelledJobOutput = "cancelled"
)

// DefaultPorts is the port list scanned when the request names none: common
// infrastructure, homelab and admin services.
var DefaultPorts = []int{
	21, 22, 23, 25, 53, 80, 81, 88, 110, 111, 135, 139, 143, 179, 389, 443, 445,
	465, 514, 515, 548, 554, 587, 631, 636, 873, 993, 995, 1080, 1194, 1433, 1521,
	1723, 1883, 2049, 2222, 2375, 2376, 3000, 3128, 3306, 3389, 3689, 4443, 5000,
	5001, 5060, 5432, 5601, 5672, 5900, 5985, 6379, 6443, 7070, 8000, 8006, 8008,
	8080, 8081, 8083, 8086, 8088, 8123, 8181, 8443, 8444, 8888, 9000, 9001, 9090,
	9091, 9100, 9200, 9443, 10000, 10250, 11211, 15672, 25565, 27017, 32400,
}

var rfc1918 = []netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
}

// AgentRepository is the data-access port of agent-run discovery.
// *database.DB satisfies it structurally.
type AgentRepository interface {
	GetHost(ctx context.Context, id string) (*models.Host, error)
	GetAllHosts(ctx context.Context) ([]models.Host, error)
	GetRemoteCommandByID(ctx context.Context, id string) (*models.RemoteCommand, error)
	CancelPendingRemoteCommand(ctx context.Context, id, output string) (bool, error)
	CreateAuditLog(ctx context.Context, username, action, hostID, ipAddress, details, status string) (int64, error)
}

// Dispatcher is the part of *dispatch.Dispatcher used here.
type Dispatcher interface {
	Create(ctx context.Context, req dispatch.Request) (*dispatch.Result, error)
}

// StatusBroadcaster pushes a command's terminal status to live viewers.
// *ws.CommandStreamHub satisfies it.
type StatusBroadcaster interface {
	BroadcastStatus(commandID, status, output string)
}

// Actor identifies who triggers a discovery, for the command row and audit.
type Actor struct {
	Username  string
	IPAddress string
}

type snmpSecrets struct {
	Community      string `json:"community,omitempty"`
	AuthPassphrase string `json:"auth_passphrase,omitempty"`
	PrivPassphrase string `json:"priv_passphrase,omitempty"`
}

type heldSecret struct {
	secrets snmpSecrets
	expires time.Time
}

// AgentService runs the agent-side discovery use-cases. It also implements
// agent.CommandInterceptor (SNMP credential injection at claim time,
// cancellation signalling) and agent.CommandCompletionListener.
type AgentService struct {
	repo   AgentRepository
	disp   Dispatcher
	cfg    *config.Config
	status StatusBroadcaster
	now    func() time.Time

	mu        sync.Mutex
	secrets   map[string]heldSecret // secret_ref -> credentials, in memory only
	cancelled map[string]time.Time  // command id -> cancel request time
}

// NewAgentService wires the service. status may be nil.
func NewAgentService(repo AgentRepository, disp Dispatcher, cfg *config.Config, status StatusBroadcaster) *AgentService {
	return &AgentService{
		repo: repo, disp: disp, cfg: cfg, status: status, now: time.Now,
		secrets: map[string]heldSecret{}, cancelled: map[string]time.Time{},
	}
}

// Limits reports the bounds a client must respect.
func (s *AgentService) Limits() models.DiscoveryLimits {
	return models.DiscoveryLimits{
		DefaultPorts:   append([]int(nil), DefaultPorts...),
		MaxPorts:       s.maxPorts(),
		MinPrefixLen:   scanMinPrefixLen,
		AllowedSubnets: s.allowedSubnetStrings(),
	}
}

func (s *AgentService) maxPorts() int {
	m := s.cfg.DiscoveryPortScanMaxPorts
	if m <= 0 {
		return defaultMaxPorts
	}
	return min(m, hardMaxPorts)
}

func (s *AgentService) allowedSubnets() []netip.Prefix {
	var out []netip.Prefix
	for _, raw := range s.cfg.DiscoveryAllowedSubnets {
		if p, err := netip.ParsePrefix(strings.TrimSpace(raw)); err == nil && p.Addr().Is4() {
			out = append(out, p.Masked())
		}
	}
	return out
}

func (s *AgentService) allowedSubnetStrings() []string {
	out := []string{}
	for _, p := range s.allowedSubnets() {
		out = append(out, p.String())
	}
	return out
}

// requireAgent loads the host and checks its agent can run the action.
func (s *AgentService) requireAgent(ctx context.Context, hostID string, active bool) error {
	host, err := s.repo.GetHost(ctx, hostID)
	if err != nil || host == nil {
		return apperr.NotFound("host not found")
	}
	if !host.Collectors["network_discovery"] {
		return apperr.Validation("this host's agent does not support network discovery").I18n(apperr.CodeDiscoveryAgentUnsupported, nil)
	}
	if active && !host.Collectors["network_scan"] {
		return apperr.Validation("active scanning is disabled on this agent").I18n(apperr.CodeDiscoveryScanDisabled, nil)
	}
	return nil
}

// checkTargetPrefix enforces RFC 1918 and the declared allowed subnets.
func (s *AgentService) checkTargetPrefix(p netip.Prefix) error {
	private := false
	for _, r := range rfc1918 {
		if r.Bits() <= p.Bits() && r.Contains(p.Addr()) {
			private = true
			break
		}
	}
	if !private {
		return apperr.Validation("only private IPv4 addresses can be probed").I18n(apperr.CodeDiscoveryPrivateOnly, nil)
	}
	allowed := s.allowedSubnets()
	if len(allowed) == 0 {
		return nil
	}
	for _, a := range allowed {
		if a.Bits() <= p.Bits() && a.Contains(p.Addr()) {
			return nil
		}
	}
	return apperr.Validation(fmt.Sprintf("%s is outside the subnets allowed for active discovery", p)).
		I18n(apperr.CodeDiscoverySubnetNotAllowed, map[string]string{"target": p.String(), "allowed": strings.Join(s.allowedSubnetStrings(), ", ")})
}

// ValidateScanCIDR checks a port-scan range: IPv4, /24 to /32, private and
// within the allowed subnets. Returns the canonical (masked) prefix.
func (s *AgentService) ValidateScanCIDR(cidr string) (netip.Prefix, error) {
	p, err := netip.ParsePrefix(strings.TrimSpace(cidr))
	if err != nil {
		return netip.Prefix{}, apperr.Validation("invalid CIDR address").I18n(apperr.CodeDiscoveryCIDRInvalid, nil)
	}
	if !p.Addr().Is4() {
		return netip.Prefix{}, apperr.Validation("only IPv4 is supported for network discovery").I18n(apperr.CodeDiscoveryIPv4Only, nil)
	}
	if p.Bits() < scanMinPrefixLen {
		return netip.Prefix{}, apperr.Validation(fmt.Sprintf("the prefix must be between /%d and /32", scanMinPrefixLen)).
			I18n(apperr.CodeDiscoveryPrefixRange, map[string]string{"min": strconv.Itoa(scanMinPrefixLen), "max": "32"})
	}
	p = p.Masked()
	if err := s.checkTargetPrefix(p); err != nil {
		return netip.Prefix{}, err
	}
	return p, nil
}

func (s *AgentService) validateTarget(target string) (netip.Addr, error) {
	a, err := netip.ParseAddr(strings.TrimSpace(target))
	if err != nil || !a.Is4() {
		return netip.Addr{}, apperr.Validation("invalid IPv4 address").I18n(apperr.CodeDiscoveryIPInvalid, nil)
	}
	if err := s.checkTargetPrefix(netip.PrefixFrom(a, 32)); err != nil {
		return netip.Addr{}, err
	}
	return a, nil
}

// ParsePorts parses "22,80,8000-8100" into a sorted, de-duplicated list
// capped at max ports. An empty spec yields DefaultPorts.
func ParsePorts(spec string, maxPorts int) ([]int, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return append([]int(nil), DefaultPorts...), nil
	}
	invalid := func(v string) error {
		return apperr.Validation(fmt.Sprintf("invalid port list near %q", v)).I18n(apperr.CodeDiscoveryPortsInvalid, map[string]string{"value": v})
	}
	tooMany := apperr.Validation(fmt.Sprintf("too many ports: %d at most per scan", maxPorts)).
		I18n(apperr.CodeDiscoveryTooManyPorts, map[string]string{"max": strconv.Itoa(maxPorts)})
	seen := map[int]bool{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lo, hi, isRange := strings.Cut(part, "-")
		start, err1 := strconv.Atoi(strings.TrimSpace(lo))
		end := start
		var err2 error
		if isRange {
			end, err2 = strconv.Atoi(strings.TrimSpace(hi))
		}
		if err1 != nil || err2 != nil || start < 1 || end > 65535 || start > end {
			return nil, invalid(part)
		}
		if end-start+1 > maxPorts {
			return nil, tooMany
		}
		for p := start; p <= end; p++ {
			seen[p] = true
		}
		if len(seen) > maxPorts {
			return nil, tooMany
		}
	}
	if len(seen) == 0 {
		return nil, invalid(spec)
	}
	ports := make([]int, 0, len(seen))
	for p := range seen {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	return ports, nil
}

// StartARP queues a read of the agent's ARP/neighbour table (passive: the
// agent sends no packet).
func (s *AgentService) StartARP(ctx context.Context, actor Actor, hostID string) (string, error) {
	if err := s.requireAgent(ctx, hostID, false); err != nil {
		return "", err
	}
	return s.dispatch(ctx, actor, hostID, "arp", "", map[string]any{}, map[string]any{})
}

// StartPortScan validates and queues a TCP connect port scan.
func (s *AgentService) StartPortScan(ctx context.Context, actor Actor, req models.PortScanRequest) (string, error) {
	prefix, err := s.ValidateScanCIDR(req.CIDR)
	if err != nil {
		return "", err
	}
	ports, err := ParsePorts(req.Ports, s.maxPorts())
	if err != nil {
		return "", err
	}
	if err := s.requireAgent(ctx, req.HostID, true); err != nil {
		return "", err
	}
	payload := map[string]any{"cidr": prefix.String(), "ports": ports}
	audit := map[string]any{"cidr": prefix.String(), "ports": len(ports)}
	return s.dispatch(ctx, actor, req.HostID, "port_scan", prefix.String(), payload, audit)
}

// StartSNMP validates and queues an SNMP GET. Credentials are held in
// memory under a one-time reference and only reach the agent at claim time.
func (s *AgentService) StartSNMP(ctx context.Context, actor Actor, req models.SNMPProbeRequest) (string, error) {
	addr, err := s.validateTarget(req.Target)
	if err != nil {
		return "", err
	}
	payload, secrets, err := snmpPayload(req)
	if err != nil {
		return "", err
	}
	if err := s.requireAgent(ctx, req.HostID, true); err != nil {
		return "", err
	}
	ref, err := newSecretRef()
	if err != nil {
		return "", apperr.Internal(err)
	}
	payload["target"] = addr.String()
	payload["secret_ref"] = ref
	s.holdSecret(ref, secrets)

	audit := map[string]any{"target": addr.String(), "version": req.Version}
	id, err := s.dispatch(ctx, actor, req.HostID, "snmp", addr.String(), payload, audit)
	if err != nil {
		s.mu.Lock()
		delete(s.secrets, ref)
		s.mu.Unlock()
		return "", err
	}
	return id, nil
}

func snmpPayload(req models.SNMPProbeRequest) (map[string]any, snmpSecrets, error) {
	invalid := func(field string) error {
		return apperr.Validation("invalid or missing SNMP parameter: "+field).I18n(apperr.CodeDiscoverySNMPInvalid, map[string]string{"field": field})
	}
	port := req.Port
	if port == 0 {
		port = 161
	}
	if port < 1 || port > 65535 {
		return nil, snmpSecrets{}, invalid("port")
	}
	payload := map[string]any{"port": port, "version": req.Version, "include_interfaces": req.IncludeInterfaces}
	var sec snmpSecrets
	switch req.Version {
	case "2c":
		if req.Community == "" || len(req.Community) > snmpMaxCredential {
			return nil, sec, invalid("community")
		}
		sec.Community = req.Community
	case "3":
		if req.Username == "" || len(req.Username) > 64 {
			return nil, sec, invalid("username")
		}
		payload["username"] = req.Username
		payload["security_level"] = req.SecurityLevel
		switch req.SecurityLevel {
		case "noAuthNoPriv":
		case "authNoPriv", "authPriv":
			if req.AuthProtocol != "SHA" && req.AuthProtocol != "SHA256" {
				return nil, sec, invalid("auth_protocol")
			}
			if !validPassphrase(req.AuthPassphrase) {
				return nil, sec, invalid("auth_passphrase")
			}
			payload["auth_protocol"] = req.AuthProtocol
			sec.AuthPassphrase = req.AuthPassphrase
			if req.SecurityLevel == "authPriv" {
				if req.PrivProtocol != "AES" {
					return nil, sec, invalid("priv_protocol")
				}
				if !validPassphrase(req.PrivPassphrase) {
					return nil, sec, invalid("priv_passphrase")
				}
				payload["priv_protocol"] = req.PrivProtocol
				sec.PrivPassphrase = req.PrivPassphrase
			}
		default:
			return nil, sec, invalid("security_level")
		}
	default:
		return nil, sec, invalid("version")
	}
	return payload, sec, nil
}

func validPassphrase(p string) bool {
	return len(p) >= snmpMinPassphrase && len(p) <= snmpMaxCredential
}

func newSecretRef() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *AgentService) holdSecret(ref string, sec snmpSecrets) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked()
	s.secrets[ref] = heldSecret{secrets: sec, expires: s.now().Add(secretTTL)}
}

// sweepLocked drops expired credentials and stale cancel requests.
func (s *AgentService) sweepLocked() {
	now := s.now()
	for ref, h := range s.secrets {
		if now.After(h.expires) {
			delete(s.secrets, ref)
		}
	}
	for id, at := range s.cancelled {
		if now.Sub(at) > cancelTTL {
			delete(s.cancelled, id)
		}
	}
}

func (s *AgentService) dispatch(ctx context.Context, actor Actor, hostID, action, target string, payload, auditDetails map[string]any) (string, error) {
	body, _ := json.Marshal(payload)
	details, _ := json.Marshal(auditDetails)
	res, err := s.disp.Create(ctx, dispatch.Request{
		HostID:      hostID,
		Module:      discoveryModule,
		Action:      action,
		Target:      target,
		Payload:     string(body),
		TriggeredBy: actor.Username,
		Audit: &dispatch.AuditLogRequest{
			Username:  actor.Username,
			Action:    "discovery_" + action,
			HostID:    hostID,
			IPAddress: actor.IPAddress,
			Details:   string(details),
		},
	})
	if err != nil {
		return "", apperr.Failed("failed to create command")
	}
	return res.Command.ID, nil
}

// PrepareClaimed injects the held SNMP credentials into the payload of a
// claimed snmp command, right before it is sent to the agent, and forgets
// them. A command whose credentials expired (or were lost on a restart)
// goes out without them and the agent fails it with an explicit message.
func (s *AgentService) PrepareClaimed(_ context.Context, _ string, cmds []models.PendingCommand) []models.PendingCommand {
	for i := range cmds {
		if cmds[i].Module != discoveryModule || cmds[i].Action != "snmp" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(cmds[i].Payload), &payload); err != nil {
			continue
		}
		ref, _ := payload["secret_ref"].(string)
		delete(payload, "secret_ref")
		s.mu.Lock()
		s.sweepLocked()
		if held, ok := s.secrets[ref]; ok {
			payload["secrets"] = held.secrets
			delete(s.secrets, ref)
		}
		s.mu.Unlock()
		if b, err := json.Marshal(payload); err == nil {
			cmds[i].Payload = string(b)
		}
	}
	return cmds
}

// CancelRequested reports whether an operator cancelled this command.
func (s *AgentService) CancelRequested(commandID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.cancelled[commandID]
	return ok
}

// HandleCommandCompletion forgets a finished command's cancel request.
func (s *AgentService) HandleCommandCompletion(commandID, _ string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cancelled, commandID)
}

func (s *AgentService) loadJobCommand(ctx context.Context, id string) (*models.RemoteCommand, error) {
	cmd, err := s.repo.GetRemoteCommandByID(ctx, id)
	if err != nil || cmd == nil || cmd.Module != discoveryModule {
		return nil, apperr.NotFound("discovery job not found")
	}
	return cmd, nil
}

// Cancel stops a discovery job: a job no agent picked up yet is failed
// right away; a running one is flagged and the agent stops at its next
// progress report (the port scan keeps the partial result). Cancelling a
// finished job is a no-op.
func (s *AgentService) Cancel(ctx context.Context, actor Actor, id string) (*models.DiscoveryJob, error) {
	cmd, err := s.loadJobCommand(ctx, id)
	if err != nil {
		return nil, err
	}
	switch cmd.Status {
	case "pending":
		ok, err := s.repo.CancelPendingRemoteCommand(ctx, id, cancelledJobOutput)
		if err != nil {
			return nil, apperr.Failed("failed to cancel the job")
		}
		if ok {
			if s.status != nil {
				s.status.BroadcastStatus(id, "failed", cancelledJobOutput)
			}
			break
		}
		s.markCancelled(id) // claimed in the meantime: stop it on the agent
	case "running":
		s.markCancelled(id)
	default:
		return s.Job(ctx, id)
	}
	details, _ := json.Marshal(map[string]string{"job_id": id, "kind": cmd.Action})
	_, _ = s.repo.CreateAuditLog(ctx, actor.Username, "discovery_cancel", cmd.HostID, actor.IPAddress, string(details), "success")
	return s.Job(ctx, id)
}

func (s *AgentService) markCancelled(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked()
	s.cancelled[id] = s.now()
}

// Job returns a discovery job's state and, once completed, its parsed
// result cross-referenced with the registered hosts.
func (s *AgentService) Job(ctx context.Context, id string) (*models.DiscoveryJob, error) {
	cmd, err := s.loadJobCommand(ctx, id)
	if err != nil {
		return nil, err
	}
	job := &models.DiscoveryJob{
		ID: cmd.ID, HostID: cmd.HostID, Kind: cmd.Action, Status: cmd.Status,
		CreatedAt: cmd.CreatedAt, EndedAt: cmd.EndedAt,
	}
	switch cmd.Status {
	case "failed":
		job.Error = jobError(cmd.Output)
	case "completed":
		if err := s.decodeResult(ctx, job, cmd.Output); err != nil {
			job.Status = "failed"
			job.Error = "invalid result returned by the agent"
		}
	}
	return job, nil
}

func jobError(output string) string {
	msg := strings.TrimSpace(strings.TrimPrefix(output, "ERROR:"))
	if len(msg) > maxJobErrorLen {
		msg = strings.ToValidUTF8(msg[:maxJobErrorLen], "")
	}
	return msg
}

func (s *AgentService) decodeResult(ctx context.Context, job *models.DiscoveryJob, output string) error {
	switch job.Kind {
	case "arp":
		var r models.ARPResult
		if err := json.Unmarshal([]byte(output), &r); err != nil {
			return err
		}
		byIP := s.hostsByIP(ctx)
		for i := range r.Entries {
			e := &r.Entries[i]
			e.VendorHint = VendorHint(e.MAC)
			e.LocallyAdministered = locallyAdministered(e.MAC)
			if h, ok := byIP[e.IP]; ok {
				e.AlreadyRegistered, e.ExistingHostID, e.ExistingHostName = true, h.ID, h.Name
			}
		}
		if r.Entries == nil {
			r.Entries = []models.ARPNeighbor{}
		}
		job.ARP = &r
	case "port_scan":
		var r models.PortScanResult
		if err := json.Unmarshal([]byte(output), &r); err != nil {
			return err
		}
		byIP := s.hostsByIP(ctx)
		for i := range r.Hosts {
			if h, ok := byIP[r.Hosts[i].IP]; ok {
				r.Hosts[i].AlreadyRegistered, r.Hosts[i].ExistingHostID, r.Hosts[i].ExistingHostName = true, h.ID, h.Name
			}
		}
		if r.Hosts == nil {
			r.Hosts = []models.PortScanHost{}
		}
		job.PortScan = &r
	case "snmp":
		var r models.SNMPResult
		if err := json.Unmarshal([]byte(output), &r); err != nil {
			return err
		}
		job.SNMP = &r
	default:
		return fmt.Errorf("unknown discovery kind %q", job.Kind)
	}
	return nil
}

func (s *AgentService) hostsByIP(ctx context.Context) map[string]models.Host {
	out := map[string]models.Host{}
	hosts, err := s.repo.GetAllHosts(ctx)
	if err != nil {
		return out
	}
	for _, h := range hosts {
		if h.IPAddress != "" {
			out[h.IPAddress] = h
		}
	}
	return out
}

// vendorPrefixes is a deliberately small OUI table: the virtualisation
// platforms and single-board computers a self-hosted LAN commonly holds.
// It is a hint, not a vendor database.
var vendorPrefixes = map[string]string{
	"00:05:69": "VMware",
	"00:0c:29": "VMware",
	"00:1c:14": "VMware",
	"00:50:56": "VMware",
	"52:54:00": "QEMU/KVM",
	"bc:24:11": "Proxmox VE",
	"08:00:27": "VirtualBox",
	"00:15:5d": "Hyper-V",
	"00:16:3e": "Xen",
	"b8:27:eb": "Raspberry Pi",
	"dc:a6:32": "Raspberry Pi",
	"e4:5f:01": "Raspberry Pi",
	"d8:3a:dd": "Raspberry Pi",
	"2c:cf:67": "Raspberry Pi",
	"28:cd:c1": "Raspberry Pi",
}

// VendorHint returns a vendor label for well-known MAC prefixes, or "".
func VendorHint(mac string) string {
	mac = strings.ToLower(mac)
	if strings.HasPrefix(mac, "02:42:") {
		return "Docker"
	}
	if len(mac) < 8 {
		return ""
	}
	return vendorPrefixes[mac[:8]]
}

// locallyAdministered reports the U/L bit: set on randomised (privacy) MACs
// and most software-generated ones.
func locallyAdministered(mac string) bool {
	if len(mac) < 2 {
		return false
	}
	b, err := strconv.ParseUint(mac[:2], 16, 8)
	return err == nil && b&0x02 != 0
}
