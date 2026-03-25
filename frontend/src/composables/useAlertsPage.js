import { computed, ref } from 'vue'
import { useConfirmDialog } from './useConfirmDialog'
import { useDateFormatter } from './useDateFormatter'
import { useHostsStore } from '../stores/hosts'
import { useAlertRulesStore } from '../stores/alertRules'
import apiClient from '../api'

export function useAlertsPage() {
  const { confirm } = useConfirmDialog()
  const { formatLocaleDateTime } = useDateFormatter()
  const hostsStore = useHostsStore()
  const rulesStore = useAlertRulesStore()

  const alertsTab = ref('rules')
  const incidents = ref([])
  const incidentsLoading = ref(false)
  const incidentsError = ref('')
  const incidentsLoaded = ref(false)
  const showModal = ref(false)
  const saving = ref(false)
  const saveError = ref('')
  const editingRule = ref(null)

  // Refs réactives du store (pas de déstructuration = garde la réactivité)
  const rules = rulesStore.rules
  const hosts = hostsStore.hosts
  const loading = rulesStore.loading
  const fetched = rulesStore.fetched
  const fetchError = rulesStore.error

  const activeIncidentCount = computed(() => incidents.value.filter((incident) => !incident.resolved_at).length)

  async function init() {
    // Invalider le TTL à chaque montage pour forcer un fetch frais (F5 / cold start)
    rulesStore.invalidate()
    hostsStore.invalidate()

    // Charger règles, hosts et incidents en parallèle
    await Promise.all([
      rulesStore.fetchRules(),
      hostsStore.fetchHosts(),
      loadIncidents(),
    ])
  }

  async function loadIncidents() {
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

  async function switchToIncidents() {
    alertsTab.value = 'incidents'
    await loadIncidents()
  }

  function startAddAlert() {
    editingRule.value = null
    saveError.value = ''
    showModal.value = true
  }

  function startEditAlert(rule) {
    editingRule.value = rule
    saveError.value = ''
    showModal.value = true
  }

  async function saveAlert(payload) {
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
    } catch (err) {
      saveError.value = 'Erreur : ' + (err.response?.data?.error || err.message)
    } finally {
      saving.value = false
    }
  }

  async function toggleEnabled(rule) {
    try {
      await apiClient.updateAlertRule(rule.id, { enabled: !rule.enabled })
      await rulesStore.fetchRules(true)
    } catch {
      // ignore
    }
  }

  async function deleteAlert(rule) {
    const confirmed = await confirm({
      title: "Supprimer l'alerte ?",
      message: `Voulez-vous vraiment supprimer la regle "${rule.name || 'Sans nom'}" ?\n\nCette action est irreversible.`,
      variant: 'danger',
    })
    if (!confirmed) return

    try {
      await apiClient.deleteAlertRule(rule.id)
      await rulesStore.fetchRules(true)
    } catch (err) {
      saveError.value = 'Erreur lors de la suppression : ' + (err.response?.data?.error || err.message)
    }
  }

  function closeModal() {
    showModal.value = false
    editingRule.value = null
    saveError.value = ''
  }

  function formatDate(dateStr) {
    return formatLocaleDateTime(dateStr)
  }

  function onWebSocketAlert(payload) {
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
    rules,
    hosts,
    loading,
    fetched,
    fetchError,
    showModal,
    saving,
    saveError,
    editingRule,
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
