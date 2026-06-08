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
