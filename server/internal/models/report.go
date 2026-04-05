package models

import "time"

// ========== Agent Capabilities ==========

// AgentCapabilities tracks which collectors are active on the agent
type AgentCapabilities struct {
	Docker  bool `json:"docker"`   // Docker collector enabled
	APT     bool `json:"apt"`      // APT package collector enabled
	SMART   bool `json:"smart"`    // SMART disk health enabled
	CPUTemp bool `json:"cpu_temp"` // CPU temperature collector enabled
	WebLogs bool `json:"web_logs"` // Web access log parsing enabled
	Systemd bool `json:"systemd"`  // Systemd unit monitoring enabled
	Journal bool `json:"journal"`  // Journald log collection enabled
}

// ========== Agent Heartbeat / Full Report ==========

// AgentReport is the full status report sent by the agent to the server
type AgentReport struct {
	HostID          string              `json:"host_id"`
	AgentVersion    string              `json:"agent_version"`
	Capabilities    *AgentCapabilities  `json:"capabilities,omitempty"` // Which collectors are enabled on this agent
	Metrics         *SystemMetrics      `json:"metrics,omitempty"`
	Docker          *DockerReport       `json:"docker,omitempty"`
	AptStatus       *AptStatus          `json:"apt_status,omitempty"`
	WebLogs         *WebLogReport       `json:"web_logs,omitempty"`
	DockerNetworks  []DockerNetwork     `json:"docker_networks,omitempty"`
	ContainerEnvs   []ContainerEnv      `json:"container_envs,omitempty"`
	ComposeProjects []ComposeProject    `json:"compose_projects,omitempty"`
	DiskMetrics     []DiskMetrics       `json:"disk_metrics,omitempty"`
	DiskHealth      []DiskHealth        `json:"disk_health,omitempty"`
	CustomTasks     []CustomTaskSummary `json:"custom_tasks,omitempty"`
	Timestamp       time.Time           `json:"timestamp"`
}
