import { ref, onMounted } from 'vue'
import { npmApi } from '../api/npm'
import type { NPMProxyHostEnriched } from '../types/npm'
import { getApiErrorMessage } from '../api/client'
import { useConfirmDialog } from './useConfirmDialog'

export function useNPM() {
  const dialog = useConfirmDialog()
  const hosts = ref<NPMProxyHostEnriched[]>([])
  const loading = ref(true)
  const loadError = ref('')
  const actionError = ref('')
  const toggling = ref<Record<string, boolean>>({})
  const togglingNPM = ref<Record<string, boolean>>({})

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
    loading,
    loadError,
    actionError,
    toggling,
    togglingNPM,
    load,
    toggleNPM,
    toggle,
  }
}
