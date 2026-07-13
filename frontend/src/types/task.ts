// Scheduled-task domain types — re-exported from the generated Go models.
import type { ScheduledTask } from './generated'

export type { ScheduledTask, CustomTaskSummary, ScheduledTaskRequest } from './generated'

// tygo renders the embedded models.ScheduledTask as a nested property rather than
// flattening it (Go JSON inlines anonymous embeds), so define the flat shape here.
export type ScheduledTaskWithHost = ScheduledTask & { host_name: string }

/** One row of GET /scheduled-tasks/:id/executions — a remote_commands record, not a generated model. */
export interface ScheduledTaskExecution {
  id: string | number
  created_at: string
  status: string
  started_at?: string
  ended_at?: string
  triggered_by?: string
  output?: string
}
