import { computed, onMounted, ref, Ref, ComputedRef } from 'vue'
import { useRoute } from 'vue-router'
import api, { getApiErrorMessage } from '../api'
import { useConfirmDialog } from './useConfirmDialog'

interface ExecutionPayload {
  triggered_at?: string
  repo_name?: string
  branch?: string
  tag_name?: string
  release_name?: string
  [key: string]: unknown
}

interface GitWebhook {
  id: string
  name: string
  enabled: boolean
  secret?: string
  repo_filter?: string
  branch_filter?: string
  last_execution?: ExecutionPayload
  [key: string]: unknown
}

interface ReleaseTracker {
  id: string
  name: string
  enabled: boolean
  provider: string
  repo_owner: string
  repo_name: string
  last_release_tag?: string
  last_execution?: ExecutionPayload
  [key: string]: unknown
}

interface Host {
  id: string
  [key: string]: unknown
}

interface RecentExecution extends ExecutionPayload {
  triggered_at: string
  sourceId: string
  sourceName: string
}

interface CreatedWebhook {
  id?: string
  secret?: string
}

type FormPayload = Record<string, unknown>

interface UseGitWebhooksPageApi {
  activeTab: Ref<string>
  hosts: Ref<Host[]>
  error: Ref<string>
  saving: Ref<boolean>
  modalError: Ref<string>
  webhooks: Ref<GitWebhook[]>
  loadingWebhooks: Ref<boolean>
  showWebhookModal: Ref<boolean>
  editingWebhook: Ref<GitWebhook | null>
  newWebhookSecret: Ref<string>
  newWebhookId: Ref<string>
  trackers: Ref<ReleaseTracker[]>
  loadingTrackers: Ref<boolean>
  showTrackerModal: Ref<boolean>
  editingTracker: Ref<ReleaseTracker | null>
  prefillDockerImage: Ref<string>
  prefillDockerTag: Ref<string>
  recentWebhookExecutions: ComputedRef<RecentExecution[]>
  recentTrackerExecutions: ComputedRef<RecentExecution[]>
  openCreateWebhook: () => void
  openEditWebhook: (webhook: GitWebhook) => void
  closeWebhookModal: () => void
  saveWebhook: (payload: FormPayload) => Promise<void>
  toggleWebhook: (webhook: GitWebhook) => Promise<void>
  confirmDeleteWebhook: (webhook: GitWebhook) => Promise<void>
  closeSecretModal: () => void
  openCreateTracker: () => void
  openEditTracker: (tracker: ReleaseTracker) => void
  closeTrackerModal: () => void
  saveTracker: (payload: FormPayload) => Promise<void>
  toggleTracker: (tracker: ReleaseTracker) => Promise<void>
  checkNow: (tracker: ReleaseTracker) => Promise<void>
  confirmDeleteTracker: (tracker: ReleaseTracker) => Promise<void>
  repoURL: (tracker: ReleaseTracker) => string
  providerBadge: (provider: string) => string
  execStatusBadge: (status: string) => string
  formatRelative: (dateStr: string) => string
}

