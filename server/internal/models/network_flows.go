package models

import "time"

// NetworkFlowsReport is the agent-reported "top talkers" block for one report
// cycle: the busiest remote peers this host talked to, derived from conntrack
// accounting and aggregated agent-side into a bounded top-N + "others" bucket
// (see agent/internal/collector/network_flows.go). Mirrors
// agent/internal/collector.NetworkFlowsReport — kept in its own file rather
// than network.go, which already covers the unrelated logical-topology domain
// (server/internal/networkview).
type NetworkFlowsReport struct {
	// Available is false when the host can't provide per-connection byte
	// counters (nf_conntrack not loaded, accounting disabled, non-Linux, or
	// collection disabled in agent.yaml) — never an error, a capability flag.
	Available bool `json:"available"`
	// Reason is an actionable message when Available is false.
	Reason      string              `json:"reason,omitempty"`
	TopTalkers  []NetworkFlowTalker `json:"top_talkers,omitempty"`
	Others      *NetworkFlowBucket  `json:"others,omitempty"`
	TotalFlows  int                 `json:"total_flows"`
	CollectedAt time.Time           `json:"collected_at"`
}

// NetworkFlowTalker is one aggregated remote peer for a report cycle.
// RxBytes/TxBytes/Packets are deltas since the previous cycle, not conntrack's
// own cumulative counters — see agent/internal/collector/network_flows.go's
// per-cycle delta tracking. This is why server-side history aggregation must
// SUM() these values per time_bucket, not MAX()-MIN() like system_metrics'
// truly-cumulative network_rx_bytes/network_tx_bytes.
type NetworkFlowTalker struct {
	RemoteIP   string `json:"remote_ip"`
	RemotePort int    `json:"remote_port"`
	Protocol   string `json:"protocol"`  // "tcp" | "udp"
	Direction  string `json:"direction"` // "inbound" | "outbound"
	// ProcessName/PID are best-effort (socket→inode→pid via /proc), empty/0
	// when attribution failed — never treated as a collection error.
	ProcessName string `json:"process_name,omitempty"`
	PID         int    `json:"pid,omitempty"`
	RxBytes     uint64 `json:"rx_bytes"`
	TxBytes     uint64 `json:"tx_bytes"`
	Packets     uint64 `json:"packets"`
	Connections int    `json:"connections"`
}

// NetworkFlowBucket aggregates every talker beyond the agent's top-N cutoff.
type NetworkFlowBucket struct {
	Connections int    `json:"connections"`
	RxBytes     uint64 `json:"rx_bytes"`
	TxBytes     uint64 `json:"tx_bytes"`
}

// NetworkFlowMetric is one persisted row in network_flow_metrics: either a
// named talker (IsOthers=false) or the single per-cycle "others" rollup
// (IsOthers=true, RemoteIP/RemotePort/Protocol/Direction/ProcessName/PID all
// zero-valued).
type NetworkFlowMetric struct {
	ID          int64     `json:"id" db:"id"`
	HostID      string    `json:"host_id" db:"host_id"`
	Timestamp   time.Time `json:"timestamp" db:"timestamp"`
	IsOthers    bool      `json:"is_others" db:"is_others"`
	RemoteIP    string    `json:"remote_ip,omitempty" db:"remote_ip"`
	RemotePort  int       `json:"remote_port,omitempty" db:"remote_port"`
	Protocol    string    `json:"protocol,omitempty" db:"protocol"`
	Direction   string    `json:"direction,omitempty" db:"direction"`
	ProcessName string    `json:"process_name,omitempty" db:"process_name"`
	PID         int       `json:"pid,omitempty" db:"pid"`
	RxBytes     uint64    `json:"rx_bytes" db:"rx_bytes"`
	TxBytes     uint64    `json:"tx_bytes" db:"tx_bytes"`
	Packets     uint64    `json:"packets" db:"packets"`
	Connections int       `json:"connections" db:"connections"`
}

// NetworkFlowSummaryPoint is one time-bucketed point of a host's total tracked
// bandwidth (all talkers summed), used for the overview chart.
type NetworkFlowSummaryPoint struct {
	Timestamp time.Time `json:"timestamp"`
	RxBytes   uint64    `json:"rx_bytes"`
	TxBytes   uint64    `json:"tx_bytes"`
}
