import { ref, toValue, type MaybeRef } from 'vue'
import api from '../api'

export interface DiskMetric {
  mount_point: string
  filesystem?: string
  used_gb: number
  size_gb: number
  used_percent: number
  inodes_total: number
  inodes_used: number
  inodes_percent: number
}

export function useDiskMetrics(hostId: MaybeRef<string>, initialData?: DiskMetric[] | null) {
  const metrics = ref<DiskMetric[]>(initialData ?? [])
  const loading = ref(!initialData)

  async function load(): Promise<void> {
    loading.value = true
    try {
      const res = await api.getDiskMetrics(toValue(hostId))
      metrics.value = res.data || []
    } catch (err) {
      console.error('Failed to load disk metrics:', err)
    } finally {
      loading.value = false
    }
  }

  return { metrics, loading, load }
}
