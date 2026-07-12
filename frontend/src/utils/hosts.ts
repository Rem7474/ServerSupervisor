/** Minimal shape needed to detect a never-connected host — satisfied by both
 * the full generated Host type and the leaner WS dashboard projection. */
export interface NeverConnectedHostLike {
  status?: string | null
  created_at?: string | number | Date | null
  last_seen?: string | number | Date | null
}

// A host whose last_seen is still within a few seconds of its created_at has
// never actually reported — RegisterHost stamps last_seen=now() at creation,
// and only a real agent report moves it meaningfully later than that.
const NEVER_SEEN_TOLERANCE_MS = 60_000

/** True when a host was registered but its agent has never actually reported in. */
export function isNeverConnectedHost(host: NeverConnectedHostLike): boolean {
  if (host.status !== 'offline') return false
  const created = new Date(host.created_at ?? '').getTime()
  const seen = new Date(host.last_seen ?? '').getTime()
  return Number.isFinite(created) && Number.isFinite(seen) && Math.abs(seen - created) < NEVER_SEEN_TOLERANCE_MS
}
