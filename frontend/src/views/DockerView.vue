<template>
  <div>
    <div class="page-header mb-4">
      <div class="page-pretitle">
        <router-link
          to="/"
          class="text-decoration-none"
        >
          Dashboard
        </router-link>
        <span class="text-muted mx-1">/</span>
        <span>Docker</span>
      </div>
      <h2 class="page-title">
        Docker
      </h2>
      <div class="text-secondary">
        Vue globale de tous les conteneurs sur l'infrastructure
      </div>
    </div>

    <WsStatusBar
      :status="wsStatus"
      :error="wsError"
      :retry-count="retryCount"
      :data-stale-alert="dataStaleAlert"
      @reconnect="reconnect"
      @dismiss-stale-alert="dataStaleAlert = false"
    />

    <!-- Tabs -->
    <ul class="nav nav-tabs mb-4">
      <li class="nav-item">
        <a
          class="nav-link"
          :class="{ active: activeTab === 'containers' }"
          href="#"
          @click.prevent="activeTab = 'containers'"
        >
          Conteneurs
          <span class="badge bg-azure-lt text-azure ms-1">{{ containers.length }}</span>
          <span
            v-if="runningCount > 0"
            class="badge bg-green-lt text-green ms-1"
          >{{ runningCount }} actifs</span>
        </a>
      </li>
      <li class="nav-item">
        <a
          class="nav-link"
          :class="{ active: activeTab === 'compose' }"
          href="#"
          @click.prevent="activeTab = 'compose'"
        >
          Projets Compose
          <span class="badge bg-azure-lt text-azure ms-1">{{ composeProjects.length }}</span>
        </a>
      </li>
    </ul>

    <div class="side-layout">
      <div class="side-main">
        <ErrorBoundary
          v-if="activeTab === 'containers'"
          title="Erreur lors du rendu des conteneurs"
        >
          <DockerContainersTab
            :containers="(containers as any)"
            :version-comparisons="(versionComparisons as any)"
            :can-run-docker="canRunDocker"
            :action-loading="(dockerActionLoading as any)"
            @container-action="(handleContainerAction as any)"
          />
        </ErrorBoundary>
        <ComposeProjectsTab
          v-if="activeTab === 'compose'"
          :compose-projects="(composeProjects as any)"
          :containers="(containers as any)"
          :version-comparisons="(versionComparisons as any)"
          :can-run-docker="canRunDocker"
          :action-loading="(composeActionLoading as any)"
          @compose-action="(handleComposeAction as any)"
        />
      </div>

      <CommandLogPanel
        :command="dockerLiveCmd"
        :show="showDockerConsole"
        title="Console Live"
        empty-text="Aucune console active"
        wrapper-class="side-panel"
        @open="showDockerConsole = true"
        @close="closeDockerConsole"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useWebSocket } from '../composables/useWebSocket'
import type { WSDockerSnapshot } from '../types/ws'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import { useLocalStorage } from '../composables/useLocalStorage'
import { addToast } from '../composables/useGlobalToast'
import WsStatusBar from '../components/WsStatusBar.vue'
import ErrorBoundary from '../components/common/ErrorBoundary.vue'
import DockerContainersTab from '../components/docker/DockerContainersTab.vue'
import ComposeProjectsTab from '../components/docker/ComposeProjectsTab.vue'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'
import { useCommandStream } from '../composables/useCommandStream'
import apiClient from '../api'
import type { DockerContainer, ComposeProject } from '../types/docker'

const auth = useAuthStore()
const dialog = useConfirmDialog()

const containers = ref<DockerContainer[]>([])
const composeProjects = ref<ComposeProject[]>([])
const versionComparisons = ref<any[]>([])
const activeTab = useLocalStorage('dockerActiveTab', 'containers')

const canRunDocker = computed(() => auth.role === 'admin' || auth.role === 'operator')
const runningCount = computed(() => containers.value.filter((c) => c.state === 'running').length)

const dockerActionLoading = ref<Record<string, string | null>>({})
const composeActionLoading = ref<Record<string, string | null>>({})

// Docker console
const showDockerConsole = ref(false)
const dockerLiveCmd = ref<any>(null)

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
  } catch (err: any) {
    if (originalContainer) {
      const prevState = originalContainer.state
      containers.value = containers.value.map((c) =>
        c.name === name && c.host_id === hostId ? { ...c, state: prevState } : c
      )
    }
    addToast(err?.response?.data?.error || err?.message || 'Erreur Docker', 'error', 6000)
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
  } catch (err: any) {
    addToast(err?.response?.data?.error || err?.message || 'Erreur Docker', 'error', 6000)
  } finally {
    composeActionLoading.value = { ...composeActionLoading.value, [name]: null }
  }
}

function connectDockerStream(commandId: string, hostId: string, containerName: string, action: string): void {
  const hostName = hostMap.value[hostId] || containerName
  dockerLiveCmd.value = { id: commandId, host_name: hostName, module: 'docker', action, target: containerName, status: 'pending', output: '' }
  showDockerConsole.value = true
  openCommandStream(commandId, {
    onInit: (p: any) => {
      if (dockerLiveCmd.value?.id !== commandId) return
      dockerLiveCmd.value = { ...dockerLiveCmd.value, status: p.status, output: p.output || '' }
    },
    onChunk: (p: any) => {
      if (dockerLiveCmd.value?.id !== commandId) return
      dockerLiveCmd.value = { ...dockerLiveCmd.value, output: (dockerLiveCmd.value.output || '') + (p.chunk || '') }
    },
    onStatus: (p: any) => {
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
</script>
