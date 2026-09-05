import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { npmApi } from '../api/npm'
import type { NPMProxyHostEnriched } from '../types/npm'
import { getApiErrorMessage } from '../api/client'
import { useConfirmDialog } from './useConfirmDialog'

export type NPMSortKey = 'connection_name' | 'domain' | 'forward' | 'npm_enabled' | 'uptime_status' | 'ssl_days_remaining'

const NPM_REFRESH_SEC = 30

export function useNPM() {
  const { t } = useI18n()
  const dialog = useConfirmDialog()
  const hosts = ref<NPMProxyHostEnriched[]>([])
  const loading = ref(true)
  const loadError = ref('')
  const actionError = ref('')
  const toggling = ref<Record<string, boolean>>({})
  const togglingNPM = ref<Record<string, boolean>>({})
  const autoRefresh = ref(true)
  const lastUpdatedAt = ref<Date | null>(null)
  let refreshTimer: ReturnType<typeof setInterval> | null = null
  const hasPendingToggle = computed(() =>
    Object.values(toggling.value).some(Boolean) || Object.values(togglingNPM.value).some(Boolean)
  )
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
      lastUpdatedAt.value = new Date()
    } catch (e: unknown) {
      loadError.value = getApiErrorMessage(e, t('npm.couldNotLoadProxyHostsError'))
    } finally {
      loading.value = false
    }
  }

  function startRefreshTimer(): void {
    stopRefreshTimer()
    refreshTimer = setInterval(() => {
      if (autoRefresh.value && !hasPendingToggle.value) load()
    }, NPM_REFRESH_SEC * 1000)
  }

  function stopRefreshTimer(): void {
    if (refreshTimer) { clearInterval(refreshTimer); refreshTimer = null }
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
        title: t('npm.disableProxyHostConfirmTitle'),
        message: t('npm.disableProxyHostConfirmMessage', { name: host.domain_names?.[0] || host.id }),
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
      actionError.value = getApiErrorMessage(e, value ? t('npm.enableProxyHostFailedError') : t('npm.disableProxyHostFailedError'))
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
      actionError.value = getApiErrorMessage(e, t('npm.updateMonitoringError'))
      setTimeout(() => { actionError.value = '' }, 5000)
    } finally {
      toggling.value[host.id] = false
    }
  }

  onMounted(() => {
    load()
    startRefreshTimer()
  })
  onUnmounted(stopRefreshTimer)

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
    autoRefresh,
    lastUpdatedAt,
    NPM_REFRESH_SEC,
    load,
    toggleNPM,
    toggle,
    toggleSort,
    needsAttention,
  }
}
