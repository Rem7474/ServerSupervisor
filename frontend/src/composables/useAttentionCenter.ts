import { ref, computed, onMounted } from 'vue'
import apiClient from '../api'
import { isNeverConnectedHost } from '../utils/hosts'

/**
 * Aggregates signals the backend already detects but that are otherwise only
 * visible by landing on the exact right page (a suggested Proxmox link only
 * shows on that host's detail page, an NPM host with monitoring off only
 * shows in the NPM list, ...). Fetches from the existing list endpoints
 * client-side rather than adding a new backend aggregation endpoint.
 *
 * Deliberately excludes CVEs and Proxmox node/storage health — those already
 * have their own banners on the Dashboard, duplicating them here would just
 * be noise.
 */
export interface AttentionItem {
  key: string
  label: string
  count: number
  to: string
  severity: 'info' | 'warning'
}

interface ReleaseTrackerLike {
  tracker_type: string
  enabled: boolean
  custom_task_id?: string
  update_action?: string
  compose_project?: string
}

// A tracker only dispatches an update if it has a task to run: for git
// trackers that's custom_task_id, for docker trackers it's custom_task_id
// (default "custom" update_action) or compose_project (update_action="compose").
function isTrackerMonitorOnly(t: ReleaseTrackerLike): boolean {
  if (t.tracker_type === 'git') return !t.custom_task_id
  if (t.update_action === 'compose') return !t.compose_project
  return !t.custom_task_id
}

const SSL_WARNING_DAYS = 14

export function useAttentionCenter() {
  const loading = ref(true)
  const suggestedProxmoxLinks = ref(0)
  const neverConnectedHosts = ref(0)
  const npmMonitoringOff = ref(0)
  const monitorOnlyTrackers = ref(0)
  const expiringSslCerts = ref(0)

  async function refresh(): Promise<void> {
    loading.value = true
    const [links, hosts, npmHosts, trackers, certs] = await Promise.allSettled([
      apiClient.getProxmoxLinks('suggested'),
      apiClient.getHosts(),
      apiClient.listAllProxyHosts(),
      apiClient.getReleaseTrackers(),
      apiClient.getSSLCertificates(),
    ])

    suggestedProxmoxLinks.value =
      links.status === 'fulfilled' && Array.isArray(links.value.data) ? links.value.data.length : 0

    neverConnectedHosts.value =
      hosts.status === 'fulfilled' && Array.isArray(hosts.value.data)
        ? hosts.value.data.filter(isNeverConnectedHost).length
        : 0

    npmMonitoringOff.value =
      npmHosts.status === 'fulfilled'
        ? (npmHosts.value.data.proxy_hosts || []).filter((h) => h.npm_enabled && !h.monitoring_enabled).length
        : 0

    monitorOnlyTrackers.value =
      trackers.status === 'fulfilled'
        ? (trackers.value.data.trackers || []).filter((t) => t.enabled && isTrackerMonitorOnly(t)).length
        : 0

    expiringSslCerts.value =
      certs.status === 'fulfilled'
        ? (certs.value.data.certificates || []).filter(
            (c) => c.enabled && c.days_remaining != null && c.days_remaining <= SSL_WARNING_DAYS
          ).length
        : 0

    loading.value = false
  }

  const items = computed<AttentionItem[]>(() => {
    const list: AttentionItem[] = []
    if (expiringSslCerts.value > 0) {
      list.push({
        key: 'ssl',
        label: `${expiringSslCerts.value} certificat${expiringSslCerts.value > 1 ? 's' : ''} SSL bientôt expiré${expiringSslCerts.value > 1 ? 's' : ''}`,
        count: expiringSslCerts.value,
        to: '/monitoring?tab=ssl',
        severity: 'warning',
      })
    }
    if (neverConnectedHosts.value > 0) {
      list.push({
        key: 'never-connected',
        label: `${neverConnectedHosts.value} hôte${neverConnectedHosts.value > 1 ? 's' : ''} enregistré${neverConnectedHosts.value > 1 ? 's' : ''} sans agent connecté`,
        count: neverConnectedHosts.value,
        to: '/',
        severity: 'warning',
      })
    }
    if (suggestedProxmoxLinks.value > 0) {
      list.push({
        key: 'proxmox-links',
        label: `${suggestedProxmoxLinks.value} liaison${suggestedProxmoxLinks.value > 1 ? 's' : ''} Proxmox suggérée${suggestedProxmoxLinks.value > 1 ? 's' : ''} à confirmer`,
        count: suggestedProxmoxLinks.value,
        to: '/proxmox',
        severity: 'info',
      })
    }
    if (npmMonitoringOff.value > 0) {
      list.push({
        key: 'npm-monitoring',
        label: `${npmMonitoringOff.value} proxy host${npmMonitoringOff.value > 1 ? 's' : ''} NPM sans monitoring activé`,
        count: npmMonitoringOff.value,
        to: '/npm',
        severity: 'info',
      })
    }
    if (monitorOnlyTrackers.value > 0) {
      list.push({
        key: 'trackers',
        label: `${monitorOnlyTrackers.value} suivi${monitorOnlyTrackers.value > 1 ? 's' : ''} de version sans tâche de déploiement`,
        count: monitorOnlyTrackers.value,
        to: '/git-webhooks?tab=trackers',
        severity: 'info',
      })
    }
    return list
  })

  onMounted(refresh)

  return { loading, items, refresh }
}
