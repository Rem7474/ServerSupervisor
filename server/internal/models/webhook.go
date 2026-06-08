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

// GitWebhookRequest is the create/update body for a git webhook — the writable
// subset of GitWebhook. id, secret, last_triggered_at, created_at, host_name and
// last_execution are server-managed and never accepted from the client.
type GitWebhookRequest struct {
	Name            string   `json:"name"`
	Provider        string   `json:"provider"`
	RepoFilter      string   `json:"repo_filter"`
	BranchFilter    string   `json:"branch_filter"`
	EventFilter     string   `json:"event_filter"`
	HostID          string   `json:"host_id"`
	CustomTaskID    string   `json:"custom_task_id"`
	NotifyChannels  []string `json:"notify_channels"`
	NotifyOnSuccess bool     `json:"notify_on_success"`
	NotifyOnFailure bool     `json:"notify_on_failure"`
	Enabled         bool     `json:"enabled"`
}

// ToModel maps the request onto a GitWebhook (pure field copy; callers apply any
// create-time defaults).
func (r GitWebhookRequest) ToModel() GitWebhook {
	return GitWebhook{
		Name:            r.Name,
		Provider:        r.Provider,
		RepoFilter:      r.RepoFilter,
		BranchFilter:    r.BranchFilter,
		EventFilter:     r.EventFilter,
		HostID:          r.HostID,
		CustomTaskID:    r.CustomTaskID,
		NotifyChannels:  r.NotifyChannels,
		NotifyOnSuccess: r.NotifyOnSuccess,
		NotifyOnFailure: r.NotifyOnFailure,
		Enabled:         r.Enabled,
	}
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
