package models

import "time"

// ========== Git Webhooks ==========

type GitWebhook struct {
	ID              string               `json:"id"`
	Name            string               `json:"name"`
	Secret          string               `json:"secret,omitempty"` // Only included when explicitly fetched
	Provider        string               `json:"provider"`
	RepoFilter      string               `json:"repo_filter"`
	BranchFilter    string               `json:"branch_filter"`
	EventFilter     string               `json:"event_filter"`
	HostID          string               `json:"host_id"`
	CustomTaskID    string               `json:"custom_task_id"`
	NotifyChannels  []string             `json:"notify_channels"`
	NotifyOnSuccess bool                 `json:"notify_on_success"`
	NotifyOnFailure bool                 `json:"notify_on_failure"`
	Enabled         bool                 `json:"enabled"`
	LastTriggeredAt *time.Time           `json:"last_triggered_at,omitempty"`
	CreatedAt       time.Time            `json:"created_at"`
	HostName        string               `json:"host_name,omitempty"`      // joined from hosts
	LastExecution   *GitWebhookExecution `json:"last_execution,omitempty"` // most recent execution
}

type GitWebhookExecution struct {
	ID            string     `json:"id"`
	WebhookID     string     `json:"webhook_id"`
	CommandID     *string    `json:"command_id,omitempty"`
	Provider      string     `json:"provider"`
	RepoName      string     `json:"repo_name"`
	Branch        string     `json:"branch"`
	CommitSHA     string     `json:"commit_sha"`
	CommitMessage string     `json:"commit_message"`
	Pusher        string     `json:"pusher"`
	Status        string     `json:"status"`
	TriggeredAt   time.Time  `json:"triggered_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}
