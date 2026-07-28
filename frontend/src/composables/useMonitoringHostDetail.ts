import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { npmApi } from '../api/npm'
import { getApiErrorMessage } from '../api/client'
import type { NPMProxyHostEnriched } from '../types/npm'

// No single-host GET endpoint exists (see api/npm.ts) — the list is small
// (every proxy host across every connected NPM instance), same tradeoff
// useProxmoxGuest.ts already makes fetching all guests to find one by id.
export function useMonitoringHostDetail() {
  const route = useRoute()
  const hostId = route.params.id as string

  const host = ref<NPMProxyHostEnriched | null>(null)
  const loading = ref(true)
  const error = ref('')

  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const res = await npmApi.listAllProxyHosts()
      const found = (res.data.proxy_hosts ?? []).find((h) => h.id === hostId)
      if (!found) {
        error.value = 'Proxy host introuvable.'
        return
      }
      host.value = found
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e, 'Impossible de charger le proxy host.')
    } finally {
      loading.value = false
    }
  }

  onMounted(load)

  return {
    host,
    loading,
    error,
  }
}
