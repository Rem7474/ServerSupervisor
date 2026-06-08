// Git-webhook domain types — mirror server/internal/models/webhook.go.

export interface GitWebhookExecution {
  id: string
  webhook_id: string
  command_id?: string
  provider: string
  repo_name: string
  branch: string
  commit_sha: string
  commit_message: string
  pusher: string
  status: string
  triggered_at: string
  completed_at?: string
}

export interface GitWebhook {
  id: string
  name: string
  secret?: string // only returned when explicitly fetched (create / regenerate)
  provider: string
  repo_filter: string
  branch_filter: string
  event_filter: string
  host_id: string
  custom_task_id: string
  notify_channels: string[]
  notify_on_success: boolean
  notify_on_failure: boolean
  enabled: boolean
  last_triggered_at?: string
  created_at: string
  host_name?: string
  last_execution?: GitWebhookExecution
}
