import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import api from '../api'
import { formatDateTime } from '../utils/formatters'
import { useCommandStream } from './useCommandStream'
import { getApiErrorMessage, isApiAbort } from '../api/client'
import { useAbortSignal } from './useAbortSignal'
import type { ReleaseTracker, ReleaseTrackerExecution, ReleaseTrackerRequest, ReleaseVersionHistoryItem } from '../types/tracker'
import type { ComposeProject } from '../types/docker'
import type { Host } from '../types/host'
import type { WebhookFormData } from './useWebhookForm'

interface CmdRow { id: string; status?: string; output?: string; [key: string]: unknown }
// The API enriches the tracker with the resolved release URL (not in the Go model).
type TrackerView = ReleaseTracker & { release_url?: string }

export function useReleaseTrackerDetail() {
  const { t } = useI18n()
  const route = useRoute()
  const id = route.params.id as string
  const signal = useAbortSignal()

  const tracker = ref<TrackerView | null>(null)
  const executions = ref<ReleaseTrackerExecution[]>([])
  const versionHistory = ref<ReleaseVersionHistoryItem[]>([])
  const hosts = ref<Host[]>([])
  const loading = ref(false)
  const error = ref('')
  const historyLoading = ref(false)
  const checking = ref(false)
  const running = ref(false)
  const selectedCmd = ref<CmdRow | null>(null)
  const showConsole = ref(false)
  const nowTick = ref(Date.now())
  let cooldownTimer: number | null = null

  const composeProjects = ref<ComposeProject[]>([])
  const tasksYaml = ref('')
  const loadingSnippet = ref(false)

  const showModal = ref(false)
  const saving = ref(false)
  const modalError = ref('')

  const { openCommandStream, closeStream } = useCommandStream()

  const canRunManually = computed(() => {
    if (!tracker.value) return false
    // Any tracker can be in monitor-only mode (no host/task dispatch configured).
    if (!tracker.value.host_id || !tracker.value.custom_task_id) {
      return false
    }
    return true
  })

  const runDisabledReason = computed(() => {
    if (!tracker.value) return ''
    if (!tracker.value.host_id || !tracker.value.custom_task_id) {
      return t('webhooks.monitorOnlyModeHint')
    }
    return ''
  })

  const cooldownRemainingMs = computed(() => {
    const trk = tracker.value
    if (!trk) return 0
    const hours = Number(trk.cooldown_hours || 0)
    if (!hours || hours <= 0 || !trk.last_release_detected_at) return 0

    const detectedAt = new Date(trk.last_release_detected_at).getTime()
    if (!Number.isFinite(detectedAt)) return 0

    if (trk.last_triggered_at) {
      const triggeredAt = new Date(trk.last_triggered_at).getTime()
      if (Number.isFinite(triggeredAt) && triggeredAt >= detectedAt) return 0
    }

    const endsAt = detectedAt + (hours * 60 * 60 * 1000)
    return Math.max(0, endsAt - nowTick.value)
  })

  const cooldownActive = computed(() => cooldownRemainingMs.value > 0)

  const cooldownRemainingText = computed(() => {
    const ms = cooldownRemainingMs.value
    if (ms <= 0) return '0m'
    const totalMinutes = Math.ceil(ms / 60000)
    const days = Math.floor(totalMinutes / (24 * 60))
    const hours = Math.floor((totalMinutes % (24 * 60)) / 60)
    const minutes = totalMinutes % 60
    if (days > 0) return `${days}${t('webhooks.daySuffix')} ${hours}h`
    if (hours > 0) return `${hours}h ${minutes}m`
    return `${minutes}m`
  })

  const cooldownEtaText = computed(() => {
    const trk = tracker.value
    if (!trk) return '-'
    const hours = Number(trk.cooldown_hours || 0)
    if (!hours || hours <= 0 || !trk.last_release_detected_at) return '-'

    const detectedAt = new Date(trk.last_release_detected_at).getTime()
    if (!Number.isFinite(detectedAt)) return '-'

    return formatDateTime(new Date(detectedAt + (hours * 60 * 60 * 1000)).toISOString())
  })

  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const [res, hostsRes] = await Promise.all([api.getReleaseTracker(id, signal), api.getHosts(signal)])
      tracker.value = res.data.tracker
      executions.value = res.data.executions || []
      hosts.value = hostsRes.data || []
    } catch (e: unknown) {
      if (isApiAbort(e)) return
      error.value = getApiErrorMessage(e, t('webhooks.loadErrorGeneric'))
    } finally {
      loading.value = false
    }

    await loadVersionHistory()

    if (tracker.value?.host_id) {
      await loadSnippetData(tracker.value.host_id)
    }
  }

  async function loadSnippetData(hostId: string): Promise<void> {
    loadingSnippet.value = true
    try {
      const [composeRes, yamlRes] = await Promise.all([
        api.getHostComposeProjects(hostId).catch(() => ({ data: [] })),
        api.getHostTasksYaml(hostId).catch(() => ({ data: { yaml: '' } })),
      ])
      composeProjects.value = composeRes.data || []
      tasksYaml.value = yamlRes.data?.yaml || ''
    } finally {
      loadingSnippet.value = false
    }
  }

  async function loadVersionHistory(): Promise<void> {
    historyLoading.value = true
    try {
      const res = await api.getReleaseTrackerVersionHistory(id)
      versionHistory.value = res.data.history || []
    } catch {
      versionHistory.value = []
    } finally {
      historyLoading.value = false
    }
  }

  async function loadExecutions(): Promise<void> {
    try {
      const res = await api.getReleaseTrackerExecutions(id)
      executions.value = res.data.executions || []
    } catch { /* ignore */ }
  }

  function clearExecutionLogs(): void {
    closeStream()
    selectedCmd.value = null
    showConsole.value = false
  }

  function connectExecutionStream(commandId: string): void {
    openCommandStream(commandId, {
      onInit(payload) {
        if (!selectedCmd.value || selectedCmd.value.id !== commandId) return
        selectedCmd.value = {
          ...selectedCmd.value,
          status: payload.status || selectedCmd.value.status,
          output: payload.output ?? selectedCmd.value.output,
        }
      },
      onChunk(payload) {
        if (!selectedCmd.value || selectedCmd.value.id !== commandId) return
        selectedCmd.value = {
          ...selectedCmd.value,
          output: (selectedCmd.value.output || '') + (payload.chunk || ''),
        }
      },
      onStatus(payload) {
        if (!selectedCmd.value || selectedCmd.value.id !== commandId) return
        selectedCmd.value = {
          ...selectedCmd.value,
          status: payload.status || selectedCmd.value.status,
          output: payload.output ?? selectedCmd.value.output,
        }

        const idx = executions.value.findIndex((e: ReleaseTrackerExecution) => e.command_id === commandId)
        if (idx !== -1) {
          const next = [...executions.value]
          next[idx] = { ...next[idx], status: payload.status || next[idx].status }
          executions.value = next
        }
      },
    })
  }

  async function openExecutionLogs(commandId: string): Promise<void> {
    closeStream()
    try {
      const res = await api.getCommandStatus(commandId)
      const cmd = res.data
      selectedCmd.value = cmd as unknown as CmdRow
      showConsole.value = true
      if (cmd?.status === 'pending' || cmd?.status === 'running') {
        connectExecutionStream(commandId)
      }
    } catch {
      error.value = t('webhooks.couldNotLoadCommandLogsError')
    }
  }

  async function runManually(): Promise<void> {
    if (!canRunManually.value) {
      error.value = runDisabledReason.value || t('webhooks.manualRunUnavailableError')
      return
    }
    running.value = true
    try {
      await api.runReleaseTracker(id)
      setTimeout(async () => {
        await load()
        running.value = false
      }, 2000)
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e, t('webhooks.triggerError'))
      running.value = false
    }
  }

  async function triggerCheck(): Promise<void> {
    checking.value = true
    try {
      await api.checkReleaseTrackerNow(id)
      setTimeout(async () => {
        await load()
        checking.value = false
      }, 2000)
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e, t('common.error'))
      checking.value = false
    }
  }

  function openEdit(): void {
    modalError.value = ''
    showModal.value = true
  }

  async function saveEdit(payload: WebhookFormData): Promise<void> {
    saving.value = true
    modalError.value = ''
    try {
      await api.updateReleaseTracker(id, payload as unknown as ReleaseTrackerRequest)
      closeEdit()
      await load()
    } catch (e: unknown) {
      modalError.value = getApiErrorMessage(e, t('common.error'))
    } finally {
      saving.value = false
    }
  }

  function closeEdit(): void {
    showModal.value = false
    modalError.value = ''
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

  onMounted(() => {
    load()
    cooldownTimer = window.setInterval(() => {
      nowTick.value = Date.now()
    }, 60000)
  })
  onUnmounted(() => {
    if (cooldownTimer !== null) {
      window.clearInterval(cooldownTimer)
      cooldownTimer = null
    }
    closeStream()
  })

  return {
    id,
    tracker,
    executions,
    versionHistory,
    hosts,
    loading,
    error,
    historyLoading,
    checking,
    running,
    selectedCmd,
    showConsole,
    composeProjects,
    tasksYaml,
    loadingSnippet,
    showModal,
    saving,
    modalError,
    canRunManually,
    runDisabledReason,
    cooldownActive,
    cooldownRemainingText,
    cooldownEtaText,
    loadExecutions,
    clearExecutionLogs,
    openExecutionLogs,
    runManually,
    triggerCheck,
    openEdit,
    saveEdit,
    closeEdit,
    providerBadge,
  }
}
