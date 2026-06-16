// Audit & command-history domain types. RemoteCommand and AuditLog come from the
// generated Go models; RemoteCommandWithHost lives in the database package (not
// models, so not generated) and is defined here as an extension.
import type { RemoteCommand } from './generated'

export type { RemoteCommand, AuditLog } from './generated'

export interface RemoteCommandWithHost extends RemoteCommand {
  host_name: string
}

/** Merged chronological event for the host timeline feed. */
export interface HostTimelineEvent {
  id: string
  type: 'audit' | 'command' | 'incident'
  timestamp: string
  title: string
  detail?: string
  status?: string
  severity?: string
  module?: string
}
