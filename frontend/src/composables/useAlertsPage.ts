import { computed, ComputedRef, Ref, ref } from 'vue'
import { useConfirmDialog } from './useConfirmDialog'
import { useDateFormatter } from './useDateFormatter'
import { useHostsStore } from '../stores/hosts'
import { useAlertRulesStore } from '../stores/alertRules'
import apiClient, { getApiErrorMessage } from '../api'
import { storeToRefs } from 'pinia'

interface Host {
  id: string
  [key: string]: unknown
}

interface AlertRule {
  id: number
  name: string
  enabled: boolean
  [key: string]: unknown
}

interface Incident {
  id: string
  resolved_at?: string | null
  [key: string]: unknown
}

interface Notification {
  type: string
  notification?: Incident
}

interface AlertRuleCapabilities {
  metrics: unknown[]
  agent_metrics?: unknown[]
  proxmox_metrics?: unknown[]
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
  incidents: Ref<Incident[]>
  incidentsLoading: Ref<boolean>
  incidentsError: Ref<string>
  incidentsLoaded: Ref<boolean>
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
  activeIncidentCount: ComputedRef<number>
  init: () => Promise<void>
  loadIncidents: () => Promise<void>
  switchToIncidents: () => Promise<void>
  startAddAlert: () => void
  startEditAlert: (rule: AlertRule) => void
  saveAlert: (payload: AlertRulePayload) => Promise<void>
  toggleEnabled: (rule: AlertRule) => Promise<void>
  deleteAlert: (rule: AlertRule) => Promise<void>
  closeModal: () => void
  formatDate: (dateStr: string) => string
  onWebSocketAlert: (payload: Notification) => void
}

export function useAlertsPage(): UseAlertsPageApi {
  const { confirm } = useConfirmDialog()
  const { formatLocaleDateTime } = useDateFormatter()
  const hostsStore = useHostsStore()
  const rulesStore = useAlertRulesStore()

  const alertsTab: Ref<string> = ref('rules')
  const incidents: Ref<Incident[]> = ref([])
  const incidentsLoading: Ref<boolean> = ref(false)
  const incidentsError: Ref<string> = ref('')
  const incidentsLoaded: Ref<boolean> = ref(false)
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

  const activeIncidentCount: ComputedRef<number> = computed(
    () => incidents.value.filter((incident) => !incident.resolved_at).length
  )

  async function init(): Promise<void> {
    capabilitiesLoading.value = true
    capabilitiesError.value = ''

    const [rulesResult, hostsResult, agentCapsResult, proxmoxCapsResult] = await Promise.allSettled([
      rulesStore.fetchRules(),
      hostsStore.fetchHosts(),
      apiClient.getAgentAlertRuleCapabilities(),
      apiClient.getProxmoxAlertRuleCapabilities(),
    ])

    if (agentCapsResult.status === 'fulfilled' && proxmoxCapsResult.status === 'fulfilled') {
      const agentMetrics = agentCapsResult.value?.data?.metrics || []
      const proxmoxMetrics = proxmoxCapsResult.value?.data?.proxmox_metrics || []
      const proxmoxScope = proxmoxCapsResult.value?.data?.proxmox_scope || {
        modes: [],
        connections: [],
        nodes: [],
        storages: [],
        guests: [],
        disks: [],
      }
      capabilities.value = {
        metrics: [...agentMetrics, ...proxmoxMetrics],
        agent_metrics: agentMetrics,
        proxmox_metrics: proxmoxMetrics,
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

  async function loadIncidents(): Promise<void> {
    incidentsLoading.value = true
    incidentsError.value = ''
    try {
      const response = await apiClient.getNotifications()
      incidents.value = response.data?.notifications || []
      incidentsLoaded.value = true
    } catch {
      incidentsError.value = 'Impossible de charger les incidents'
    } finally {
      incidentsLoading.value = false
    }
  }

  async function switchToIncidents(): Promise<void> {
    alertsTab.value = 'incidents'
    if (!incidentsLoaded.value) await loadIncidents()
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

  async function toggleEnabled(rule: AlertRule): Promise<void> {
    try {
      const nextEnabled = !rule.enabled
      await apiClient.updateAlertRule(rule.id, { enabled: nextEnabled })
      await rulesStore.fetchRules(true)
      if (!nextEnabled && incidentsLoaded.value) {
        await loadIncidents()
      }
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

  function onWebSocketAlert(payload: Notification): void {
    // Incident created or resolved — refresh the list
    if (payload.type === 'alert_incident_update') {
      loadIncidents()
      return
    }

    if (payload.type !== 'new_alert' || !payload.notification) return

    const incoming = payload.notification
    const idx = incidents.value.findIndex((item) => item.id === incoming.id)

    if (idx >= 0) {
      incidents.value = [
        { ...incidents.value[idx], ...incoming },
        ...incidents.value.slice(0, idx),
        ...incidents.value.slice(idx + 1),
      ]
    } else {
      incidents.value = [incoming, ...incidents.value]
    }

    incidentsLoaded.value = true
    loadIncidents()
  }

  return {
    alertsTab,
    incidents,
    incidentsLoading,
    incidentsError,
    incidentsLoaded,
    rules: rules as Ref<AlertRule[]>,
    hosts: hosts as Ref<Host[]>,
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
    activeIncidentCount,
    init,
    loadIncidents,
    switchToIncidents,
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
