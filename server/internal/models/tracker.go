package models

import "time"

// ========== Release Trackers ==========

type ReleaseTracker struct {
	ID                    string                   `json:"id"`
	Name                  string                   `json:"name"`
	TrackerType           string                   `json:"tracker_type"` // "git" | "docker"
	Provider              string                   `json:"provider"`     // github, gitlab, gitea (git type only)
	RepoOwner             string                   `json:"repo_owner"`
	RepoName              string                   `json:"repo_name"`
	DockerImage           string                   `json:"docker_image"` // docker image name (docker type: required; git type: unused)
	DockerTag             string                   `json:"docker_tag"`   // docker tag to monitor (docker type, e.g. "latest")
	HostID                string                   `json:"host_id"`
	CustomTaskID          string                   `json:"custom_task_id"`
	LastReleaseTag        string                   `json:"last_release_tag"`
	LatestImageDigest     string                   `json:"latest_image_digest,omitempty"` // manifest sha256 for docker trackers
	CooldownHours         int                      `json:"cooldown_hours"`
	LastReleaseDetectedAt *time.Time               `json:"last_release_detected_at,omitempty"`
	LastCheckedAt         *time.Time               `json:"last_checked_at,omitempty"`
	LastTriggeredAt       *time.Time               `json:"last_triggered_at,omitempty"`
	LastError             string                   `json:"last_error,omitempty"`
	NotifyChannels        []string                 `json:"notify_channels"`
	NotifyOnRelease       bool                     `json:"notify_on_release"`
	Enabled               bool                     `json:"enabled"`
	CreatedAt             time.Time                `json:"created_at"`
	HostName              string                   `json:"host_name,omitempty"`
	LastExecution         *ReleaseTrackerExecution `json:"last_execution,omitempty"`

	// Compose update mode (Watchtower-like). UpdateAction "custom" (default)
	// dispatches a tasks.yaml command; "compose" dispatches the native compose
	// module which runs pull + up -d on ComposeProject.
	UpdateAction          string `json:"update_action"`
	ComposeProject        string `json:"compose_project,omitempty"`
	ComposeService        string `json:"compose_service,omitempty"` // empty = whole project
	PreUpdateTaskID       string `json:"pre_update_task_id,omitempty"`
	PostUpdateTaskID      string `json:"post_update_task_id,omitempty"`
	CleanupAfterUpdate    bool   `json:"cleanup_after_update"`
	HealthcheckTimeoutSec int    `json:"healthcheck_timeout_sec"`
	RollbackOnFailure     bool   `json:"rollback_on_failure"`
	RegistryCredentialsID string `json:"registry_credentials_id,omitempty"`
}

// ReleaseTrackerRequest is the create/update body for a release tracker — the
// writable configuration subset of ReleaseTracker. The polling state
// (last_release_tag, latest_image_digest, last_*_at, last_error) and the joined
// fields (id, created_at, host_name, last_execution) are server-managed and never
// accepted from the client.
type ReleaseTrackerRequest struct {
	Name                  string   `json:"name"`
	TrackerType           string   `json:"tracker_type"`
	Provider              string   `json:"provider"`
	RepoOwner             string   `json:"repo_owner"`
	RepoName              string   `json:"repo_name"`
	DockerImage           string   `json:"docker_image"`
	DockerTag             string   `json:"docker_tag"`
	HostID                string   `json:"host_id"`
	CustomTaskID          string   `json:"custom_task_id"`
	CooldownHours         int      `json:"cooldown_hours"`
	NotifyChannels        []string `json:"notify_channels"`
	NotifyOnRelease       bool     `json:"notify_on_release"`
	Enabled               bool     `json:"enabled"`
	UpdateAction          string   `json:"update_action"`
	ComposeProject        string   `json:"compose_project"`
	ComposeService        string   `json:"compose_service"`
	PreUpdateTaskID       string   `json:"pre_update_task_id"`
	PostUpdateTaskID      string   `json:"post_update_task_id"`
	CleanupAfterUpdate    bool     `json:"cleanup_after_update"`
	HealthcheckTimeoutSec int      `json:"healthcheck_timeout_sec"`
	RollbackOnFailure     bool     `json:"rollback_on_failure"`
	RegistryCredentialsID string   `json:"registry_credentials_id"`
}

// ToModel maps the request onto a ReleaseTracker (config fields only; callers
// apply validation and any create-time defaults).
func (r ReleaseTrackerRequest) ToModel() ReleaseTracker {
	return ReleaseTracker{
		Name:                  r.Name,
		TrackerType:           r.TrackerType,
		Provider:              r.Provider,
		RepoOwner:             r.RepoOwner,
		RepoName:              r.RepoName,
		DockerImage:           r.DockerImage,
		DockerTag:             r.DockerTag,
		HostID:                r.HostID,
		CustomTaskID:          r.CustomTaskID,
		CooldownHours:         r.CooldownHours,
		NotifyChannels:        r.NotifyChannels,
		NotifyOnRelease:       r.NotifyOnRelease,
		Enabled:               r.Enabled,
		UpdateAction:          r.UpdateAction,
		ComposeProject:        r.ComposeProject,
		ComposeService:        r.ComposeService,
		PreUpdateTaskID:       r.PreUpdateTaskID,
		PostUpdateTaskID:      r.PostUpdateTaskID,
		CleanupAfterUpdate:    r.CleanupAfterUpdate,
		HealthcheckTimeoutSec: r.HealthcheckTimeoutSec,
		RollbackOnFailure:     r.RollbackOnFailure,
		RegistryCredentialsID: r.RegistryCredentialsID,
	}
}

type ReleaseTrackerExecution struct {
	ID          string     `json:"id"`
	TrackerID   string     `json:"tracker_id"`
	CommandID   *string    `json:"command_id,omitempty"`
	TagName     string     `json:"tag_name"`
	ReleaseURL  string     `json:"release_url"`
	ReleaseName string     `json:"release_name"`
	Status      string     `json:"status"`
	TriggeredAt time.Time  `json:"triggered_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// RegistryCredential stores authentication for polling private image registries.
// Password is write-only at the API boundary (omitted from list/get responses).
type RegistryCredential struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	RegistryHost string    `json:"registry_host"`
	Username     string    `json:"username"`
	Password     string    `json:"password,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// RegistryCredentialRequest is the create/update body for a registry credential —
// the writable subset of RegistryCredential (id, created_at, updated_at are
// server-managed). Password is required on create; an empty value on update keeps
// the stored secret.
type RegistryCredentialRequest struct {
	Name         string `json:"name"`
	RegistryHost string `json:"registry_host"`
	Username     string `json:"username"`
	Password     string `json:"password,omitempty"`
}

// ToModel maps the request onto a RegistryCredential.
func (r RegistryCredentialRequest) ToModel() RegistryCredential {
	return RegistryCredential{
		Name:         r.Name,
		RegistryHost: r.RegistryHost,
		Username:     r.Username,
		Password:     r.Password,
	}
}

// TrackableContainer is a compose-managed container discovered across hosts
// that does not yet have a release tracker — used to pre-fill bulk creation.
type TrackableContainer struct {
	HostID         string `json:"host_id"`
	HostName       string `json:"host_name"`
	Image          string `json:"image"`
	ImageTag       string `json:"image_tag"`
	ComposeProject string `json:"compose_project"`
	ComposeService string `json:"compose_service"`
}

type ReleaseVersionHistoryItem struct {
	Version     string     `json:"version"`
	Name        string     `json:"name,omitempty"`
	ReleaseURL  string     `json:"release_url,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

// ========== GitHub Release Info (used by gitprovider) ==========

type GitHubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
}
