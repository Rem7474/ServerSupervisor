package models

import "time"

// ========== Network Snapshot ==========

type PortMapping struct {
	HostIP        string `json:"host_ip"`
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
	Raw           string `json:"raw"`
}

type NetworkContainer struct {
	ID           string            `json:"id"`
	HostID       string            `json:"host_id"`
	Hostname     string            `json:"hostname"`
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	ImageTag     string            `json:"image_tag"`
	State        string            `json:"state"`
	Status       string            `json:"status"`
	Ports        string            `json:"ports"`
	PortMappings []PortMapping     `json:"port_mappings"`
	Labels       map[string]string `json:"labels,omitempty" db:"-"`
	NetRxBytes   uint64            `json:"net_rx_bytes"`
	NetTxBytes   uint64            `json:"net_tx_bytes"`
}

type NetworkHost struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Hostname       string    `json:"hostname"`
	IPAddress      string    `json:"ip_address"`
	Status         string    `json:"status"`
	NetworkRxBytes uint64    `json:"network_rx_bytes"`
	NetworkTxBytes uint64    `json:"network_tx_bytes"`
	LastSeen       time.Time `json:"last_seen"`
}

type NetworkSnapshot struct {
	Hosts      []NetworkHost      `json:"hosts"`
	Containers []NetworkContainer `json:"containers"`
	UpdatedAt  time.Time          `json:"updated_at"`
}

// ========== Network Topology (Persistent Configuration) ==========

// NetworkTopologyConfig stores persisted configuration (replaces localStorage)
type NetworkTopologyConfig struct {
	ID             int64     `json:"id" db:"id"`
	RootLabel      string    `json:"root_label" db:"root_label"`
	RootIP         string    `json:"root_ip" db:"root_ip"`
	ExcludedPorts  []int     `json:"excluded_ports" db:"-"`        // Stored as JSONB
	ServiceMap     string    `json:"service_map" db:"service_map"` // JSON {port: name}
	HostOverrides  string    `json:"host_overrides" db:"host_overrides"`   // JSON
	ManualServices string    `json:"manual_services" db:"manual_services"` // JSON
	AutheliaLabel  string    `json:"authelia_label" db:"authelia_label"`
	AutheliaIP     string    `json:"authelia_ip" db:"authelia_ip"`
	InternetLabel  string    `json:"internet_label" db:"internet_label"`
	InternetIP     string    `json:"internet_ip" db:"internet_ip"`
	NodePositions  string    `json:"node_positions" db:"node_positions"` // JSON {nodeId: {x, y}}
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// TopologySnapshot is the complete network state sent via WebSocket
type TopologySnapshot struct {
	Hosts      []NetworkHost          `json:"hosts"`
	Containers []NetworkContainer     `json:"containers"`
	Config     *NetworkTopologyConfig `json:"config,omitempty"`
	UpdatedAt  time.Time              `json:"updated_at"`
}
