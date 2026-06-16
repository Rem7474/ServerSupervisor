import { ref, toValue, type MaybeRef } from 'vue'
import api from '../api'

export interface DiskHealth {
  device: string
  model?: string
  serial_number?: string
  smart_status: string
  temperature: number
  power_on_hours: number
  power_cycles?: number
  realloc_sectors: number
  pending_sectors: number
  uncorrectable_sectors?: number
  percentage_used?: number
}

export function useDiskHealth(hostId: MaybeRef<string>, initialData?: DiskHealth[] | null) {
  const health = ref<DiskHealth[]>(initialData ?? [])
  const loading = ref(!initialData)

  async function load(): Promise<void> {
    loading.value = true
    try {
      const res = await api.getDiskHealth(toValue(hostId))
      health.value = res.data || []
    } catch (err) {
      console.error('Failed to load disk health:', err)
    } finally {
      loading.value = false
    }
  }

  return { health, loading, load }
}
