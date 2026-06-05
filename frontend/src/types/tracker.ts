// Release-tracker domain types — mirror server/internal/models/tracker.go.

export interface ReleaseTrackerExecution {
  id: string
  tracker_id: string
  command_id?: string
  tag_name: string
  release_url: string
  release_name: string
  status: string
  triggered_at: string
  completed_at?: string
}

export interface ReleaseTracker {
  id: string
  name: string
  tracker_type: string // 'git' | 'docker'
  provider: string
  repo_owner: string
  repo_name: string
  docker_image: string
  docker_tag: string
  host_id: string
  custom_task_id: string
  last_release_tag: string
  latest_image_digest?: string
  cooldown_hours: number
  last_release_detected_at?: string
  last_checked_at?: string
  last_triggered_at?: string
  last_error?: string
  notify_channels: string[]
  notify_on_release: boolean
  enabled: boolean
  created_at: string
  host_name?: string
  last_execution?: ReleaseTrackerExecution
  // Compose update mode
  update_action: string
  compose_project?: string
  compose_service?: string
  pre_update_task_id?: string
  post_update_task_id?: string
  cleanup_after_update: boolean
  healthcheck_timeout_sec: number
  rollback_on_failure: boolean
  registry_credentials_id?: string
}

export interface RegistryCredential {
  id: string
  name: string
  registry_host: string
  username: string
  password?: string
  created_at: string
  updated_at: string
}

export interface TrackableContainer {
  host_id: string
  host_name: string
  image: string
  image_tag: string
  compose_project: string
  compose_service: string
}

export interface ReleaseVersionHistoryItem {
  version: string
  name?: string
  release_url?: string
  published_at?: string
}
