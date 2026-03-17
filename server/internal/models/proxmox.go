package models

import "time"

// ProxmoxConnection stores configuration for one Proxmox VE endpoint.
// token_secret is never serialised to JSON to avoid leaks.
type ProxmoxConnection struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	APIURL             string     `json:"api_url"`
	TokenID            string     `json:"token_id"`
	InsecureSkipVerify bool       `json:"insecure_skip_verify"`
	Enabled            bool       `json:"enabled"`
	PollIntervalSec    int        `json:"poll_interval_sec"`
	LastError          string     `json:"last_error"`
	LastErrorAt        *time.Time `json:"last_error_at,omitempty"`
	LastSuccessAt      *time.Time `json:"last_success_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	// Computed stats (joined, not stored)
	NodeCount  int `json:"node_count,omitempty"`
	GuestCount int `json:"guest_count,omitempty"`
}

type ProxmoxNode struct {
	ID           string     `json:"id"`
	ConnectionID string     `json:"connection_id"`
	NodeName     string     `json:"node_name"`
	Status       string     `json:"status"`
	CPUCount     int        `json:"cpu_count"`
	CPUUsage     float64    `json:"cpu_usage"`
	MemTotal     int64      `json:"mem_total"`
	MemUsed      int64      `json:"mem_used"`
	Uptime       int64      `json:"uptime"`
	PVEVersion   string     `json:"pve_version"`
	ClusterName  string     `json:"cluster_name"`
	IPAddress    string     `json:"ip_address"`
	LastSeenAt   time.Time  `json:"last_seen_at"`
	// Pending apt updates (polled from /nodes/{node}/apt/update)
	PendingUpdates    int        `json:"pending_updates"`
	SecurityUpdates   int        `json:"security_updates"`
	LastUpdateCheckAt *time.Time `json:"last_update_check_at,omitempty"`
	// Computed counts
	VMCount  int `json:"vm_count,omitempty"`
	LXCCount int `json:"lxc_count,omitempty"`
	// Detail view (populated on single-node fetch)
	Guests   []ProxmoxGuest   `json:"guests,omitempty"`
	Storages []ProxmoxStorage `json:"storages,omitempty"`
	Disks    []ProxmoxDisk    `json:"disks,omitempty"`
	Tasks    []ProxmoxTask    `json:"tasks,omitempty"`
}

type ProxmoxGuest struct {
	ID           string    `json:"id"`
	ConnectionID string    `json:"connection_id"`
	NodeName     string    `json:"node_name"`
	GuestType    string    `json:"guest_type"` // vm | lxc
	VMID         int       `json:"vmid"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	CPUAlloc     float64   `json:"cpu_alloc"`
	CPUUsage     float64   `json:"cpu_usage"`
	MemAlloc     int64     `json:"mem_alloc"`
	MemUsage     int64     `json:"mem_usage"`
	DiskAlloc    int64     `json:"disk_alloc"`
	Tags         string    `json:"tags"`
	Uptime       int64     `json:"uptime"`
	LastSeenAt   time.Time `json:"last_seen_at"`
}

type ProxmoxStorage struct {
	ID           string    `json:"id"`
	ConnectionID string    `json:"connection_id"`
	NodeName     string    `json:"node_name"`
	StorageName  string    `json:"storage_name"`
	StorageType  string    `json:"storage_type"`
	Total        int64     `json:"total"`
	Used         int64     `json:"used"`
	Avail        int64     `json:"avail"`
	Enabled      bool      `json:"enabled"`
	Active       bool      `json:"active"`
	Shared       bool      `json:"shared"`
	LastSeenAt   time.Time `json:"last_seen_at"`
}

// ProxmoxTask represents a single Proxmox task (from GET /nodes/{node}/tasks).
type ProxmoxTask struct {
	ID           string     `json:"id"`
	ConnectionID string     `json:"connection_id"`
	NodeName     string     `json:"node_name"`
	UPID         string     `json:"upid"`
	TaskType     string     `json:"task_type"`
	Status       string     `json:"status"`      // running | stopped
	UserName     string     `json:"user_name"`
	StartTime    *time.Time `json:"start_time,omitempty"`
	EndTime      *time.Time `json:"end_time,omitempty"`
	ExitStatus   string     `json:"exit_status"` // OK | error message | ""
	ObjectID     string     `json:"object_id"`   // vmid or other Proxmox object
	LastSeenAt   time.Time  `json:"last_seen_at"`
}

// ProxmoxBackupJob represents a Proxmox backup job configuration (GET /cluster/backup).
type ProxmoxBackupJob struct {
	ID           string    `json:"id"`
	ConnectionID string    `json:"connection_id"`
	JobID        string    `json:"job_id"`
	Enabled      bool      `json:"enabled"`
	Schedule     string    `json:"schedule"`
	Storage      string    `json:"storage"`
	Mode         string    `json:"mode"`     // snapshot | suspend | stop
	Compress     string    `json:"compress"`
	VMIDs        string    `json:"vmids"`    // comma-separated VMIDs or "all"
	MailTo       string    `json:"mail_to"`
	LastSeenAt   time.Time `json:"last_seen_at"`
}

