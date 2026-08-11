//go:build !linux

package collector

import (
	"context"
	"time"
)

// NetworkFlowsReport mirrors the Linux implementation's shape (see
// network_flows.go) so callers and the wire format stay identical across
// platforms — only Available/Reason are ever populated here.
type NetworkFlowsReport struct {
	Available   bool                `json:"available"`
	Reason      string              `json:"reason,omitempty"`
	TopTalkers  []NetworkFlowTalker `json:"top_talkers,omitempty"`
	Others      *NetworkFlowBucket  `json:"others,omitempty"`
	TotalFlows  int                 `json:"total_flows"`
	CollectedAt time.Time           `json:"collected_at"`
}

type NetworkFlowTalker struct {
	RemoteIP    string `json:"remote_ip"`
	RemotePort  int    `json:"remote_port"`
	Protocol    string `json:"protocol"`
	Direction   string `json:"direction"`
	ProcessName string `json:"process_name,omitempty"`
	PID         int    `json:"pid,omitempty"`
	RxBytes     uint64 `json:"rx_bytes"`
	TxBytes     uint64 `json:"tx_bytes"`
	Packets     uint64 `json:"packets"`
	Connections int    `json:"connections"`
}

type NetworkFlowBucket struct {
	Connections int    `json:"connections"`
	RxBytes     uint64 `json:"rx_bytes"`
	TxBytes     uint64 `json:"tx_bytes"`
}

// CollectNetworkFlows is unavailable on non-Linux platforms (conntrack is a
// Linux-only kernel facility) — always reports the capability as absent
// rather than erroring, same contract as the Linux implementation.
func CollectNetworkFlows(_ context.Context, _ int) (*NetworkFlowsReport, error) {
	return &NetworkFlowsReport{Available: false, Reason: "unsupported platform", CollectedAt: time.Now()}, nil
}

// checkConntrackAcct backs diagnostics.go's CheckConfig on non-Linux
// platforms — always unavailable, same reasoning as CollectNetworkFlows.
func checkConntrackAcct() (bool, string) {
	return false, "unsupported platform"
}
