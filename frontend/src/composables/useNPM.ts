import { ref, computed, onMounted } from 'vue'
import { npmApi } from '../api/npm'
import type { NPMProxyHostEnriched } from '../types/npm'
import { getApiErrorMessage } from '../api/client'
import { useConfirmDialog } from './useConfirmDialog'

export type NPMSortKey = 'connection_name' | 'domain' | 'forward' | 'npm_enabled' | 'uptime_status' | 'ssl_days_remaining'

export function useNPM() {
  const dialog = useConfirmDialog()
  const hosts = ref<NPMProxyHostEnriched[]>([])
  const loading = ref(true)
  const loadError = ref('')
  const actionError = ref('')
  const toggling = ref<Record<string, boolean>>({})
  const togglingNPM = ref<Record<string, boolean>>({})
  // Matches the backend's default ORDER BY c.name ASC, domain_names[1] ASC
  // (db_npm.go) until the user picks a column.
  const sortKey = ref<NPMSortKey>('connection_name')
  const sortDir = ref<'asc' | 'desc'>('asc')

  function toggleSort(key: NPMSortKey): void {
    if (sortKey.value === key) {
      sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
    } else {
      sortKey.value = key
      sortDir.value = 'asc'
    }
  }

  const sortedHosts = computed(() => {
    const dir = sortDir.value === 'asc' ? 1 : -1
    return [...hosts.value].sort((a, b) => {
      switch (sortKey.value) {
        case 'domain':
          return dir * (a.domain_names[0] || '').localeCompare(b.domain_names[0] || '', 'fr')
        case 'forward':
          return dir * `${a.forward_host}:${a.forward_port}`.localeCompare(`${b.forward_host}:${b.forward_port}`, 'fr', { numeric: true })
        case 'npm_enabled':
          return dir * (Number(a.npm_enabled) - Number(b.npm_enabled))
        case 'uptime_status': {
          // Down first, then up, then unmonitored — matches what an operator
          // scanning for trouble actually wants to see first.
          const rank = (h: NPMProxyHostEnriched) => (h.uptime_status === 'down' ? 0 : h.uptime_status === 'up' ? 1 : 2)
          return dir * (rank(a) - rank(b))
        }
        case 'ssl_days_remaining':
          return dir * ((a.ssl_days_remaining ?? Infinity) - (b.ssl_days_remaining ?? Infinity))
        default:
          return dir * a.connection_name.localeCompare(b.connection_name, 'fr')
      }
    })
  })

  // A proxy host actively routing real traffic (npm_enabled) with no uptime
  // probe watching it is a monitoring blind spot — nothing would notice if
  // it went down. Surfaced as a row highlight, not just a toggle state.
  function needsAttention(host: NPMProxyHostEnriched): boolean {
    return host.npm_enabled && !host.uptime_monitoring_enabled
  }

  async function load(): Promise<void> {
    loading.value = true
    loadError.value = ''
    try {
      const res = await npmApi.listAllProxyHosts()
      hosts.value = res.data.proxy_hosts ?? []
    } catch (e: unknown) {
      loadError.value = getApiErrorMessage(e, 'Impossible de charger les proxy hosts.')
    } finally {
      loading.value = false
    }
  }

  // toggleNPM appelle NPM pour activer/désactiver le proxy host dans NPM lui-même.
  async function toggleNPM(host: NPMProxyHostEnriched, value: boolean): Promise<void> {
    const prev = host.npm_enabled
    host.npm_enabled = value
    if (!value) {
      // Optimisme : si on désactive NPM, monitoring s'éteint aussi
      host.monitoring_enabled = false
      host.uptime_monitoring_enabled = false
      host.ssl_monitoring_enabled = false
    }

    if (!value) {
      // Désactiver un proxy host coupe immédiatement son routage réel dans NPM —
      // seule direction de ce toggle qui mérite une confirmation.
      const confirmed = await dialog.confirm({
        title: 'Désactiver le proxy host',
        message: `Désactiver "${host.domain_names?.[0] || host.id}" dans NPM coupe immédiatement le routage réel vers ce service.`,
        variant: 'warning',
      })
      if (!confirmed) {
        // Reassigner explicitement (même hors erreur API) pour resynchroniser
        // la case à cocher native, dont l'état DOM a déjà changé au clic.
        host.npm_enabled = prev
        host.monitoring_enabled = prev
        host.uptime_monitoring_enabled = prev
        host.ssl_monitoring_enabled = prev
        return
      }
    }

    togglingNPM.value[host.id] = true
    actionError.value = ''
    try {
      const res = await npmApi.setNPMEnabled(host.id, value)
      const idx = hosts.value.findIndex(h => h.id === host.id)
      if (idx !== -1) hosts.value[idx] = res.data
    } catch (e: unknown) {
      // Rollback
      host.npm_enabled = prev
      if (!value) {
        host.monitoring_enabled = prev
        host.uptime_monitoring_enabled = prev
        host.ssl_monitoring_enabled = prev
      }
      actionError.value = getApiErrorMessage(e, `Impossible de ${value ? 'activer' : 'désactiver'} le proxy host dans NPM.`)
      setTimeout(() => { actionError.value = '' }, 5000)
    } finally {
      togglingNPM.value[host.id] = false
    }
  }

  // toggle gère les flags de monitoring ServerSupervisor (uptime/SSL).
  async function toggle(
    host: NPMProxyHostEnriched,
    field: 'uptime_monitoring_enabled' | 'ssl_monitoring_enabled',
    value: boolean,
  ): Promise<void> {
    const prev = host[field]
    host[field] = value

    toggling.value[host.id] = true
    actionError.value = ''
    try {
      const res = await npmApi.updateProxyHost(host.id, { [field]: value })
      const idx = hosts.value.findIndex(h => h.id === host.id)
      if (idx !== -1) hosts.value[idx] = res.data
    } catch (e: unknown) {
      host[field] = prev
      actionError.value = getApiErrorMessage(e, 'Erreur lors de la mise à jour du monitoring.')
      setTimeout(() => { actionError.value = '' }, 5000)
    } finally {
      toggling.value[host.id] = false
    }
  }

  onMounted(load)

  return {
    hosts,
    sortedHosts,
    sortKey,
    sortDir,
    loading,
    loadError,
    actionError,
    toggling,
    togglingNPM,
    load,
    toggleNPM,
    toggle,
    toggleSort,
    needsAttention,
  }
}
