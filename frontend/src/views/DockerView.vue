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

    <div
      v-if="actionError"
      class="alert alert-danger alert-dismissible mb-3"
      role="alert"
    >
      {{ actionError }}
      <button
        type="button"
        class="btn-close"
        aria-label="Fermer le message d'erreur"
        @click="actionError = ''"
      />
    </div>

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
        <DockerContainersTab
          v-if="activeTab === 'containers'"
          :containers="containers"
          :version-comparisons="versionComparisons"
          :can-run-docker="canRunDocker"
          :action-loading="dockerActionLoading"
          @container-action="handleContainerAction"
        />
        <ComposeProjectsTab
          v-if="activeTab === 'compose'"
          :compose-projects="composeProjects"
          :containers="containers"
          :version-comparisons="versionComparisons"
          :can-run-docker="canRunDocker"
          :action-loading="composeActionLoading"
          @compose-action="handleComposeAction"
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

<script setup>
import { ref, computed } from 'vue'
import { useWebSocket } from '../composables/useWebSocket'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import { useLocalStorage } from '../composables/useLocalStorage'
import { useToast } from '../composables/useToast'
import WsStatusBar from '../components/WsStatusBar.vue'
import DockerContainersTab from '../components/DockerContainersTab.vue'
import ComposeProjectsTab from '../components/ComposeProjectsTab.vue'
import CommandLogPanel from '../components/CommandLogPanel.vue'
import { useCommandStream } from '../composables/useCommandStream'
import apiClient from '../api'

const auth = useAuthStore()
const dialog = useConfirmDialog()

const containers = ref([])
const composeProjects = ref([])
const versionComparisons = ref([])
const activeTab = useLocalStorage('dockerActiveTab', 'containers')
const { value: actionError, showToast: showActionError } = useToast('')

const canRunDocker = computed(() => auth.role === 'admin' || auth.role === 'operator')

const dockerActionLoading = ref({})
const composeActionLoading = ref({})

// Docker console
const showDockerConsole = ref(false)
const dockerLiveCmd = ref(null)

const { openCommandStream, closeStream: closeDockerStream } = useCommandStream({ token: () => auth.token })

const hostMap = computed(() => {
  const map = {}
  containers.value.forEach(c => { if (c.host_id) map[c.host_id] = c.hostname })
  composeProjects.value.forEach(p => { if (p.host_id) map[p.host_id] = p.hostname })
  return map
})

async function handleContainerAction({ hostId, name, action }) {
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

  try {
    const res = await apiClient.sendDockerCommand(hostId, name, action)
    connectDockerStream(res.data.command_id, hostId, name, action)
  } catch (err) {
    showActionError(err.response?.data?.error || err.message, 6000)
  } finally {
    dockerActionLoading.value = { ...dockerActionLoading.value, [name]: null }
  }
}

async function handleComposeAction({ hostId, name, action, workingDir }) {
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
  } catch (err) {
    showActionError(err.response?.data?.error || err.message, 6000)
  } finally {
    composeActionLoading.value = { ...composeActionLoading.value, [name]: null }
  }
}

function connectDockerStream(commandId, hostId, containerName, action) {
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

function closeDockerConsole() {
  closeDockerStream()
  dockerLiveCmd.value = null
  showDockerConsole.value = false
}

const { wsStatus, wsError, retryCount, dataStaleAlert, reconnect } = useWebSocket('/api/v1/ws/docker', (payload) => {
  if (payload.type !== 'docker') return
  containers.value = payload.containers || []
  composeProjects.value = payload.compose_projects || []
  versionComparisons.value = payload.version_comparisons || []
})
</script>
