import { ref, computed, reactive } from 'vue'
import apiClient from '../api'
import { getApiErrorMessage } from '../api/client'
import type { DiscoveredHost } from '../types/discovery'

interface BulkResultItem {
  name: string
  ip_address: string
  created: boolean
  host_id?: string
  api_key?: string
  error?: string
}

// Subnet ping-sweep: scan a CIDR for live addresses, then let the admin
// confirm a batch of the discovered-but-unregistered ones as new hosts in
// one call — the bulk counterpart to useAddHost's single-host flow.
export function useNetworkDiscovery() {
  const cidr = ref('')
  const scanning = ref(false)
  const scanError = ref('')
  const results = ref<DiscoveredHost[]>([])
  const hasScanned = ref(false)

  // ip_address -> editable name, defaults to the ip itself.
  const names = reactive<Record<string, string>>({})
  const selected = ref<Set<string>>(new Set())

  const candidates = computed<DiscoveredHost[]>(() =>
    results.value.filter((r) => r.responded && !r.already_registered)
  )
  const allSelected = computed(
    () => candidates.value.length > 0 && selected.value.size === candidates.value.length
  )

  async function scan(): Promise<void> {
    const value = cidr.value.trim()
    if (!value) return
    scanning.value = true
    scanError.value = ''
    results.value = []
    selected.value = new Set()
    try {
      const res = await apiClient.discoverHosts(value)
      results.value = res.data?.results ?? []
      hasScanned.value = true
      for (const r of results.value) {
        if (r.responded && !r.already_registered && !names[r.ip_address]) {
          names[r.ip_address] = r.ip_address
        }
      }
    } catch (e: unknown) {
      scanError.value = getApiErrorMessage(e, 'Erreur lors du scan réseau')
    } finally {
      scanning.value = false
    }
  }

  function toggleSelected(ip: string): void {
    const next = new Set(selected.value)
    if (next.has(ip)) next.delete(ip)
    else next.add(ip)
    selected.value = next
  }

  function toggleSelectAll(): void {
    selected.value = allSelected.value
      ? new Set()
      : new Set(candidates.value.map((c) => c.ip_address))
  }

  const adding = ref(false)
  const addError = ref('')
  const bulkResults = ref<BulkResultItem[] | null>(null)

  async function addSelected(): Promise<void> {
    const hosts = candidates.value
      .filter((c) => selected.value.has(c.ip_address))
      .map((c) => ({
        name: (names[c.ip_address] || c.ip_address).trim() || c.ip_address,
        ip_address: c.ip_address,
      }))
    if (!hosts.length) return
    adding.value = true
    addError.value = ''
    try {
      const res = await apiClient.registerHostsBulk(hosts)
      bulkResults.value = res.data?.results ?? []
    } catch (e: unknown) {
      addError.value = getApiErrorMessage(e, "Erreur lors de l'ajout des hôtes")
    } finally {
      adding.value = false
    }
  }

  function reset(): void {
    cidr.value = ''
    results.value = []
    hasScanned.value = false
    selected.value = new Set()
    for (const key of Object.keys(names)) delete names[key]
    bulkResults.value = null
    addError.value = ''
    scanError.value = ''
  }

  return {
    cidr,
    scanning,
    scanError,
    results,
    hasScanned,
    candidates,
    names,
    selected,
    allSelected,
    scan,
    toggleSelected,
    toggleSelectAll,
    adding,
    addError,
    bulkResults,
    addSelected,
    reset,
  }
}
