import { Ref, ref } from 'vue'
import { useConfirmDialog } from './useConfirmDialog'
import { useDateFormatter } from './useDateFormatter'
import { useHostsStore } from '../stores/hosts'
import { useAlertRulesStore } from '../stores/alertRules'
import apiClient, { getApiErrorMessage } from '../api'
import { storeToRefs } from 'pinia'
import type { Host } from '../types/host'
import type { ReleaseTracker } from '../types/tracker'
import type { AlertRule } from '../types/alert'
import type { WSNotificationMessage } from '../types/ws'

interface AlertRuleCapabilities {
  metrics: unknown[]
  agent_metrics?: unknown[]
  proxmox_metrics?: unknown[]
  synthetic_metrics?: unknown[]
  docker_metrics?: unknown[]
  proxmox_scope: {
    modes: string[]
    connections: unknown[]
    nodes: unknown[]
    storages: unknown[]
    guests: unknown[]
    disks: unknown[]
  }
}

type AlertRulePayload = Record<string, unknown>

interface UseAlertsPageApi {
  alertsTab: Ref<string>
  trackers: Ref<ReleaseTracker[]>
  trackersLoading: Ref<boolean>
  trackersError: Ref<string>
  rules: Ref<AlertRule[]>
  hosts: Ref<Host[]>
  loading: Ref<boolean>
  fetched: Ref<boolean>
  fetchError: Ref<string>
  showModal: Ref<boolean>
  saving: Ref<boolean>
  saveError: Ref<string>
  editingRule: Ref<AlertRule | null>
  capabilities: Ref<AlertRuleCapabilities | null>
  capabilitiesLoading: Ref<boolean>
  capabilitiesError: Ref<string>
  init: () => Promise<void>
  loadTrackers: () => Promise<void>
  switchToTrackers: () => Promise<void>
  startAddAlert: () => void
  startEditAlert: (rule: AlertRule) => void
  saveAlert: (payload: AlertRulePayload) => Promise<void>
  toggleEnabled: (rule: AlertRule) => Promise<void>
  deleteAlert: (rule: AlertRule) => Promise<void>
  closeModal: () => void
  formatDate: (dateStr: string) => string
  onWebSocketAlert: (payload: WSNotificationMessage) => void
}

