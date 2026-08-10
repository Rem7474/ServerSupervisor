package models

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
