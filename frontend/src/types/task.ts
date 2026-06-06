// Scheduled-task domain types — mirror server/internal/models/task.go.

export interface ScheduledTask {
  id: string
  host_id: string
  name: string
  module: string // apt | docker | systemd | journal | processes | custom
  action: string
  target: string
  payload: string // JSON extra args
  cron_expression: string
  enabled: boolean
  last_run_at: string | null
  next_run_at: string | null
  last_run_status: string | null // completed | failed | null
  last_command_id?: string
  created_by: string
  created_at: string
}

export interface ScheduledTaskWithHost extends ScheduledTask {
  host_name: string
}

/** Lightweight custom task (id + name) declared in an agent's tasks.yaml. */
export interface CustomTaskSummary {
  id: string
  name: string
}
