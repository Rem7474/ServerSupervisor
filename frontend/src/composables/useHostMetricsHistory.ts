import { toValue, type MaybeRef } from 'vue'
import api from '../api'

export interface MetricsHistoryPoint {
  timestamp: string
  cpu_usage_percent?: number
  memory_percent?: number
  memory_used?: number
  memory_total?: number
}

export async function fetchMetricsHistory(
  hostId: MaybeRef<string>,
  hours: number,
  metricsSource: string,
  proxmoxGuestId?: string | null,
): Promise<MetricsHistoryPoint[]> {
  const id = toValue(hostId)
  if (metricsSource === 'proxmox' && proxmoxGuestId) {
    // Keep ≤24h buckets under 5 min so the backend serves them from the raw
    // proxmox_guest_metrics table (fresh, fine-grained) rather than the 5-min
    // CAGG — matching the agent CPU/RAM and disk charts, which both read raw for
    // ≤24h. Coarser ranges (>24h) still use ≥5-min buckets → CAGG re-bucketed.
    const bucketMinutes = hours <= 1 ? 1 : hours <= 6 ? 2 : hours <= 24 ? 3 : hours <= 168 ? 30 : 60
    const res = await api.getProxmoxGuestMetrics(proxmoxGuestId, hours, bucketMinutes)
    const raw: Record<string, unknown>[] = Array.isArray(res.data) ? res.data : []
    return raw.map((p) => ({
      timestamp: p.timestamp as string,
      cpu_usage_percent: p.cpu_avg as number | undefined,
      memory_percent: p.memory_avg as number | undefined,
    }))
  }
  if (hours > 24) {
    const res = await api.getMetricsAggregated(id, hours)
    return Array.isArray(res.data?.metrics) ? res.data.metrics : []
  }
  const res = await api.getMetricsHistory(id, hours)
  return Array.isArray(res.data) ? res.data : []
}
