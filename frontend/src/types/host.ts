// Shared frontend domain types. Mirror the server's `models.*` JSON shapes so
// API responses are typed end to end (catches backend↔frontend field drift at
// compile time instead of at runtime). Keep in sync with server/internal/models.

/** Host lifecycle status (server: models.Host.Status). */
export type HostStatus = 'online' | 'offline' | 'warning' | 'unknown'

/**
 * A monitored host as returned by `GET /api/v1/hosts` and `GET /api/v1/hosts/:id`.
 * Mirrors server `models.Host` (the `api_key` field is never serialised).
 */
export interface Host {
  id: string
  name: string
  hostname: string
  ip_address: string
  os: string
  agent_version: string
  tags?: string[]
  status: HostStatus
  last_seen: string
  created_at: string
  updated_at: string
  /** Active metric collectors, e.g. { docker: true, smart: false }. */
  collectors?: Record<string, boolean>
}
