//go:build !linux

package collector

import "fmt"

type NetworkInterface struct {
	Name    string `json:"name"`
	RxBytes uint64 `json:"rx_bytes"`
	TxBytes uint64 `json:"tx_bytes"`
}

type SystemMetrics struct {
	CPUUsagePercent   float64            `json:"cpu_usage_percent"`
	CPUCores          int                `json:"cpu_cores"`
	CPUModel          string             `json:"cpu_model"`
	CPUTemperature    float64            `json:"cpu_temperature"`
	FanRPM            float64            `json:"fan_rpm"`
	LoadAvg1          float64            `json:"load_avg_1"`
	LoadAvg5          float64            `json:"load_avg_5"`
	LoadAvg15         float64            `json:"load_avg_15"`
	MemoryTotal       uint64             `json:"memory_total"`
	MemoryUsed        uint64             `json:"memory_used"`
	MemoryFree        uint64             `json:"memory_free"`
	MemoryPercent     float64            `json:"memory_percent"`
	SwapTotal         uint64             `json:"swap_total"`
	SwapUsed          uint64             `json:"swap_used"`
	Disks             []DiskInfo         `json:"disks"`
	NetworkRxBytes    uint64             `json:"network_rx_bytes"`
	NetworkTxBytes    uint64             `json:"network_tx_bytes"`
	NetworkInterfaces []NetworkInterface `json:"network_interfaces,omitempty"`
	Uptime            uint64             `json:"uptime"`
	OS                string             `json:"os"`
	Hostname          string             `json:"hostname"`
}

type DiskInfo struct {
	MountPoint  string  `json:"mount_point"`
	Device      string  `json:"device"`
	FSType      string  `json:"fs_type"`
	TotalBytes  uint64  `json:"total_bytes"`
	UsedBytes   uint64  `json:"used_bytes"`
	FreeBytes   uint64  `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

// CollectSystem is Linux-only in production; this stub exists for non-Linux local builds.
func CollectSystem(_ bool) (*SystemMetrics, error) {
	return nil, fmt.Errorf("system collector is only supported on linux")
}
