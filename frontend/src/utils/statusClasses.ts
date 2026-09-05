import { i18n } from '../i18n'

/**
 * Single source of truth for status/state → CSS class mappings.
 * Two semantic categories:
 *  - Entity state  : is something alive? (host, container, service, Proxmox guest)
 *  - Execution state: is a task/command done? (pending → running → completed/failed)
 */

const { t } = i18n.global

// ─── Entity states ────────────────────────────────────────────────────────────

const ENTITY_STATE_MAP: Record<string, string> = {
  // Host
  online:     'badge bg-success-lt text-success',
  offline:    'badge bg-danger-lt text-danger',
  // Docker container
  running:    'badge bg-success-lt text-success',
  restarting: 'badge bg-warning-lt text-warning',
  paused:     'badge bg-warning-lt text-warning',
  created:    'badge bg-primary-lt text-primary',
  exited:     'badge bg-secondary-lt text-secondary',
  dead:       'badge bg-danger-lt text-danger',
  removing:   'badge bg-warning-lt text-warning',
  // Proxmox guest
  stopped:    'badge bg-secondary-lt text-secondary',
}

export function getEntityStateClass(
  state: string | null | undefined,
  fallback = 'badge bg-secondary-lt text-secondary'
): string {
  if (!state) return fallback
  return ENTITY_STATE_MAP[state.toLowerCase()] ?? fallback
}

// Labels for the same entity states — kept alongside the class map so a
// badge's color and text can't drift apart the way they previously did
// (Docker showed "running" translated for a running container, Proxmox
// showed the raw "running" for the exact same concept on a guest). Built as
// a function (not a static Record) so it re-evaluates t() on every call
// instead of freezing translations to whatever locale was active at import
// time.
function entityStateLabels(): Record<string, string> {
  return {
    online:     t('common.statusOnline'),
    offline:    t('common.statusOffline'),
    running:    t('common.stateRunning'),
    restarting: t('common.stateRestarting'),
    paused:     t('common.statePaused'),
    created:    t('common.stateCreated'),
    exited:     t('common.stateExited'),
    dead:       t('common.stateDead'),
    removing:   t('common.stateRemoving'),
    stopped:    t('common.stateExited'),
  }
}

export function getEntityStateLabel(state: string | null | undefined, fallback?: string): string {
  if (!state) return fallback ?? ''
  return entityStateLabels()[state.toLowerCase()] ?? fallback ?? state
}

// ─── Execution / command states ───────────────────────────────────────────────

const EXECUTION_STATE_MAP: Record<string, string> = {
  pending:   'badge bg-warning-lt text-warning',
  running:   'badge bg-primary-lt text-primary',
  completed: 'badge bg-success-lt text-success',
  success:   'badge bg-success-lt text-success',
  succeeded: 'badge bg-success-lt text-success',
  // Proxmox's own vzdump backup runs report "OK", not "completed"/"success".
  ok:        'badge bg-success-lt text-success',
  failed:    'badge bg-danger-lt text-danger',
  error:     'badge bg-danger-lt text-danger',
  skipped:   'badge bg-secondary-lt text-secondary',
  cancelled: 'badge bg-secondary-lt text-secondary',
}

export function getExecutionStateClass(
  status: string | null | undefined,
  fallback = 'badge bg-secondary-lt text-secondary'
): string {
  if (!status) return fallback
  return EXECUTION_STATE_MAP[status.toLowerCase()] ?? fallback
}

function executionStateLabels(): Record<string, string> {
  return {
    pending:   t('common.statePending'),
    running:   t('common.stateRunning'),
    completed: t('common.stateCompleted'),
    success:   t('common.stateSuccess'),
    succeeded: t('common.stateSuccess'),
    ok:        t('common.stateOk'),
    failed:    t('common.stateFailed'),
    error:     t('common.error'),
    skipped:   t('common.stateSkipped'),
    cancelled: t('common.stateCancelled'),
  }
}

export function getExecutionStateLabel(status: string | null | undefined, fallback?: string): string {
  if (!status) return fallback ?? ''
  return executionStateLabels()[status.toLowerCase()] ?? fallback ?? status
}

/**
 * Color-only variant (no `badge` prefix) — use when the element already
 * carries the `badge` class: `<span class="badge" :class="execBadgeColor(s)">`
 */
export function execBadgeColor(
  status: string | null | undefined,
  fallback = 'bg-secondary-lt text-secondary'
): string {
  const full = getExecutionStateClass(status)
  return full === 'badge bg-secondary-lt text-secondary' ? fallback : full.replace('badge ', '')
}
