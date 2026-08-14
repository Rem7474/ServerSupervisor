//go:build !linux

package collector

// L7CaptureError mirrors the Linux implementation for CheckConfig's benefit.
// The whole network-flows collector is already unavailable off Linux (see
// network_flows_stub.go), so the optional L7 capture never even gets asked to
// run and has no separate condition of its own to report.
func L7CaptureError() string { return "" }
