// Audit & command-history domain types. RemoteCommand and AuditLog come from the
// generated Go models; RemoteCommandWithHost lives in the database package (not
// models, so not generated) and is defined here as an extension.
import type { RemoteCommand, HostTimelineEvent as GeneratedHostTimelineEvent } from './generated'

export type { RemoteCommand, AuditLog } from './generated'

export interface RemoteCommandWithHost extends RemoteCommand {
  host_name: string
}

// Generation can't express the Go doc comment's "type is one of ..." union
// (it's just a string field on the wire) — narrow it here.
export interface HostTimelineEvent extends GeneratedHostTimelineEvent {
  type: 'audit' | 'command' | 'incident'
}
