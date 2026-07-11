import { ref, Ref } from 'vue'
import { defineStore } from 'pinia'
import apiClient from '../api'
import type { Host } from '../types/host'

const TTL_MS = 60_000 // 1 minute

export const useHostsStore = defineStore('hosts', () => {
  const hosts: Ref<Host[]> = ref([])
  const loading: Ref<boolean> = ref(false)
  const fetchedAt: Ref<number | null> = ref(null)

  async function fetchHosts(force: boolean = false): Promise<void> {
    if (!force && fetchedAt.value && Date.now() - fetchedAt.value < TTL_MS) return
    loading.value = true
    try {
      const res = await apiClient.getHosts()
      hosts.value = res.data || []
      fetchedAt.value = Date.now()
    } catch (err) {
      console.error('[hosts] failed to fetch hosts, keeping stale data:', err)
    } finally {
      loading.value = false
    }
  }

  // Bulk-replaces the host list from a source at least as fresh as a REST
  // fetch (namely the dashboard WebSocket snapshot — see useDashboard.ts).
  // Marks the data fresh so fetchHosts() doesn't immediately re-fetch over
  // REST right after a live push. This store is the single source of truth
  // for host status app-wide; stores/dashboard.ts reads through it instead
  // of keeping its own independent copy, so the navbar badge and the
  // dashboard KPIs can never disagree about which hosts are online.
  function setHosts(nextHosts: Host[]): void {
    hosts.value = nextHosts
    fetchedAt.value = Date.now()
  }

  function invalidate(): void {
    fetchedAt.value = null
  }

  function upsert(host: Host): void {
    const idx = hosts.value.findIndex((h) => h.id === host.id)
    if (idx >= 0) {
      hosts.value = [...hosts.value.slice(0, idx), host, ...hosts.value.slice(idx + 1)]
    } else {
      hosts.value = [...hosts.value, host]
    }
  }

  function remove(hostId: string): void {
    hosts.value = hosts.value.filter((h) => h.id !== hostId)
  }

  return { hosts, loading, fetchHosts, setHosts, invalidate, upsert, remove }
})
