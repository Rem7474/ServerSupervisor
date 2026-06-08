// Security / web-logs domain types.
// Most web-logs endpoints return ad-hoc aggregates (summary / timeseries /
// domain details) built dynamically server-side, so they stay loosely typed.
// The IP timeline is the exception: it returns the WebLogIPTimelineRow model.

export interface WebLogIPTimelineRow {
  timestamp: string
  host_id: string
  host_name: string
  source: string
  ip: string
  method: string
  path: string
  status: number
  bytes: number
  user_agent: string
  domain: string
  category: string
  blocked?: boolean
  blocked_source?: string
  blocked_reason?: string
  blocked_at?: string
  blocked_until?: string
}

/** Response of GET /security/web-logs/ip/:ip. */
export interface IPTimelineResponse {
  ip: string
  host_id: string
  period: string
  since: string
  count: number
  requests: WebLogIPTimelineRow[]
}
