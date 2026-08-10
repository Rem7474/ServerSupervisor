// Network flows ("top talkers") domain types. The base shapes are generated
// from the Go models (see generated.ts); this file re-exports them and adds
// the narrowed unions generation can't express plus the response envelopes
// (not Go models) the /network/flows/* endpoints return.
import type {
  NetworkFlowMetric as GeneratedNetworkFlowMetric,
  NetworkFlowSummaryPoint,
} from './generated'

export type NetworkFlowProtocol = 'tcp' | 'udp'
export type NetworkFlowDirection = 'inbound' | 'outbound'

/** NetworkFlowMetric with protocol/direction narrowed to their known value sets. */
export type NetworkFlowMetric = Omit<GeneratedNetworkFlowMetric, 'protocol' | 'direction'> & {
  protocol?: NetworkFlowProtocol
  direction?: NetworkFlowDirection
}

export type { NetworkFlowSummaryPoint }

/** GET /v1/hosts/:id/network/flows/summary response envelope. */
export interface NetworkFlowsSummaryResponse {
  hours: number
  points: NetworkFlowSummaryPoint[]
}

/** GET /v1/hosts/:id/network/flows/history response envelope. */
export interface NetworkFlowsHistoryResponse {
  hours: number
  remote_ip: string
  remote_port: number
  protocol: string
  points: NetworkFlowSummaryPoint[]
}
