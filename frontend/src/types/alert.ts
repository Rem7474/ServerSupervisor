// Alert domain types — mirror server/internal/models/alert.go.
// Note the request/response duality: the API returns AlertRule (duration_seconds,
// pointer thresholds) but create/update accept AlertRulePayload (duration,
// plain-number thresholds), which the server maps to the stored model.

export type AlertSourceType = 'agent' | 'proxmox' | 'synthetic'

export interface CommandTrigger {
  module: string
  action: string
  target?: string
  payload?: string
}

export interface ProxmoxMetricScope {
  scope_mode?: string
  connection_id?: string
  node_id?: string
  storage_id?: string
  guest_id?: string
  disk_id?: string
}

export interface AlertActions {
  channels: string[]
  smtp_to?: string
  ntfy_topic?: string
  cooldown?: number
  command_trigger?: CommandTrigger
}

/** AlertRule as returned by GET /alert-rules (stored model shape). */
export interface AlertRule {
  id: number
  name?: string
  source_type?: AlertSourceType
  host_id: string | null
  proxmox_scope?: ProxmoxMetricScope
  metric: string
  operator: string
  threshold_warn: number | null
  threshold_crit: number | null
  threshold_clear_warn?: number
  threshold_clear_crit?: number
  duration_seconds: number
  actions: AlertActions
  last_fired?: string
  enabled: boolean
  created_at: string
  updated_at?: string
  active_incident_count: number
}

/** Create/update/test body — uses `duration` and plain-number thresholds. */
export interface AlertRulePayload {
  id?: number
  name?: string
  enabled?: boolean
  source_type?: AlertSourceType
  host_id?: string
  proxmox_scope?: ProxmoxMetricScope
  metric?: string
  operator?: string
  threshold_warn?: number
  threshold_crit?: number
  threshold_clear_warn?: number
  threshold_clear_crit?: number
  duration?: number
  actions?: AlertActions
}

export interface AlertIncident {
  id: number
  rule_id: number | null
  host_id: string
  severity: string // warn | crit
  triggered_at: string
  resolved_at: string | null
  value: number
}
