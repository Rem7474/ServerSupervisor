import { ref, toValue, type MaybeRef } from 'vue'
import api from '../api'
import type { NetworkFlowMetric } from '../types/networkFlows'

export function useNetworkFlows(hostId: MaybeRef<string>, initialData?: NetworkFlowMetric[] | null) {
  const talkers = ref<NetworkFlowMetric[]>(initialData ?? [])
  const loading = ref(!initialData)

  async function load(): Promise<void> {
    // Only show the big skeleton for a genuine first load (no rows yet) —
    // a background refresh (NetworkFlowsTable's refreshTick watcher) swaps
    // the rows in place instead of flashing the whole card, same guard as
    // NetworkFlowsHistoryChart.vue/DiskHistoryChart.vue's loadHistory().
    if (!talkers.value.length) loading.value = true
    try {
      const res = await api.getNetworkFlows(toValue(hostId))
      talkers.value = res.data || []
    } catch (err) {
      console.error('Failed to load network flows:', err)
    } finally {
      loading.value = false
    }
  }

  return { talkers, loading, load }
}
