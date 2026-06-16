import api from '../api'

export interface DiskHistoryPoint {
  timestamp: string
  used_percent: number
  used_gb?: number
  size_gb?: number
}

export async function fetchDiskMetricsHistory(
  hostId: string,
  mount: string,
  hours: number,
): Promise<DiskHistoryPoint[]> {
  const res = await api.getDiskMetricsAggregated(hostId, mount, hours)
  return Array.isArray(res.data?.points) ? res.data.points : []
}
