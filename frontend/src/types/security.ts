// Security / web-logs domain types.
// Most web-logs endpoints return ad-hoc aggregates (summary / timeseries /
// domain details / live) built dynamically server-side (map[string]any), so they
// stay loosely typed. The IP timeline is the exception: GetIPTimeline returns the
// WebLogIPTimelineRow model (generated.ts).
import type { WebLogIPTimelineRow } from './generated'

export type { WebLogIPTimelineRow } from './generated'

/** Response of GET /security/web-logs/ip/:ip (envelope, not a Go model). */
export interface IPTimelineResponse {
  ip: string
  host_id: string
  period: string
  since: string
  count: number
  requests: WebLogIPTimelineRow[]
}

/** Optional query params for GET /security/web-logs/domain/:domain beyond
 * domain+period — status/method/path/ip narrow every aggregate the endpoint
 * returns (KPIs, top paths/clients, the requests page), sort/dir/page/limit
 * only affect the requests page (see DomainDetailsFilter in
 * server/internal/database/db_web_logs_detail.go). */
export interface DomainDetailsParams {
  hostId?: string
  source?: string
  page?: number
  limit?: number
  status?: '' | '2xx' | '3xx' | '4xx' | '5xx' | 'blocked' | 'suspicious'
  method?: string
  path?: string
  ip?: string
  sort?: 'time' | 'status' | 'bytes'
  dir?: 'asc' | 'desc'
}
