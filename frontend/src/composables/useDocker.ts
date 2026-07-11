import { ref, computed } from 'vue'
import { useWebSocket } from './useWebSocket'
import type { WSDockerSnapshot } from '../types/ws'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from './useConfirmDialog'
import { addToast } from './useGlobalToast'
import { useCommandStream } from './useCommandStream'
import apiClient from '../api'
import type { DockerContainer, ComposeProject, VersionComparison } from '../types/docker'
import { getApiErrorMessage } from '../api/client'

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
        title: `${action === 'stop' ? 'Arrêter' : 'Redémarrer'} le conteneur`,
        message: `Confirmer : ${action} du conteneur « ${name} » ?`,
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
    } catch (err: unknown) {
      if (originalContainer) {
        const prevState = originalContainer.state
        containers.value = containers.value.map((c) =>
          c.name === name && c.host_id === hostId ? { ...c, state: prevState } : c
        )
      }
      addToast(getApiErrorMessage(err, 'Erreur Docker'), 'error', 6000)
    } finally {
      dockerActionLoading.value = { ...dockerActionLoading.value, [name]: null }
    }
  }

  async function handleComposeAction({ hostId, name, action, workingDir }: { hostId: string; name: string; action: string; workingDir?: string }): Promise<void> {
    if (composeActionLoading.value[name]) return

    if (action === 'compose_down' || action === 'compose_restart') {
      const ok = await dialog.confirm({
        title: `${action === 'compose_down' ? 'Arrêter' : 'Redémarrer'} le projet`,
        message: `Confirmer : ${action.replace('compose_', '')} du projet « ${name} » ?`,
        variant: 'warning',
      })
      if (!ok) return
    }

    composeActionLoading.value = { ...composeActionLoading.value, [name]: action }

    try {
      const res = await apiClient.sendDockerCommand(hostId, name, action, workingDir)
      connectDockerStream(res.data.command_id, hostId, name, action)
    } catch (err: unknown) {
      addToast(getApiErrorMessage(err, 'Erreur Docker'), 'error', 6000)
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
    showDockerConsole,
    dockerLiveCmd,
    handleContainerAction,
    handleComposeAction,
    closeDockerConsole,
    wsStatus,
    wsError,
    retryCount,
    dataStaleAlert,
    reconnect,
  }
}
