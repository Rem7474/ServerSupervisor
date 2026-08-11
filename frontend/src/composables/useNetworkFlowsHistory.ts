import api from '../api'
import type { TimeRange } from '../api/client'
import type { NetworkFlowSummaryPoint } from '../types/networkFlows'

/** Total tracked bandwidth for a host over time (every talker + "others" summed). */
export async function fetchNetworkFlowsSummary(hostId: string, period: string, range?: TimeRange): Promise<NetworkFlowSummaryPoint[]> {
  const res = await api.getNetworkFlowsSummary(hostId, period, range)
  return Array.isArray(res.data?.points) ? res.data.points : []
}

/** One talker's bandwidth over time, for the drill-down chart. */
export async function fetchNetworkFlowsHistory(
  hostId: string,
  remoteIp: string,
  remotePort: number,
  protocol: string,
  period: string,
  range?: TimeRange,
): Promise<NetworkFlowSummaryPoint[]> {
  const res = await api.getNetworkFlowsHistory(hostId, { remote_ip: remoteIp, remote_port: remotePort, protocol, period }, range)
  return Array.isArray(res.data?.points) ? res.data.points : []
}
