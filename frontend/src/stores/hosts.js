import { ref } from 'vue'
import { defineStore } from 'pinia'
import apiClient from '../api'

const TTL_MS = 60_000 // 1 minute

export const useHostsStore = defineStore('hosts', () => {
  const hosts = ref([])
  const loading = ref(false)
  const fetchedAt = ref(null)

  async function fetchHosts(force = false) {
    if (!force && fetchedAt.value && Date.now() - fetchedAt.value < TTL_MS) return
    loading.value = true
    try {
      const res = await apiClient.getHosts()
      hosts.value = res.data || []
      fetchedAt.value = Date.now()
    } catch {
      // Keep stale data on error
    } finally {
      loading.value = false
    }
  }

  function invalidate() {
    fetchedAt.value = null
  }

  function upsert(host) {
    const idx = hosts.value.findIndex((h) => h.id === host.id)
    if (idx >= 0) {
      hosts.value = [...hosts.value.slice(0, idx), host, ...hosts.value.slice(idx + 1)]
    } else {
      hosts.value = [...hosts.value, host]
    }
  }

  function remove(hostId) {
    hosts.value = hosts.value.filter((h) => h.id !== hostId)
  }

  return { hosts, loading, fetchHosts, invalidate, upsert, remove }
})
