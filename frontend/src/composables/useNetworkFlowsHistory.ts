import api from '../api'
import type { NetworkFlowSummaryPoint } from '../types/networkFlows'

/** Total tracked bandwidth for a host over time (every talker + "others" summed). */
export async function fetchNetworkFlowsSummary(hostId: string, hours: number): Promise<NetworkFlowSummaryPoint[]> {
  const res = await api.getNetworkFlowsSummary(hostId, hours)
  return Array.isArray(res.data?.points) ? res.data.points : []
}

/** One talker's bandwidth over time, for the drill-down chart. */
export async function fetchNetworkFlowsHistory(
  hostId: string,
  remoteIp: string,
  remotePort: number,
  protocol: string,
  hours: number,
): Promise<NetworkFlowSummaryPoint[]> {
  const res = await api.getNetworkFlowsHistory(hostId, { remote_ip: remoteIp, remote_port: remotePort, protocol, hours })
  return Array.isArray(res.data?.points) ? res.data.points : []
}
