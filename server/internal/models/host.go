package models

import "time"

// ========== Host (VM) ==========

type Host struct {
	ID           string    `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`         // User-defined name (e.g., "Prod Web Server")
	Hostname     string    `json:"hostname" db:"hostname"` // System hostname (auto-populated by agent)
	IPAddress    string    `json:"ip_address" db:"ip_address"`
	OS           string    `json:"os" db:"os"`                       // Auto-populated by agent
	AgentVersion string    `json:"agent_version" db:"agent_version"` // Agent version
	Tags         []string  `json:"tags" db:"tags"`
	APIKey       string    `json:"-" db:"api_key"`
	Status       string    `json:"status" db:"status"` // online, offline, warning
	LastSeen     time.Time `json:"last_seen" db:"last_seen"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type HostRegistration struct {
	Name      string   `json:"name" binding:"required"`
	IPAddress string   `json:"ip_address" binding:"required"`
	Tags      []string `json:"tags"`
}

type HostUpdate struct {
	Name         *string   `json:"name"`
	Hostname     *string   `json:"hostname"`
	IPAddress    *string   `json:"ip_address"`
	OS           *string   `json:"os"`
	AgentVersion *string   `json:"agent_version"`
	Tags         *[]string `json:"tags"`
}

// ========== System Metrics ==========

type SystemMetrics struct {
	ID        int64     `json:"id" db:"id"`
	HostID    string    `json:"host_id" db:"host_id"`
	Timestamp time.Time `json:"timestamp" db:"timestamp"`

	// CPU
	CPUUsagePercent float64 `json:"cpu_usage_percent" db:"cpu_usage_percent"`
	CPUCores        int     `json:"cpu_cores" db:"cpu_cores"`
	CPUModel        string  `json:"cpu_model" db:"cpu_model"`
	LoadAvg1        float64 `json:"load_avg_1" db:"load_avg_1"`
	LoadAvg5        float64 `json:"load_avg_5" db:"load_avg_5"`
	LoadAvg15       float64 `json:"load_avg_15" db:"load_avg_15"`

	// Memory
	MemoryTotal   uint64  `json:"memory_total" db:"memory_total"`
	MemoryUsed    uint64  `json:"memory_used" db:"memory_used"`
	MemoryFree    uint64  `json:"memory_free" db:"memory_free"`
	MemoryPercent float64 `json:"memory_percent" db:"memory_percent"`
	SwapTotal     uint64  `json:"swap_total" db:"swap_total"`
	SwapUsed      uint64  `json:"swap_used" db:"swap_used"`

	// Disk
	Disks []DiskInfo `json:"disks" db:"-"`

	// Network
	NetworkRxBytes uint64 `json:"network_rx_bytes" db:"network_rx_bytes"`
	NetworkTxBytes uint64 `json:"network_tx_bytes" db:"network_tx_bytes"`

	// System
	Uptime   uint64 `json:"uptime" db:"uptime"`
	OS       string `json:"os" db:"-"`
	Hostname string `json:"hostname" db:"hostname"`
}

type DiskInfo struct {
	ID          int64   `json:"id" db:"id"`
	MetricsID   int64   `json:"-" db:"metrics_id"`
	MountPoint  string  `json:"mount_point" db:"mount_point"`
	Device      string  `json:"device" db:"device"`
	FSType      string  `json:"fs_type" db:"fs_type"`
	TotalBytes  uint64  `json:"total_bytes" db:"total_bytes"`
	UsedBytes   uint64  `json:"used_bytes" db:"used_bytes"`
	FreeBytes   uint64  `json:"free_bytes" db:"free_bytes"`
	UsedPercent float64 `json:"used_percent" db:"used_percent"`
}

// SystemMetricsSummary is a global aggregated view used for dashboard charts.
type SystemMetricsSummary struct {
	Timestamp   time.Time `json:"timestamp" db:"timestamp"`
	CPUAvg      float64   `json:"cpu_avg" db:"cpu_avg"`
	MemoryAvg   float64   `json:"memory_avg" db:"memory_avg"`
	SampleCount int       `json:"sample_count" db:"sample_count"`
}

// ========== Disk Metrics ==========

type DiskMetrics struct {
	ID            int64     `json:"id" db:"id"`
	HostID        string    `json:"host_id" db:"host_id"`
	Timestamp     time.Time `json:"timestamp" db:"timestamp"`
	MountPoint    string    `json:"mount_point" db:"mount_point"`
	Filesystem    string    `json:"filesystem" db:"filesystem"`
	SizeGB        float64   `json:"size_gb" db:"size_gb"`
	UsedGB        float64   `json:"used_gb" db:"used_gb"`
	AvailGB       float64   `json:"avail_gb" db:"avail_gb"`
	UsedPercent   float64   `json:"used_percent" db:"used_percent"`
	InodesTotal   int64     `json:"inodes_total" db:"inodes_total"`
	InodesUsed    int64     `json:"inodes_used" db:"inodes_used"`
	InodesFree    int64     `json:"inodes_free" db:"inodes_free"`
	InodesPercent float64   `json:"inodes_percent" db:"inodes_percent"`
}

// DiskHealth for SMART monitoring (optional, collected if smartctl available)
type DiskHealth struct {
	ID             int64     `json:"id" db:"id"`
	HostID         string    `json:"host_id" db:"host_id"`
	CollectedAt    time.Time `json:"collected_at" db:"timestamp"`
	Device         string    `json:"device" db:"device"` // /dev/sda, /dev/nvme0n1
	Model          string    `json:"model" db:"model"`
	SerialNumber   string    `json:"serial_number" db:"serial_number"`
	SmartStatus    string    `json:"smart_status" db:"smart_status"` // PASSED, FAILED, UNKNOWN
	Temperature    int       `json:"temperature" db:"temperature"`   // Celsius
	PowerOnHours   int64     `json:"power_on_hours" db:"power_on_hours"`
	PowerCycles    int64     `json:"power_cycles" db:"power_cycles"`
	ReallocSectors int       `json:"realloc_sectors" db:"realloc_sectors"`
	PendingSectors int       `json:"pending_sectors" db:"pending_sectors"`
}

// ========== Metrics Aggregation (for downsampling) ==========

// MetricsAggregate stores downsampled metrics (5-min, hourly, daily)
type MetricsAggregate struct {
	ID              int64     `json:"id" db:"id"`
	HostID          string    `json:"host_id" db:"host_id"`
	AggregationType string    `json:"aggregation_type" db:"aggregation_type"` // 5min, hour, day
	Timestamp       time.Time `json:"timestamp" db:"timestamp"`               // Start of the interval

	// Metrics (averages for the period)
	CPUUsageAvg    float64 `json:"cpu_usage_avg" db:"cpu_usage_avg"`
	CPUUsageMax    float64 `json:"cpu_usage_max" db:"cpu_usage_max"`
	MemoryUsageAvg    uint64  `json:"memory_usage_avg" db:"memory_usage_avg"`
	MemoryUsageMax    uint64  `json:"memory_usage_max" db:"memory_usage_max"`
	MemoryPercentAvg  float64 `json:"memory_percent_avg" db:"memory_percent_avg"`
	DiskUsageAvg   float64 `json:"disk_usage_avg" db:"disk_usage_avg"`
	NetworkRxBytes uint64  `json:"network_rx_bytes" db:"network_rx_bytes"`
	NetworkTxBytes uint64  `json:"network_tx_bytes" db:"network_tx_bytes"`

	SampleCount int       `json:"sample_count" db:"sample_count"` // How many raw samples in period
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}
