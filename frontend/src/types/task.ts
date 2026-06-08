// Scheduled-task domain types — re-exported from the generated Go models.
import type { ScheduledTask } from './generated'

export type { ScheduledTask, CustomTaskSummary, ScheduledTaskRequest } from './generated'

// tygo renders the embedded models.ScheduledTask as a nested property rather than
// flattening it (Go JSON inlines anonymous embeds), so define the flat shape here.
export type ScheduledTaskWithHost = ScheduledTask & { host_name: string }
