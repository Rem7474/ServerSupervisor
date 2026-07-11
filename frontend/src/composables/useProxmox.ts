import { ref, computed, onMounted, onUnmounted } from 'vue'
import api from '../api'
import { isApiAbort } from '../api/client'
import { useAbortSignal } from './useAbortSignal'
import type { ProxmoxSummary, ProxmoxNode, ProxmoxConnection } from '../types/proxmox'

const PROXMOX_REFRESH_SEC = 30

export function useProxmox() {
  const signal = useAbortSignal()
  const summary = ref<Partial<ProxmoxSummary>>({})
  const nodes = ref<ProxmoxNode[]>([])
  const instances = ref<ProxmoxConnection[]>([])
  const filterConnection = ref('')
  const loading = ref(true)
  const error = ref('')
  const autoRefresh = ref(true)
  const lastUpdatedAt = ref<Date | null>(null)
  let refreshTimer: ReturnType<typeof setInterval> | null = null
  const nodeSortKey = ref<string>('node_name')
  const nodeSortDir = ref<'asc' | 'desc'>('asc')

  const filteredNodes = computed(() =>
    filterConnection.value
      ? nodes.value.filter((n) => n.connection_id === filterConnection.value)
      : nodes.value
  )

  const sortedNodes = computed(() => {
    const list = [...filteredNodes.value]
    const dir = nodeSortDir.value === 'asc' ? 1 : -1
    list.sort((a, b) => {
      let aVal: number | string
      let bVal: number | string
      switch (nodeSortKey.value) {
        case 'node_name':
        case 'cluster_name':
        case 'status':
          aVal = String(a[nodeSortKey.value] || '').toLowerCase()
          bVal = String(b[nodeSortKey.value] || '').toLowerCase()
          break
        case 'last_seen_at':
          aVal = a.last_seen_at ? new Date(a.last_seen_at).getTime() : 0
          bVal = b.last_seen_at ? new Date(b.last_seen_at).getTime() : 0
          break
        default:
          aVal = Number((a as Record<string, unknown>)[nodeSortKey.value] || 0)
          bVal = Number((b as Record<string, unknown>)[nodeSortKey.value] || 0)
          break
      }
      if (aVal < bVal) return -1 * dir
      if (aVal > bVal) return 1 * dir
      return 0
    })
    return list
  })

  function toggleNodeSort(key: string): void {
    if (nodeSortKey.value === key) {
      nodeSortDir.value = nodeSortDir.value === 'asc' ? 'desc' : 'asc'
      return
    }
    nodeSortKey.value = key
    nodeSortDir.value = 'asc'
  }

  const hasHealthAlerts = computed(() =>
    (summary.value.nodes_down ?? 0) > 0 ||
    (summary.value.storage_near_full ?? 0) > 0 ||
    (summary.value.storage_offline ?? 0) > 0 ||
    (summary.value.recent_failed_tasks ?? 0) > 0
  )

  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const [sumRes, nodesRes, instRes] = await Promise.allSettled([
        api.getProxmoxSummary(signal),
        api.getProxmoxNodes(undefined, signal),
        api.getProxmoxInstances(signal),
      ])
      // Component unmounted mid-flight: bail without touching reactive state.
      if ([sumRes, nodesRes, instRes].some(r => r.status === 'rejected' && isApiAbort(r.reason))) return
      if (sumRes.status === 'fulfilled') summary.value = sumRes.value.data
      if (nodesRes.status === 'fulfilled') nodes.value = nodesRes.value.data
      if (instRes.status === 'fulfilled') {
        instances.value = instRes.value.data
      } else if (instRes.reason?.response?.status !== 403) {
        error.value = instRes.reason?.response?.data?.error || 'Erreur lors du chargement.'
      }
      if (sumRes.status === 'rejected' && sumRes.reason?.response?.status !== 403) {
        error.value = sumRes.reason?.response?.data?.error || 'Erreur lors du chargement.'
      }
      if (nodesRes.status === 'rejected' && nodesRes.reason?.response?.status !== 403) {
        error.value = nodesRes.reason?.response?.data?.error || 'Erreur lors du chargement.'
      }
    } finally {
      loading.value = false
      lastUpdatedAt.value = new Date()
    }
  }

  function startRefreshTimer(): void {
    stopRefreshTimer()
    refreshTimer = setInterval(() => {
      if (autoRefresh.value) load()
    }, PROXMOX_REFRESH_SEC * 1000)
  }

  function stopRefreshTimer(): void {
    if (refreshTimer) { clearInterval(refreshTimer); refreshTimer = null }
  }

  onMounted(() => { load(); startRefreshTimer() })
  onUnmounted(stopRefreshTimer)

  return {
    summary,
    instances,
    filterConnection,
    loading,
    error,
    autoRefresh,
    lastUpdatedAt,
    PROXMOX_REFRESH_SEC,
    nodeSortKey,
    nodeSortDir,
    sortedNodes,
    toggleNodeSort,
    hasHealthAlerts,
    load,
  }
}