// ProxmoxBackupRun stores the latest backup result per VM (derived from vzdump tasks).
type ProxmoxBackupRun struct {
	ID           string     `json:"id"`
	ConnectionID string     `json:"connection_id"`
	NodeName     string     `json:"node_name"`
	VMID         int        `json:"vmid"`
	TaskUPID     string     `json:"task_upid"`
	Status       string     `json:"status"`    // OK | error | running
	StartTime    *time.Time `json:"start_time,omitempty"`
	EndTime      *time.Time `json:"end_time,omitempty"`
	ExitStatus   string     `json:"exit_status"`
	LastSeenAt   time.Time  `json:"last_seen_at"`
	// Joined from proxmox_guests
	GuestName string `json:"guest_name,omitempty"`
}

// ProxmoxDisk represents a physical disk in a Proxmox node (GET /nodes/{node}/disks/list).
type ProxmoxDisk struct {
	ID           string    `json:"id"`
	ConnectionID string    `json:"connection_id"`
	NodeName     string    `json:"node_name"`
	DevPath      string    `json:"dev_path"`
	Model        string    `json:"model"`
	Serial       string    `json:"serial"`
	SizeBytes    int64     `json:"size_bytes"`
	DiskType     string    `json:"disk_type"` // ssd | hdd | nvme | unknown
	Health       string    `json:"health"`    // PASSED | FAILED | UNKNOWN
	Wearout      int       `json:"wearout"`   // SSD wear % (100=new), -1 if N/A
	LastSeenAt   time.Time `json:"last_seen_at"`
}

// ProxmoxConnectionRequest is the body for create/update endpoints.
// TokenSecret is optional on update (empty means "keep existing").
type ProxmoxConnectionRequest struct {
	Name               string `json:"name" binding:"required"`
	APIURL             string `json:"api_url" binding:"required"`
	TokenID            string `json:"token_id" binding:"required"`
	TokenSecret        string `json:"token_secret"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	Enabled            bool   `json:"enabled"`
	PollIntervalSec    int    `json:"poll_interval_sec"`
}

// ProxmoxSummary is returned by GET /proxmox/summary.
type ProxmoxSummary struct {
	ConnectionCount int   `json:"connection_count"`
	NodeCount       int   `json:"node_count"`
	VMCount         int   `json:"vm_count"`
	LXCCount        int   `json:"lxc_count"`
	StorageTotal    int64 `json:"storage_total"`
	StorageUsed     int64 `json:"storage_used"`
	// Health signals (computed, read-only)
	NodesDown         int `json:"nodes_down"`
	StorageNearFull   int `json:"storage_near_full"`   // usage > 80 %
	StorageOffline    int `json:"storage_offline"`     // active=false or enabled=false
	RecentFailedTasks int `json:"recent_failed_tasks"` // exit_status != 'OK' in last 24 h
}

// ProxmoxNodeMetricsSummary is a time-bucketed aggregate used for dashboard trend charts.
// cpu_avg is expressed as a percentage (0‒100).
type ProxmoxNodeMetricsSummary struct {
	Timestamp   time.Time `json:"timestamp"`
	CPUAvg      float64   `json:"cpu_avg"`
	MemoryAvg   float64   `json:"memory_avg"`
	SampleCount int       `json:"sample_count"`
}

// ProxmoxGuestLink maps a Proxmox guest (VM/LXC) to a ServerSupervisor host (agent).
// Status lifecycle: suggested (auto-detected) → confirmed (validated) or ignored (dismissed).
// MetricsSource controls which data source feeds CPU/RAM/disk in host views.
type ProxmoxGuestLink struct {
	ID            string    `json:"id"`
	GuestID       string    `json:"guest_id"`
	HostID        string    `json:"host_id"`
	Status        string    `json:"status"`         // suggested | confirmed | ignored
	MetricsSource string    `json:"metrics_source"` // auto | agent | proxmox
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	// Joined display fields (populated on list/get)
	GuestName    string  `json:"guest_name,omitempty"`
	GuestType    string  `json:"guest_type,omitempty"`
	NodeName     string  `json:"node_name,omitempty"`
	VMID         int     `json:"vmid,omitempty"`
	HostName     string  `json:"host_name,omitempty"`
	HostHostname string  `json:"host_hostname,omitempty"`
	// Live metrics from the Proxmox guest (populated on list/get)
	CPUUsage float64 `json:"cpu_usage"`
	MemAlloc int64   `json:"mem_alloc"`
	MemUsage int64   `json:"mem_usage"`
}

// ProxmoxGuestLinkRequest is the body for POST /proxmox/links.
type ProxmoxGuestLinkRequest struct {
	GuestID       string `json:"guest_id" binding:"required"`
	HostID        string `json:"host_id" binding:"required"`
	Status        string `json:"status"`         // defaults to "confirmed"
	MetricsSource string `json:"metrics_source"` // defaults to "auto"
}

// ProxmoxGuestLinkUpdate is the body for PUT /proxmox/links/:id.
type ProxmoxGuestLinkUpdate struct {
	Status        *string `json:"status"`
	MetricsSource *string `json:"metrics_source"`
}
