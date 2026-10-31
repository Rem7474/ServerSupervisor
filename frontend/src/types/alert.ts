// Alert domain types. Model shapes come from the generated Go models (generated.ts).
// AlertRulePayload is a relaxed (all-optional) create/update body that the rule
// form builds incrementally — the generated AlertRuleCreate requires every field,
// so it isn't a drop-in replacement and stays defined here.
import type { AlertSourceType, ProxmoxMetricScope, AlertActions, AlertMetricCapability, AlertProxmoxScope } from './generated'

export type {
  AlertRule,
  AlertActions,
  ProxmoxMetricScope,
  CommandTrigger,
  AlertIncident,
  AlertSourceType,
  AlertMetricCapability,
  AlertScopeOption,
  AlertProxmoxScope,
  AlertSplitCapabilities,
  AlertDockerCapabilities,
  AlertDockerHostScope,
  AlertHostCapabilities,
} from './generated'

/** GET /alert-rules/capabilities/{agent,synthetic}: a bare metric catalog. */
export interface AlertMetricCatalog {
  metrics: AlertMetricCapability[]
}

/**
 * Every metric catalog merged for the rule editor (useAlertsPage): all
 * metrics, the per-source lists, and the Proxmox scope options.
 */
export interface AlertRuleCapabilities {
  metrics: AlertMetricCapability[]
  agent_metrics: AlertMetricCapability[]
  proxmox_metrics: AlertMetricCapability[]
  synthetic_metrics: AlertMetricCapability[]
  docker_metrics: AlertMetricCapability[]
  proxmox_scope: AlertProxmoxScope
}

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
  baseline_window_seconds?: number
  actions?: AlertActions
}
