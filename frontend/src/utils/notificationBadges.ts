import type { NotificationItem } from '../types/generated'
import { isTrackerType, notificationResolved } from './incidentFormat'

type StateFields = Parameters<typeof notificationResolved>[0]
type TypeFields = Pick<NotificationItem, 'type' | 'severity'>

// Shared severity/type -> visual mapping, previously reimplemented (each
// slightly differently) in NotificationBell.vue, NotificationListItem.vue
// and AlertIncidentList.vue. `tone` values are BadgePill.vue's `Tone` prop —
// BadgePill has no 'primary' tone, so the tracker-detected case uses 'info'
// (renders as the same blue as everywhere else a release tracker is tagged).

export function notificationStateTone(item: StateFields): 'danger' | 'success' {
  return notificationResolved(item) ? 'success' : 'danger'
}

export function notificationStateLabel(item: StateFields): string {
  return notificationResolved(item) ? 'Terminé' : 'Actif'
}

export function notificationTypeTone(item: TypeFields): 'info' | 'secondary' | 'danger' | 'warning' {
  if (item.type === 'release_tracker_detected') return 'info'
  if (item.type === 'release_tracker_execution') return 'secondary'
  if ((item.severity || '').toLowerCase() === 'crit') return 'danger'
  if ((item.severity || '').toLowerCase() === 'warn') return 'warning'
  return 'secondary'
}

export function notificationTypeLabel(item: TypeFields): string {
  if (item.type === 'release_tracker_detected') return 'Release tracker'
  if (item.type === 'release_tracker_execution') return 'Exécution tracker'
  if ((item.severity || '').toLowerCase() === 'crit') return 'Alerte critique'
  if ((item.severity || '').toLowerCase() === 'warn') return 'Alerte avertissement'
  return '-'
}

// Solid-fill variant (icon avatar background) rather than a tinted badge —
// only NotificationBell.vue's row icon needs this; everything else renders
// through BadgePill's tone-tinted classes above.
export function notificationIconTone(item: TypeFields): string {
  if (isTrackerType(item)) return 'bg-primary text-white'
  if ((item.severity || '').toLowerCase() === 'crit') return 'bg-danger text-white'
  if ((item.severity || '').toLowerCase() === 'warn') return 'bg-warning text-white'
  return 'bg-secondary text-white'
}