export function useAlertsPage(): UseAlertsPageApi {
  const { confirm } = useConfirmDialog()
  const { formatLocaleDateTime } = useDateFormatter()
  const hostsStore = useHostsStore()
  const rulesStore = useAlertRulesStore()

  const alertsTab: Ref<string> = ref('rules')
  const trackers: Ref<ReleaseTracker[]> = ref([])
  const trackersLoading: Ref<boolean> = ref(false)
  const trackersError: Ref<string> = ref('')
  const trackersLoaded: Ref<boolean> = ref(false)
  const showModal: Ref<boolean> = ref(false)
  const saving: Ref<boolean> = ref(false)
  const saveError: Ref<string> = ref('')
  const editingRule: Ref<AlertRule | null> = ref(null)
  const capabilities: Ref<AlertRuleCapabilities | null> = ref(null)
  const capabilitiesLoading: Ref<boolean> = ref(false)
  const capabilitiesError: Ref<string> = ref('')

  // Expose store state (reactive refs shared across navigations)
  const { rules, loading, fetched, error: fetchError } = storeToRefs(rulesStore)
  const { hosts } = storeToRefs(hostsStore)

  async function init(): Promise<void> {
    capabilitiesLoading.value = true
    capabilitiesError.value = ''

    const [rulesResult, hostsResult, agentCapsResult, proxmoxCapsResult, syntheticCapsResult, dockerCapsResult] = await Promise.allSettled([
      rulesStore.fetchRules(),
      hostsStore.fetchHosts(),
      apiClient.getAgentAlertRuleCapabilities(),
      apiClient.getProxmoxAlertRuleCapabilities(),
      apiClient.getSyntheticAlertRuleCapabilities(),
      apiClient.getDockerAlertCapabilities(),
    ])

    if (agentCapsResult.status === 'fulfilled' && proxmoxCapsResult.status === 'fulfilled') {
      const agentMetrics = agentCapsResult.value?.data?.metrics || []
      const proxmoxMetrics = proxmoxCapsResult.value?.data?.proxmox_metrics || []
      const syntheticMetrics =
        syntheticCapsResult.status === 'fulfilled'
          ? syntheticCapsResult.value?.data?.metrics || []
          : []
      const dockerMetrics =
        dockerCapsResult.status === 'fulfilled'
          ? dockerCapsResult.value?.data?.metrics || []
          : []
      const proxmoxScope = proxmoxCapsResult.value?.data?.proxmox_scope || {
        modes: [],
        connections: [],
        nodes: [],
        storages: [],
        guests: [],
        disks: [],
      }
      capabilities.value = {
        metrics: [...agentMetrics, ...proxmoxMetrics, ...syntheticMetrics, ...dockerMetrics],
        agent_metrics: agentMetrics,
        proxmox_metrics: proxmoxMetrics,
        synthetic_metrics: syntheticMetrics,
        docker_metrics: dockerMetrics,
        proxmox_scope: proxmoxScope,
      }
    } else {
      capabilitiesError.value = 'Impossible de charger les capacites des metriques'
    }

    capabilitiesLoading.value = false

    if (!rulesStore.fetched && !rulesStore.loading) {
      await rulesStore.fetchRules(true)
    }

    if (rulesResult.status === 'rejected' || hostsResult.status === 'rejected') {
      // Keep existing store-managed error handling behavior.
    }
  }

  async function loadTrackers(): Promise<void> {
    trackersLoading.value = true
    trackersError.value = ''
    try {
      const response = await apiClient.getReleaseTrackers()
      trackers.value = response.data?.trackers || []
      trackersLoaded.value = true
    } catch {
      trackersError.value = 'Impossible de charger les trackers de versions'
    } finally {
      trackersLoading.value = false
    }
  }

  async function switchToTrackers(): Promise<void> {
    alertsTab.value = 'releases'
    if (!trackersLoaded.value) await loadTrackers()
  }

  function startAddAlert(): void {
    editingRule.value = null
    saveError.value = ''
    showModal.value = true
  }

  function startEditAlert(rule: AlertRule): void {
    editingRule.value = rule
    saveError.value = ''
    showModal.value = true
  }

  async function saveAlert(payload: AlertRulePayload): Promise<void> {
    saveError.value = ''
    saving.value = true
    try {
      if (editingRule.value) {
        await apiClient.updateAlertRule(editingRule.value.id, payload)
      } else {
        await apiClient.createAlertRule(payload)
      }
      await rulesStore.fetchRules(true)
      closeModal()
    } catch (err: unknown) {
      saveError.value = `Erreur : ${getApiErrorMessage(err)}`
    } finally {
      saving.value = false
    }
  }

  // Disabling a rule can close its active incidents server-side — the
  // incidents-tab refresh this used to trigger directly now happens in
  // AlertsView.vue's onToggleEnabled wrapper, which owns both this composable
  // and useNotificationHistory().
  async function toggleEnabled(rule: AlertRule): Promise<void> {
    try {
      await apiClient.updateAlertRule(rule.id, { enabled: !rule.enabled })
      await rulesStore.fetchRules(true)
    } catch {
      // ignore
    }
  }

  async function deleteAlert(rule: AlertRule): Promise<void> {
    const confirmed = await confirm({
      title: "Supprimer l'alerte ?",
      message: `Voulez-vous vraiment supprimer la regle "${rule.name || 'Sans nom'}" ?\n\nCette action est irreversible.`,
      variant: 'danger',
    })
    if (!confirmed) return

    try {
      await apiClient.deleteAlertRule(rule.id)
      await rulesStore.fetchRules(true)
    } catch (err: unknown) {
      saveError.value = `Erreur lors de la suppression : ${getApiErrorMessage(err)}`
    }
  }

  function closeModal(): void {
    showModal.value = false
    editingRule.value = null
    saveError.value = ''
  }

  function formatDate(dateStr: string): string {
    return formatLocaleDateTime(dateStr)
  }

  // Trackers-only concern now — the incidents-relevant branches of this
  // event (alert_incident_update / new_alert / a second refresh on
  // release_tracker_*) moved to useNotificationHistory.ts, which subscribes
  // to the same shared WS feed independently (see AlertsView.vue).
  function onWebSocketAlert(payload: WSNotificationMessage): void {
    if (
      (payload.type === 'release_tracker_detected' || payload.type === 'release_tracker_execution') &&
      trackersLoaded.value
    ) {
      loadTrackers()
    }
  }

  return {
    alertsTab,
    trackers,
    trackersLoading,
    trackersError,
    rules,
    hosts,
    loading: loading as Ref<boolean>,
    fetched: fetched as Ref<boolean>,
    fetchError: fetchError as Ref<string>,
    showModal,
    saving,
    saveError,
    editingRule,
    capabilities,
    capabilitiesLoading,
    capabilitiesError,
    init,
    loadTrackers,
    switchToTrackers,
    startAddAlert,
    startEditAlert,
    saveAlert,
    toggleEnabled,
    deleteAlert,
    closeModal,
    formatDate,
    onWebSocketAlert,
  }
}
