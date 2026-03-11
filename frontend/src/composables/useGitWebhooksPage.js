import { computed, onMounted, ref } from 'vue'
import api from '../api'
import { useConfirmDialog } from './useConfirmDialog'

export function useGitWebhooksPage() {
  const dialog = useConfirmDialog()

  const activeTab = ref('webhooks')
  const hosts = ref([])
  const error = ref('')
  const saving = ref(false)
  const modalError = ref('')
  const webhooks = ref([])
  const loadingWebhooks = ref(false)
  const showWebhookModal = ref(false)
  const editingWebhook = ref(null)
  const newWebhookSecret = ref('')
  const newWebhookId = ref('')
  const trackers = ref([])
  const loadingTrackers = ref(false)
  const showTrackerModal = ref(false)
  const editingTracker = ref(null)

  const recentWebhookExecutions = computed(() =>
    webhooks.value
      .filter((webhook) => webhook.last_execution)
      .map((webhook) => ({
        ...webhook.last_execution,
        sourceId: webhook.id,
        sourceName: webhook.name,
        repo_name: webhook.last_execution.repo_name || webhook.repo_filter || webhook.name,
        branch: webhook.last_execution.branch || webhook.branch_filter || '',
      }))
      .sort((left, right) => new Date(right.triggered_at) - new Date(left.triggered_at))
  )

  const recentTrackerExecutions = computed(() =>
    trackers.value
      .filter((tracker) => tracker.last_execution)
      .map((tracker) => ({
        ...tracker.last_execution,
        sourceId: tracker.id,
        tag_name: tracker.last_execution.tag_name || tracker.last_release_tag,
        release_name: tracker.last_execution.release_name || tracker.name,
      }))
      .sort((left, right) => new Date(right.triggered_at) - new Date(left.triggered_at))
  )

  onMounted(loadAll)

  async function loadAll() {
    await Promise.all([loadWebhooks(), loadTrackers(), loadHosts()])
  }

  async function loadWebhooks() {
    loadingWebhooks.value = true
    try {
      error.value = ''
      const response = await api.getGitWebhooks()
      webhooks.value = response.data.webhooks || []
    } catch (err) {
      error.value = err.response?.data?.error || 'Erreur lors du chargement des webhooks'
    } finally {
      loadingWebhooks.value = false
    }
  }

  async function loadTrackers() {
    loadingTrackers.value = true
    try {
      error.value = ''
      const response = await api.getReleaseTrackers()
      trackers.value = response.data.trackers || []
    } catch (err) {
      error.value = err.response?.data?.error || 'Erreur lors du chargement des trackers'
    } finally {
      loadingTrackers.value = false
    }
  }

  async function loadHosts() {
    try {
      const response = await api.getHosts()
      hosts.value = response.data || []
    } catch {
      hosts.value = []
    }
  }

  function openCreateWebhook() {
    editingWebhook.value = null
    modalError.value = ''
    showWebhookModal.value = true
  }

  function openEditWebhook(webhook) {
    editingWebhook.value = webhook
    modalError.value = ''
    showWebhookModal.value = true
  }

  function closeWebhookModal() {
    showWebhookModal.value = false
    editingWebhook.value = null
    modalError.value = ''
  }

  async function saveWebhook(payload) {
    saving.value = true
    modalError.value = ''
    try {
      if (editingWebhook.value) {
        await api.updateGitWebhook(editingWebhook.value.id, payload)
      } else {
        const response = await api.createGitWebhook(payload)
        const created = response.data.webhook
        if (created?.secret) {
          newWebhookId.value = created.id
          newWebhookSecret.value = created.secret
        }
      }
      closeWebhookModal()
      await loadWebhooks()
    } catch (err) {
      modalError.value = err.response?.data?.error || 'Erreur'
    } finally {
      saving.value = false
    }
  }

  async function toggleWebhook(webhook) {
    try {
      await api.updateGitWebhook(webhook.id, { ...webhook, enabled: !webhook.enabled })
      await loadWebhooks()
    } catch (err) {
      error.value = err.response?.data?.error || 'Erreur'
    }
  }

  async function confirmDeleteWebhook(webhook) {
    const ok = await dialog.confirm({
      title: `Supprimer le webhook "${webhook.name}" ?`,
      message: 'Toutes les executions associees seront egalement supprimees.',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await api.deleteGitWebhook(webhook.id)
      await loadWebhooks()
    } catch (err) {
      error.value = err.response?.data?.error || 'Erreur lors de la suppression'
    }
  }

  function closeSecretModal() {
    newWebhookSecret.value = ''
    newWebhookId.value = ''
  }

  function openCreateTracker() {
    editingTracker.value = null
    modalError.value = ''
    showTrackerModal.value = true
  }

  function openEditTracker(tracker) {
    editingTracker.value = tracker
    modalError.value = ''
    showTrackerModal.value = true
  }

  function closeTrackerModal() {
    showTrackerModal.value = false
    editingTracker.value = null
    modalError.value = ''
  }

  async function saveTracker(payload) {
    saving.value = true
    modalError.value = ''
    try {
      if (editingTracker.value) {
        await api.updateReleaseTracker(editingTracker.value.id, payload)
      } else {
        await api.createReleaseTracker(payload)
      }
      closeTrackerModal()
      await loadTrackers()
    } catch (err) {
      modalError.value = err.response?.data?.error || 'Erreur'
    } finally {
      saving.value = false
    }
  }

  async function toggleTracker(tracker) {
    try {
      await api.updateReleaseTracker(tracker.id, { ...tracker, enabled: !tracker.enabled })
      await loadTrackers()
    } catch (err) {
      error.value = err.response?.data?.error || 'Erreur'
    }
  }

  async function checkNow(tracker) {
    try {
      await api.checkReleaseTrackerNow(tracker.id)
      setTimeout(() => loadTrackers(), 2000)
    } catch (err) {
      error.value = err.response?.data?.error || 'Erreur'
    }
  }

  async function confirmDeleteTracker(tracker) {
    const ok = await dialog.confirm({
      title: `Supprimer le tracker "${tracker.name}" ?`,
      message: 'Toutes les executions associees seront egalement supprimees.',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await api.deleteReleaseTracker(tracker.id)
      await loadTrackers()
    } catch (err) {
      error.value = err.response?.data?.error || 'Erreur lors de la suppression'
    }
  }

  function repoURL(tracker) {
    switch (tracker.provider) {
      case 'gitlab':
        return `https://gitlab.com/${tracker.repo_owner}/${tracker.repo_name}`
      case 'gitea':
        return `https://codeberg.org/${tracker.repo_owner}/${tracker.repo_name}`
      default:
        return `https://github.com/${tracker.repo_owner}/${tracker.repo_name}`
    }
  }

  function providerBadge(provider) {
    const map = {
      github: 'bg-blue-lt text-blue',
      gitlab: 'bg-orange-lt text-orange',
      gitea: 'bg-teal-lt text-teal',
      forgejo: 'bg-purple-lt text-purple',
      custom: 'bg-secondary-lt text-secondary',
    }
    return map[provider] || 'bg-secondary-lt text-secondary'
  }

  function execStatusBadge(status) {
    const map = {
      pending: 'bg-yellow-lt text-yellow',
      running: 'bg-blue-lt text-blue',
      completed: 'bg-success-lt text-success',
      failed: 'bg-danger-lt text-danger',
      skipped: 'bg-secondary-lt text-secondary',
    }
    return map[status] || 'bg-secondary-lt text-secondary'
  }

  function formatRelative(dateStr) {
    if (!dateStr) return '-'
    return new Date(dateStr).toLocaleString('fr-FR')
  }

  return {
    activeTab,
    hosts,
    error,
    saving,
    modalError,
    webhooks,
    loadingWebhooks,
    showWebhookModal,
    editingWebhook,
    newWebhookSecret,
    newWebhookId,
    trackers,
    loadingTrackers,
    showTrackerModal,
    editingTracker,
    recentWebhookExecutions,
    recentTrackerExecutions,
    openCreateWebhook,
    openEditWebhook,
    closeWebhookModal,
    saveWebhook,
    toggleWebhook,
    confirmDeleteWebhook,
    closeSecretModal,
    openCreateTracker,
    openEditTracker,
    closeTrackerModal,
    saveTracker,
    toggleTracker,
    checkNow,
    confirmDeleteTracker,
    repoURL,
    providerBadge,
    execStatusBadge,
    formatRelative,
  }
}
