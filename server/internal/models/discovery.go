package models

import "time"

// NetworkScanRequest is a subnet discovery scan request: ping-sweep every
// usable address in an IPv4 CIDR block.
type NetworkScanRequest struct {
	CIDR string `json:"cidr" binding:"required"`
}

// DiscoveredHost is one address's outcome from a subnet discovery scan.
type DiscoveredHost struct {
	IPAddress         string `json:"ip_address"`
	Responded         bool   `json:"responded"`
	LatencyMs         int    `json:"latency_ms,omitempty"`
	AlreadyRegistered bool   `json:"already_registered"`
	ExistingHostID    string `json:"existing_host_id,omitempty"`
	ExistingHostName  string `json:"existing_host_name,omitempty"`
}

// Agent-run discovery (module "discovery"): the ARP/neighbour table read,
// the TCP connect port scan and the SNMP probe run on a chosen agent, from
// that host's LAN position, as regular remote commands.

// DiscoveryAgentRequest picks the agent that runs an ARP table read.
type DiscoveryAgentRequest struct {
	HostID string `json:"host_id" binding:"required"`
}

// PortScanRequest starts a port scan from an agent. Ports accepts a list of
// ports and ranges ("22,80,8000-8100"); empty means the default top ports.
type PortScanRequest struct {
	HostID string `json:"host_id" binding:"required"`
	CIDR   string `json:"cidr" binding:"required"`
	Ports  string `json:"ports"`
}

// SNMPProbeRequest starts an SNMP GET from an agent. Community and the v3
// passphrases are write-only: they are handed to the agent when it claims
// the command and are never stored, logged nor returned.
type SNMPProbeRequest struct {
	HostID            string `json:"host_id" binding:"required"`
	Target            string `json:"target" binding:"required"`
	Port              int    `json:"port"`
	Version           string `json:"version" binding:"required"` // 2c | 3
	IncludeInterfaces bool   `json:"include_interfaces"`
	Community         string `json:"community"`
	Username          string `json:"username"`
	SecurityLevel     string `json:"security_level"` // noAuthNoPriv | authNoPriv | authPriv
	AuthProtocol      string `json:"auth_protocol"`  // SHA | SHA256
	AuthPassphrase    string `json:"auth_passphrase"`
	PrivProtocol      string `json:"priv_protocol"` // AES
	PrivPassphrase    string `json:"priv_passphrase"`
}

// DiscoveryJobStarted is returned when a discovery command is queued.
type DiscoveryJobStarted struct {
	JobID string `json:"job_id"`
}

// DiscoveryLimits describes the bounds the UI must respect.
type DiscoveryLimits struct {
	DefaultPorts   []int    `json:"default_ports"`
	MaxPorts       int      `json:"max_ports"`
	MinPrefixLen   int      `json:"min_prefix_len"`
	AllowedSubnets []string `json:"allowed_subnets"`
}

// ARPNeighbor is one ARP/neighbour table entry seen by the agent.
type ARPNeighbor struct {
	IP                  string `json:"ip"`
	MAC                 string `json:"mac"`
	Interface           string `json:"interface"`
	VendorHint          string `json:"vendor_hint,omitempty"`
	LocallyAdministered bool   `json:"locally_administered"`
	AlreadyRegistered   bool   `json:"already_registered"`
	ExistingHostID      string `json:"existing_host_id,omitempty"`
	ExistingHostName    string `json:"existing_host_name,omitempty"`
}

// ARPResult is the outcome of an ARP table read.
type ARPResult struct {
	Entries   []ARPNeighbor `json:"entries"`
	Truncated bool          `json:"truncated,omitempty"`
}

// PortScanHost is one address with at least one open TCP port.
type PortScanHost struct {
	IP                string `json:"ip"`
	OpenPorts         []int  `json:"open_ports"`
	AlreadyRegistered bool   `json:"already_registered"`
	ExistingHostID    string `json:"existing_host_id,omitempty"`
	ExistingHostName  string `json:"existing_host_name,omitempty"`
}

// PortScanResult is the outcome of a port scan; partial when Cancelled or
// TimedOut.
type PortScanResult struct {
	CIDR         string         `json:"cidr"`
	HostsScanned int            `json:"hosts_scanned"`
	PortsScanned int            `json:"ports_scanned"`
	ProbesDone   int            `json:"probes_done"`
	ProbesTotal  int            `json:"probes_total"`
	Cancelled    bool           `json:"cancelled"`
	TimedOut     bool           `json:"timed_out"`
	DurationMs   int64          `json:"duration_ms"`
	Hosts        []PortScanHost `json:"hosts"`
}

// SNMPInterface is one ifTable row summary.
type SNMPInterface struct {
	Index       int    `json:"index"`
	Descr       string `json:"descr"`
	SpeedBps    uint64 `json:"speed_bps"`
	PhysAddress string `json:"phys_address,omitempty"`
	AdminStatus string `json:"admin_status"`
	OperStatus  string `json:"oper_status"`
}

// SNMPResult is the outcome of an SNMP probe (system group + optional
// interface summary). Never carries credentials.
type SNMPResult struct {
	Target              string          `json:"target"`
	Version             string          `json:"version"`
	SysDescr            string          `json:"sys_descr"`
	SysObjectID         string          `json:"sys_object_id"`
	SysUpTimeSeconds    uint64          `json:"sys_uptime_seconds"`
	SysContact          string          `json:"sys_contact"`
	SysName             string          `json:"sys_name"`
	SysLocation         string          `json:"sys_location"`
	Interfaces          []SNMPInterface `json:"interfaces,omitempty"`
	InterfacesTruncated bool            `json:"interfaces_truncated,omitempty"`
	InterfacesError     string          `json:"interfaces_error,omitempty"`
}

// DiscoveryJob is the state of one agent-run discovery command. Exactly one
// of ARP / PortScan / SNMP is set once the job completed.
type DiscoveryJob struct {
	ID        string          `json:"id"`
	HostID    string          `json:"host_id"`
	Kind      string          `json:"kind"`   // arp | port_scan | snmp
	Status    string          `json:"status"` // pending | running | completed | failed
	Error     string          `json:"error,omitempty"`
	ARP       *ARPResult      `json:"arp,omitempty"`
	PortScan  *PortScanResult `json:"port_scan,omitempty"`
	SNMP      *SNMPResult     `json:"snmp,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	EndedAt   *time.Time      `json:"ended_at,omitempty"`
}
