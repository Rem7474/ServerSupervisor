import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useWebSocket } from './useWebSocket'
import type { WSDockerSnapshot } from '../types/ws'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from './useConfirmDialog'
import { addToast } from './useGlobalToast'
import { useCommandStream } from './useCommandStream'
import { usePendingCommand } from './usePendingCommand'
import apiClient from '../api'
import type { DockerContainer, ComposeProject, VersionComparison } from '../types/docker'
import { getApiErrorMessage } from '../api/client'
import { confirmBulkAction } from '../utils/bulkActionHelpers'

interface DockerLiveCmd {
  id: string
  host_name?: string
  module?: string
  action?: string
  target?: string
  status?: string
  output?: string
}

export function useDocker() {
  const { t } = useI18n()
  const auth = useAuthStore()
  const dialog = useConfirmDialog()

  const containers = ref<DockerContainer[]>([])
  const composeProjects = ref<ComposeProject[]>([])
  const versionComparisons = ref<VersionComparison[]>([])

  const canRunDocker = computed(() => auth.role === 'admin' || auth.role === 'operator')
  const runningCount = computed(() => containers.value.filter((c) => c.state === 'running').length)

  const dockerActionLoading = ref<Record<string, string | null>>({})
  const composeActionLoading = ref<Record<string, string | null>>({})

  // Docker console
  const showDockerConsole = ref(false)
  const dockerLiveCmd = ref<DockerLiveCmd | null>(null)

  const { openCommandStream, closeStream: closeDockerStream } = useCommandStream()
  const pendingCommand = usePendingCommand()

  const hostMap = computed<Record<string, string>>(() => {
    const map: Record<string, string> = {}
    containers.value.forEach((c) => { if (c.host_id) map[c.host_id] = c.hostname })
    composeProjects.value.forEach((p) => { if (p.host_id) map[p.host_id] = p.hostname })
    return map
  })

  async function handleContainerAction({ hostId, name, action }: { hostId: string; name: string; action: string }): Promise<void> {
    if (dockerActionLoading.value[name]) return

    if (action === 'stop' || action === 'restart') {
      const ok = await dialog.confirm({
        title: action === 'stop' ? t('docker.stopContainerTitle') : t('docker.restartContainerTitle'),
        message: t('docker.confirmContainerAction', { action, name }),
        variant: 'warning',
      })
      if (!ok) return
    }

    dockerActionLoading.value = { ...dockerActionLoading.value, [name]: action }

    const optimisticStates: Record<string, string> = { stop: 'stopping', start: 'starting', restart: 'restarting' }
    const originalContainer = containers.value.find((c) => c.name === name && c.host_id === hostId)
    const nextState = optimisticStates[action]
    if (originalContainer && nextState) {
      containers.value = containers.value.map((c) =>
        c.name === name && c.host_id === hostId ? { ...c, state: nextState } : c
      )
    }

    try {
      const res = await apiClient.sendDockerCommand(hostId, name, action)
      connectDockerStream(res.data.command_id, hostId, name, action)
      await pendingCommand.track(res.data.command_id)
    } catch (err: unknown) {
      if (originalContainer) {
        const prevState = originalContainer.state
        containers.value = containers.value.map((c) =>
          c.name === name && c.host_id === hostId ? { ...c, state: prevState } : c
        )
      }
      addToast(getApiErrorMessage(err, t('docker.dockerError')), 'error', 6000)
    } finally {
      dockerActionLoading.value = { ...dockerActionLoading.value, [name]: null }
    }
  }

  const bulkActionLoading = ref(false)

  // Fire-and-forget in parallel across the selected containers — no live
  // console per item (that doesn't scale past a couple of containers), just
  // a summary toast. The WS docker snapshot picks up the resulting state
  // changes on its own, same as it already does for every other command.
  async function handleBulkContainerAction(containers: DockerContainer[], action: string): Promise<void> {
    if (!containers.length || bulkActionLoading.value) return
    const verb = action === 'start' ? t('docker.verbStart') : action === 'stop' ? t('docker.verbStop') : t('docker.verbRestart')
    const names = containers.map((c) => c.name).join(', ')
    const ok = await confirmBulkAction(
      verb,
      containers.length,
      t('docker.bulkConfirmMessage', { verb, count: containers.length, names }, containers.length)
    )
    if (!ok) return

    bulkActionLoading.value = true
    try {
      const results = await Promise.allSettled(
        containers.map((c) => apiClient.sendDockerCommand(c.host_id, c.name, action))
      )
      const succeeded = results.filter((r) => r.status === 'fulfilled').length
      const failed = results.length - succeeded
      if (failed === 0) {
        addToast(t('docker.commandsSentSuccess', { count: succeeded }, succeeded), 'success')
      } else {
        addToast(
          t('docker.bulkSummary', { succeeded, failed }),
          failed === results.length ? 'error' : 'warning',
          6000
        )
      }
      // Keep the bulk-action button spinning until every dispatched command
      // actually finishes, not just until every dispatch request is acked.
      await Promise.all(
        results
          .filter((r): r is PromiseFulfilledResult<Awaited<ReturnType<typeof apiClient.sendDockerCommand>>> => r.status === 'fulfilled')
          .map((r) => pendingCommand.track(r.value.data?.command_id))
      )
    } finally {
      bulkActionLoading.value = false
    }
  }

  async function handleComposeAction({ hostId, name, action, workingDir }: { hostId: string; name: string; action: string; workingDir?: string }): Promise<void> {
    if (composeActionLoading.value[name]) return

    if (action === 'compose_down' || action === 'compose_restart') {
      const ok = await dialog.confirm({
        title: action === 'compose_down' ? t('docker.stopProjectTitle') : t('docker.restartProjectTitle'),
        message: t('docker.confirmComposeAction', { action: action.replace('compose_', ''), name }),
        variant: 'warning',
      })
      if (!ok) return
    }

    composeActionLoading.value = { ...composeActionLoading.value, [name]: action }

    try {
      const res = await apiClient.sendDockerCommand(hostId, name, action, workingDir)
      connectDockerStream(res.data.command_id, hostId, name, action)
      await pendingCommand.track(res.data.command_id)
    } catch (err: unknown) {
      addToast(getApiErrorMessage(err, t('docker.dockerError')), 'error', 6000)
    } finally {
      composeActionLoading.value = { ...composeActionLoading.value, [name]: null }
    }
  }

  function connectDockerStream(commandId: string, hostId: string, containerName: string, action: string): void {
    const hostName = hostMap.value[hostId] || containerName
    dockerLiveCmd.value = { id: commandId, host_name: hostName, module: 'docker', action, target: containerName, status: 'pending', output: '' }
    showDockerConsole.value = true
    openCommandStream(commandId, {
      onInit: (p) => {
        if (dockerLiveCmd.value?.id !== commandId) return
        dockerLiveCmd.value = { ...dockerLiveCmd.value, status: p.status, output: p.output || '' }
      },
      onChunk: (p) => {
        if (dockerLiveCmd.value?.id !== commandId) return
        dockerLiveCmd.value = { ...dockerLiveCmd.value, output: (dockerLiveCmd.value.output || '') + (p.chunk || '') }
      },
      onStatus: (p) => {
        if (dockerLiveCmd.value?.id !== commandId) return
        dockerLiveCmd.value = { ...dockerLiveCmd.value, status: p.status }
      },
    })
  }

  function closeDockerConsole(): void {
    closeDockerStream()
    dockerLiveCmd.value = null
    showDockerConsole.value = false
  }

  const { wsStatus, wsError, retryCount, dataStaleAlert, reconnect } = useWebSocket<WSDockerSnapshot>('/api/v1/ws/docker', (payload) => {
    if (payload.type !== 'docker') return
    containers.value = payload.containers || []
    composeProjects.value = payload.compose_projects || []
    versionComparisons.value = payload.version_comparisons || []
  }, { debounceMs: 750 })

  return {
    containers,
    composeProjects,
    versionComparisons,
    canRunDocker,
    runningCount,
    dockerActionLoading,
    composeActionLoading,
    bulkActionLoading,
    showDockerConsole,
    dockerLiveCmd,
    handleContainerAction,
    handleBulkContainerAction,
    handleComposeAction,
    closeDockerConsole,
    wsStatus,
    wsError,
    retryCount,
    dataStaleAlert,
    reconnect,
  }
}
