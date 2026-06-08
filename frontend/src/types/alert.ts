// Alert domain types. Model shapes come from the generated Go models (generated.ts).
// AlertRulePayload is a relaxed (all-optional) create/update body that the rule
// form builds incrementally — the generated AlertRuleCreate requires every field,
// so it isn't a drop-in replacement and stays defined here.
import type { AlertSourceType, ProxmoxMetricScope, AlertActions } from './generated'

export type {
  AlertRule,
  AlertActions,
  ProxmoxMetricScope,
  CommandTrigger,
  AlertIncident,
  AlertSourceType,
} from './generated'

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
