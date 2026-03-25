package models

import "time"

// ========== Agent Heartbeat / Full Report ==========

// AgentReport is the full status report sent by the agent to the server
type AgentReport struct {
	HostID          string              `json:"host_id"`
	AgentVersion    string              `json:"agent_version"`
	Metrics         *SystemMetrics      `json:"metrics,omitempty"`
	Docker          *DockerReport       `json:"docker,omitempty"`
	AptStatus       *AptStatus          `json:"apt_status,omitempty"`
	BotDetection    map[string]any      `json:"bot_detection,omitempty"`
	NPMAnalytics    map[string]any      `json:"npm_analytics,omitempty"`
	DockerNetworks  []DockerNetwork     `json:"docker_networks,omitempty"`
	ContainerEnvs   []ContainerEnv      `json:"container_envs,omitempty"`
	ComposeProjects []ComposeProject    `json:"compose_projects,omitempty"`
	DiskMetrics     []DiskMetrics       `json:"disk_metrics,omitempty"`
	DiskHealth      []DiskHealth        `json:"disk_health,omitempty"`
	CustomTasks     []CustomTaskSummary `json:"custom_tasks,omitempty"`
	Timestamp       time.Time           `json:"timestamp"`
}
