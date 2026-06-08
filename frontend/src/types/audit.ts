// Audit & command-history domain types — mirror server/internal/models/command.go
// (RemoteCommand, AuditLog) and database.RemoteCommandWithHost.

export interface RemoteCommand {
  id: string
  host_id: string
  module: string // docker | apt | systemd | journal | processes | custom
  action: string
  target: string
  payload: string
  status: string // pending | running | completed | failed
  output: string
  triggered_by: string
  audit_log_id?: number
  scheduled_task_id?: string
  created_at: string
  started_at: string | null
  ended_at: string | null
}

export interface RemoteCommandWithHost extends RemoteCommand {
  host_name: string
}

export interface AuditLog {
  id: number
  username: string
  action: string
  host_id: string
  host_name: string
  ip_address: string
  details: string
  status: string
  created_at: string
}
