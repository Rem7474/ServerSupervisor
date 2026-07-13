import type { NotificationItem } from '../types/generated'
import { getAlertMetricMeta } from './alertMetrics'
import { resolveIncidentHostRoute } from './incidentRouting'

type TrackerFields = Pick<NotificationItem, 'type'>
type TitleFields = Pick<NotificationItem, 'type' | 'rule_name' | 'metric'>
type RouteFields = Pick<NotificationItem, 'type' | 'tracker_id' | 'host_id' | 'metric' | 'link_host_id'>
type ResolvedFields = Pick<NotificationItem, 'type' | 'resolved_at' | 'status'>
interface ValueFields {
  metric?: NotificationItem['metric']
  value: NotificationItem['value'] | null | undefined
  value_label?: NotificationItem['value_label']
}
type HintFields = Pick<NotificationItem, 'metric' | 'clear_threshold' | 'operator'>
type IdFields = Pick<NotificationItem, 'id' | 'type'>

export function isTrackerType(item: TrackerFields): boolean {
  return item.type === 'release_tracker_detected' || item.type === 'release_tracker_execution'
}

export function notificationResolved(item: ResolvedFields): boolean {
  if (isTrackerType(item)) {
    return !!item.resolved_at || ['completed', 'success', 'failed', 'error'].includes((item.status || '').toLowerCase())
  }
  return !!item.resolved_at
}

export function trackerStatusLabel(status?: string): string {
  if (status === 'pending' || status === 'running') return 'Détection en cours'
  if (status === 'completed' || status === 'success') return 'Exécution réussie'
  if (status === 'failed' || status === 'error') return 'Exécution échouée'
  return status || 'État inconnu'
}

export function metricLabel(metric?: string): string {
  if (!metric) return '-'
  return getAlertMetricMeta(metric).label
}

// Single source of truth for a metric's display unit — was previously
// reimplemented as two different (and differently wrong) heuristics in
// useNotifications.ts (exact match, missed proxmox_guest_cpu_percent) and
// useNotificationCenter.ts (substring match, wrongly added "%" to
// disk_smart_status). getAlertMetricMeta is the canonical per-metric table
// already used by the alert rule creation form.
export function metricUnit(metric?: string): string {
  return getAlertMetricMeta(metric || '').unit
}

export function notificationTitle(item: TitleFields): string {
  if (item.rule_name) return item.rule_name
  if (item.type === 'release_tracker_detected') return 'Nouvelle version détectée'
  if (item.type === 'release_tracker_execution') return 'Exécution de tracker'
  return item.metric ? metricLabel(item.metric) : 'Notification'
}

export function notificationRoute(item: RouteFields): string {
  if (isTrackerType(item)) {
    if (item.tracker_id) return `/release-trackers/${encodeURIComponent(String(item.tracker_id))}`
    return '/git-webhooks?tab=trackers'
  }
  return resolveIncidentHostRoute(item.host_id, item.metric, item.link_host_id)
}

// Some metrics encode a state, not a percentage — formatting them with the
// generic "value + unit" fallback would show nonsense like "1.00%" for a
// SMART status. Special-case them first, fall back to the generic formatter
// (driven by metricUnit, so a metric added to alertMetrics.ts automatically
// formats correctly here too) for everything else.
export function formatIncidentValue(item: ValueFields): string {
  const { metric, value, value_label: valueLabel } = item
  if (value == null) return '-'
  if (metric === 'status_offline') return value === 1 ? 'offline' : 'online'
  if (metric === 'disk_smart_status') return Number(value) >= 1 ? 'FAILED' : 'OK'
  if (metric === 'docker_container_state') {
    if (valueLabel) return valueLabel
    const n = Number(value)
    if (n < 0.5) return 'running'
    if (n < 1.5) return 'dégradé'
    return 'critique'
  }
  if (metric === 'docker_compose_degraded_services') {
    const n = Number(value)
    return n === 1 ? '1 service dégradé' : `${n} services dégradés`
  }
  return `${Number(value).toFixed(2)}${metricUnit(metric)}`
}

// Describes the threshold the live value must cross for an active alert to
// resolve, e.g. "repasse OK ≤ 70°C" for a ">" rule.
export function resolveHint(item: HintFields): string {
  const clearThreshold = item.clear_threshold
  if (clearThreshold == null) return ''
  const formatted = formatIncidentValue({ metric: item.metric, value: clearThreshold })
  const op = item.operator || ''
  if (op === '>' || op === '>=') return `repasse OK ≤ ${formatted}`
  if (op === '<' || op === '<=') return `repasse OK ≥ ${formatted}`
  return `seuil de résolution ${formatted}`
}

export function isUnread(item: Pick<NotificationItem, 'triggered_at'>, readAt: string | null): boolean {
  return !readAt || new Date(item.triggered_at ?? 0) > new Date(readAt)
}

// Alert-incident items carry a synthetic "alert:<numeric id>" id (see
// server/internal/alerts/notify.go's `fmt.Sprintf("alert:%d", incID)`) — the
// resolve endpoint (POST /v1/alerts/incidents/:id/resolve) does a strict
// strconv.ParseInt on :id server-side and 400s on the prefixed form. Tracker
// items are never resolvable through this endpoint (they have no incident
// row to close). Centralizing this avoids the two different behaviors that
// existed before: one caller stripped the prefix, another sent it raw and
// silently failed against the server's numeric parse.
export function resolvableIncidentId(item: IdFields): string | null {
  if (isTrackerType(item)) return null
  const raw = String(item.id ?? '')
  return raw.startsWith('alert:') ? raw.slice('alert:'.length) : raw || null
}