export function useGitWebhooksPage(): UseGitWebhooksPageApi {
  const dialog = useConfirmDialog()
  const route = useRoute()

  const activeTab: Ref<string> = ref('webhooks')
  const hosts: Ref<Host[]> = ref([])
  const error: Ref<string> = ref('')
  const saving: Ref<boolean> = ref(false)
  const modalError: Ref<string> = ref('')
  const webhooks: Ref<GitWebhook[]> = ref([])
  const loadingWebhooks: Ref<boolean> = ref(false)
  const showWebhookModal: Ref<boolean> = ref(false)
  const editingWebhook: Ref<GitWebhook | null> = ref(null)
  const newWebhookSecret: Ref<string> = ref('')
  const newWebhookId: Ref<string> = ref('')
  const trackers: Ref<ReleaseTracker[]> = ref([])
  const loadingTrackers: Ref<boolean> = ref(false)
  const showTrackerModal: Ref<boolean> = ref(false)
  const editingTracker: Ref<ReleaseTracker | null> = ref(null)

  const prefillDockerImage: Ref<string> = ref('')
  const prefillDockerTag: Ref<string> = ref('')

  const recentWebhookExecutions: ComputedRef<RecentExecution[]> = computed(() =>
    webhooks.value
      .filter((webhook) => webhook.last_execution)
      .map((webhook) => {
        const execution = webhook.last_execution as ExecutionPayload
        return {
          ...execution,
          triggered_at: execution.triggered_at || new Date(0).toISOString(),
          sourceId: webhook.id,
          sourceName: webhook.name,
          repo_name: execution.repo_name || webhook.repo_filter || webhook.name,
          branch: execution.branch || webhook.branch_filter || '',
        }
      })
      .sort((left, right) => new Date(right.triggered_at).getTime() - new Date(left.triggered_at).getTime())
  )

  const recentTrackerExecutions: ComputedRef<RecentExecution[]> = computed(() =>
    trackers.value
      .filter((tracker) => tracker.last_execution)
      .map((tracker) => {
        const execution = tracker.last_execution as ExecutionPayload
        return {
          ...execution,
          triggered_at: execution.triggered_at || new Date(0).toISOString(),
          sourceId: tracker.id,
          sourceName: tracker.name,
          tag_name: execution.tag_name || tracker.last_release_tag,
          release_name: execution.release_name || tracker.name,
        }
      })
      .sort((left, right) => new Date(right.triggered_at).getTime() - new Date(left.triggered_at).getTime())
  )

  function parseCreatedWebhook(response: unknown): CreatedWebhook {
    if (typeof response !== 'object' || response === null) return {}
    const data = (response as { data?: { webhook?: CreatedWebhook } }).data
    return data?.webhook ?? {}
  }

  function readError(err: unknown, fallback: string): string {
    return getApiErrorMessage(err, fallback)
  }

  onMounted(async () => {
    if (route.query.tab === 'trackers') {
      activeTab.value = 'trackers'
    }
    if (route.query.docker_image) {
      prefillDockerImage.value = String(route.query.docker_image)
      prefillDockerTag.value = String(route.query.docker_tag || 'latest')
      activeTab.value = 'trackers'
      await Promise.all([loadWebhooks(), loadTrackers(), loadHosts()])
      openCreateTracker()
      return
    }
    await Promise.all([loadWebhooks(), loadTrackers(), loadHosts()])
  })

  async function loadWebhooks(): Promise<void> {
    loadingWebhooks.value = true
    try {
      error.value = ''
      const response = await api.getGitWebhooks()
      webhooks.value = response.data.webhooks || []
    } catch (err: unknown) {
      error.value = readError(err, 'Erreur lors du chargement des webhooks')
    } finally {
      loadingWebhooks.value = false
    }
  }

  async function loadTrackers(): Promise<void> {
    loadingTrackers.value = true
    try {
      error.value = ''
      const response = await api.getReleaseTrackers()
      trackers.value = response.data.trackers || []
    } catch (err: unknown) {
      error.value = readError(err, 'Erreur lors du chargement des trackers')
    } finally {
      loadingTrackers.value = false
    }
  }

  async function loadHosts(): Promise<void> {
    try {
      const response = await api.getHosts()
      hosts.value = response.data || []
    } catch {
      hosts.value = []
    }
  }

  function openCreateWebhook(): void {
    editingWebhook.value = null
    modalError.value = ''
    showWebhookModal.value = true
  }

  function openEditWebhook(webhook: GitWebhook): void {
    editingWebhook.value = webhook
    modalError.value = ''
    showWebhookModal.value = true
  }

  function closeWebhookModal(): void {
    showWebhookModal.value = false
    editingWebhook.value = null
    modalError.value = ''
  }

  async function saveWebhook(payload: FormPayload): Promise<void> {
    saving.value = true
    modalError.value = ''
    try {
      if (editingWebhook.value) {
        await api.updateGitWebhook(editingWebhook.value.id, payload)
      } else {
        const response = await api.createGitWebhook(payload)
        const created = parseCreatedWebhook(response)
        if (created?.secret) {
          newWebhookId.value = created.id || ''
          newWebhookSecret.value = created.secret
        }
      }
      closeWebhookModal()
      await loadWebhooks()
    } catch (err: unknown) {
      modalError.value = readError(err, 'Erreur')
    } finally {
      saving.value = false
    }
  }

  async function toggleWebhook(webhook: GitWebhook): Promise<void> {
    try {
      await api.updateGitWebhook(webhook.id, { ...webhook, enabled: !webhook.enabled })
      await loadWebhooks()
    } catch (err: unknown) {
      error.value = readError(err, 'Erreur')
    }
  }

  async function confirmDeleteWebhook(webhook: GitWebhook): Promise<void> {
    const ok = await dialog.confirm({
      title: `Supprimer le webhook "${webhook.name}" ?`,
      message: 'Toutes les executions associees seront egalement supprimees.',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await api.deleteGitWebhook(webhook.id)
      await loadWebhooks()
    } catch (err: unknown) {
      error.value = readError(err, 'Erreur lors de la suppression')
    }
  }

  function closeSecretModal(): void {
    newWebhookSecret.value = ''
    newWebhookId.value = ''
  }

  function openCreateTracker(): void {
    editingTracker.value = null
    modalError.value = ''
    showTrackerModal.value = true
  }

  function openEditTracker(tracker: ReleaseTracker): void {
    prefillDockerImage.value = ''
    prefillDockerTag.value = ''
    editingTracker.value = tracker
    modalError.value = ''
    showTrackerModal.value = true
  }

  function closeTrackerModal(): void {
    showTrackerModal.value = false
    editingTracker.value = null
    modalError.value = ''
    prefillDockerImage.value = ''
    prefillDockerTag.value = ''
  }

  async function saveTracker(payload: FormPayload): Promise<void> {
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
    } catch (err: unknown) {
      modalError.value = readError(err, 'Erreur')
    } finally {
      saving.value = false
    }
  }

  async function toggleTracker(tracker: ReleaseTracker): Promise<void> {
    try {
      await api.updateReleaseTracker(tracker.id, { ...tracker, enabled: !tracker.enabled })
      await loadTrackers()
    } catch (err: unknown) {
      error.value = readError(err, 'Erreur')
    }
  }

  async function checkNow(tracker: ReleaseTracker): Promise<void> {
    try {
      await api.checkReleaseTrackerNow(tracker.id)
      setTimeout(() => loadTrackers(), 2000)
    } catch (err: unknown) {
      error.value = readError(err, 'Erreur')
    }
  }

  async function confirmDeleteTracker(tracker: ReleaseTracker): Promise<void> {
    const ok = await dialog.confirm({
      title: `Supprimer le tracker "${tracker.name}" ?`,
      message: 'Toutes les executions associees seront egalement supprimees.',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await api.deleteReleaseTracker(tracker.id)
      await loadTrackers()
    } catch (err: unknown) {
      error.value = readError(err, 'Erreur lors de la suppression')
    }
  }

  function repoURL(tracker: ReleaseTracker): string {
    switch (tracker.provider) {
      case 'gitlab':
        return `https://gitlab.com/${tracker.repo_owner}/${tracker.repo_name}`
      case 'gitea':
        return `https://codeberg.org/${tracker.repo_owner}/${tracker.repo_name}`
      default:
        return `https://github.com/${tracker.repo_owner}/${tracker.repo_name}`
    }
  }

  function providerBadge(provider: string): string {
    const map: Record<string, string> = {
      github: 'bg-blue-lt text-blue',
      gitlab: 'bg-orange-lt text-orange',
      gitea: 'bg-teal-lt text-teal',
      forgejo: 'bg-purple-lt text-purple',
      custom: 'bg-secondary-lt text-secondary',
    }
    return map[provider] || 'bg-secondary-lt text-secondary'
  }

  function execStatusBadge(status: string): string {
    const map: Record<string, string> = {
      pending: 'bg-yellow-lt text-yellow',
      running: 'bg-blue-lt text-blue',
      completed: 'bg-success-lt text-success',
      failed: 'bg-danger-lt text-danger',
      skipped: 'bg-secondary-lt text-secondary',
    }
    return map[status] || 'bg-secondary-lt text-secondary'
  }

  function formatRelative(dateStr: string): string {
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
    prefillDockerImage,
    prefillDockerTag,
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
