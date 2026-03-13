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
	ID           string    `json:"id"`
	ConnectionID string    `json:"connection_id"`
	NodeName     string    `json:"node_name"`
	Status       string    `json:"status"`
	CPUCount     int       `json:"cpu_count"`
	CPUUsage     float64   `json:"cpu_usage"`
	MemTotal     int64     `json:"mem_total"`
	MemUsed      int64     `json:"mem_used"`
	Uptime       int64     `json:"uptime"`
	PVEVersion   string    `json:"pve_version"`
	ClusterName  string    `json:"cluster_name"`
	IPAddress    string    `json:"ip_address"`
	LastSeenAt   time.Time `json:"last_seen_at"`
	// Computed counts
	VMCount  int `json:"vm_count,omitempty"`
	LXCCount int `json:"lxc_count,omitempty"`
	// Detail view (populated on single-node fetch)
	Guests   []ProxmoxGuest   `json:"guests,omitempty"`
	Storages []ProxmoxStorage `json:"storages,omitempty"`
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
}
