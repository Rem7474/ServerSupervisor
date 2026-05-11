/**
 * Single source of truth for status/state → CSS class mappings.
 * Two semantic categories:
 *  - Entity state  : is something alive? (host, container, service, Proxmox guest)
 *  - Execution state: is a task/command done? (pending → running → completed/failed)
 */

// ─── Entity states ────────────────────────────────────────────────────────────

const ENTITY_STATE_MAP: Record<string, string> = {
  // Host
  online:     'badge bg-green-lt text-green',
  offline:    'badge bg-red-lt text-red',
  // Docker container
  running:    'badge bg-green-lt text-green',
  restarting: 'badge bg-yellow-lt text-yellow',
  paused:     'badge bg-yellow-lt text-yellow',
  created:    'badge bg-blue-lt text-blue',
  exited:     'badge bg-secondary-lt text-secondary',
  dead:       'badge bg-red-lt text-red',
  removing:   'badge bg-orange-lt text-orange',
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

// ─── Execution / command states ───────────────────────────────────────────────

const EXECUTION_STATE_MAP: Record<string, string> = {
  pending:   'badge bg-yellow-lt text-yellow',
  running:   'badge bg-blue-lt text-blue',
  completed: 'badge bg-green-lt text-green',
  success:   'badge bg-green-lt text-green',
  succeeded: 'badge bg-green-lt text-green',
  failed:    'badge bg-red-lt text-red',
  error:     'badge bg-red-lt text-red',
  skipped:   'badge bg-secondary-lt text-secondary',
}

export function getExecutionStateClass(
  status: string | null | undefined,
  fallback = 'badge bg-secondary-lt text-secondary'
): string {
  if (!status) return fallback
  return EXECUTION_STATE_MAP[status.toLowerCase()] ?? fallback
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
