<template>
  <div>
    <div class="page-header mb-4">
      <div class="page-pretitle">
        <router-link to="/" class="text-decoration-none">Dashboard</router-link>
        <span class="text-muted mx-1">/</span>
        <span>Docker</span>
      </div>
      <h2 class="page-title">Docker</h2>
      <div class="text-secondary">Vue globale de tous les conteneurs sur l'infrastructure</div>
    </div>

    <WsStatusBar :status="wsStatus" :error="wsError" :retry-count="retryCount" @reconnect="reconnect" />

    <div v-if="actionError" class="alert alert-danger alert-dismissible mb-3" role="alert">
      {{ actionError }}
      <button type="button" class="btn-close" @click="actionError = ''" aria-label="Fermer le message d'erreur"></button>
    </div>

    <!-- Tabs -->
    <ul class="nav nav-tabs mb-4">
      <li class="nav-item">
        <a class="nav-link" :class="{ active: activeTab === 'containers' }" href="#" @click.prevent="activeTab = 'containers'">
          Conteneurs
          <span class="badge bg-azure-lt text-azure ms-1">{{ containers.length }}</span>
        </a>
      </li>
      <li class="nav-item">
        <a class="nav-link" :class="{ active: activeTab === 'compose' }" href="#" @click.prevent="activeTab = 'compose'">
          Projets Compose
          <span class="badge bg-azure-lt text-azure ms-1">{{ composeProjects.length }}</span>
        </a>
      </li>
    </ul>

    <div class="docker-layout">
      <div class="docker-main">
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

      <!-- Console Docker Live (panel droit) -->
      <div v-show="showDockerConsole" class="docker-console">
        <div class="card" style="display: flex; flex-direction: column; height: 100%;">
          <div class="card-header d-flex align-items-center justify-content-between">
            <h3 class="card-title">
              <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler me-1" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                <path d="M8 9l3 3l-3 3" />
                <path d="M13 15l3 0" />
                <path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z" />
              </svg>
              Console Live
            </h3>
            <button class="btn btn-sm btn-ghost-secondary" @click="closeDockerConsole" title="Fermer" aria-label="Fermer la console">
              <svg xmlns="http://www.w3.org/2000/svg" class="icon" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                <path d="M18 6l-12 12" />
                <path d="M6 6l12 12" />
              </svg>
            </button>
          </div>
          <div class="card-body d-flex flex-column" style="flex: 1; min-height: 0; padding: 0;">
            <div v-if="!dockerLiveCmd" class="d-flex align-items-center justify-content-center flex-fill text-secondary" style="background: #1e293b; border-radius: 0 0 0.5rem 0.5rem;">
              <div class="text-center p-4">
                <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler mb-2" width="48" height="48" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round" style="opacity: 0.5;">
                  <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                  <path d="M8 9l3 3l-3 3" />
                  <path d="M13 15l3 0" />
                  <path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z" />
                </svg>
                <div style="opacity: 0.7;">Aucune console active</div>
                <div class="small mt-1" style="opacity: 0.5;">Cliquez sur les boutons logs / action pour afficher la sortie</div>
              </div>
            </div>
            <div v-else style="display: flex; flex-direction: column; height: 100%;">
              <div class="px-3 pt-3 pb-2" style="background: #1e293b; border-bottom: 1px solid rgba(255,255,255,0.1);">
                <div class="d-flex align-items-start justify-content-between mb-1">
                  <div class="flex-fill" style="min-width: 0;">
                    <div class="fw-semibold text-light" style="font-size: 0.95rem;">{{ dockerLiveCmd.containerName }}</div>
                    <div class="text-secondary small mt-1">
                      <code style="background: rgba(0,0,0,0.3); padding: 0.15rem 0.4rem; border-radius: 0.25rem; color: #94a3b8;">{{ dockerLiveCmd.action }}</code>
                    </div>
                  </div>
                  <span class="badge ms-2" :class="{
                    'bg-yellow-lt text-yellow': dockerLiveCmd.status === 'running' || dockerLiveCmd.status === 'pending',
                    'bg-green-lt text-green': dockerLiveCmd.status === 'completed',
                    'bg-red-lt text-red': dockerLiveCmd.status === 'failed'
                  }">{{ dockerLiveCmd.status }}</span>
                </div>
              </div>
              <pre
                ref="dockerConsoleOutput"
                class="mb-0 flex-fill"
                style="background:#0f172a;color:#e2e8f0;overflow-y:auto;white-space:pre-wrap;padding:1rem;margin:0;font-family:'Consolas','Monaco','Courier New',monospace;font-size:0.813rem;line-height:1.5;border-radius:0 0 0.5rem 0.5rem;"
              >{{ dockerConsoleText || '(en attente de sortie...)' }}</pre>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Bouton pour réafficher la console Docker -->
    <button
      v-show="!showDockerConsole"
      @click="showDockerConsole = true"
      class="btn btn-primary"
      style="position: fixed; bottom: 1.5rem; right: 1.5rem; z-index: 100;"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="icon me-1" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
        <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
        <path d="M8 9l3 3l-3 3" />
        <path d="M13 15l3 0" />
        <path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z" />
      </svg>
      Console
    </button>
  </div>
</template>

<script setup>
import { ref, computed, nextTick, onUnmounted } from 'vue'
import { useWebSocket } from '../composables/useWebSocket'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import { useLocalStorage } from '../composables/useLocalStorage'
import WsStatusBar from '../components/WsStatusBar.vue'
import DockerContainersTab from '../components/DockerContainersTab.vue'
import ComposeProjectsTab from '../components/ComposeProjectsTab.vue'
import apiClient from '../api'

const auth = useAuthStore()
const dialog = useConfirmDialog()

const containers = ref([])
const composeProjects = ref([])
const versionComparisons = ref([])
const activeTab = useLocalStorage('dockerActiveTab', 'containers')
const actionError = ref('')

const canRunDocker = computed(() => auth.role === 'admin' || auth.role === 'operator')

const dockerActionLoading = ref({})
const composeActionLoading = ref({})

// Docker console
const showDockerConsole = ref(false)
const dockerLiveCmd = ref(null)
const dockerConsoleText = ref('')
const dockerConsoleOutput = ref(null)
let dockerStreamWs = null

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
    connectDockerStream(res.data.command_id, name, action)
  } catch (err) {
    actionError.value = err.response?.data?.error || err.message
    setTimeout(() => { actionError.value = '' }, 6000)
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
    connectDockerStream(res.data.command_id, name, action)
  } catch (err) {
    actionError.value = err.response?.data?.error || err.message
    setTimeout(() => { actionError.value = '' }, 6000)
  } finally {
    composeActionLoading.value = { ...composeActionLoading.value, [name]: null }
  }
}

function connectDockerStream(commandId, containerName, action) {
  const prevWs = dockerStreamWs
  if (prevWs) {
    prevWs.onmessage = null
    prevWs.onerror = null
    prevWs.close()
    dockerStreamWs = null
  }

  dockerConsoleText.value = ''
  dockerLiveCmd.value = { commandId, containerName, action, status: 'pending' }
  showDockerConsole.value = true

  const token = auth.token
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const ws = new WebSocket(`${proto}://${location.host}/api/v1/ws/commands/stream/${commandId}`)
  dockerStreamWs = ws

  const closeWs = () => {
    setTimeout(() => {
      ws.close()
      if (dockerStreamWs === ws) dockerStreamWs = null
    }, 500)
  }

  ws.onopen = () => { ws.send(JSON.stringify({ type: 'auth', token })) }

  ws.onmessage = (event) => {
    if (dockerStreamWs !== ws) return
    try {
      const msg = JSON.parse(event.data)
      if (msg.type === 'cmd_stream_init') {
        dockerConsoleText.value = msg.output || ''
        if (dockerLiveCmd.value) dockerLiveCmd.value.status = msg.status
        if (msg.status === 'completed' || msg.status === 'failed') closeWs()
      } else if (msg.type === 'cmd_stream') {
        dockerConsoleText.value += msg.chunk || ''
        scrollDockerConsole()
      } else if (msg.type === 'cmd_status_update') {
        if (dockerLiveCmd.value) dockerLiveCmd.value.status = msg.status
        if (msg.status === 'completed' || msg.status === 'failed') closeWs()
      }
    } catch {}
  }

  ws.onerror = () => {
    if (dockerStreamWs === ws && dockerLiveCmd.value) dockerLiveCmd.value.status = 'failed'
  }
}

function closeDockerConsole() {
  const ws = dockerStreamWs
  if (ws) { ws.onmessage = null; ws.onerror = null; ws.close(); dockerStreamWs = null }
  dockerLiveCmd.value = null
  dockerConsoleText.value = ''
  showDockerConsole.value = false
}

async function scrollDockerConsole() {
  await nextTick()
  if (dockerConsoleOutput.value) dockerConsoleOutput.value.scrollTop = dockerConsoleOutput.value.scrollHeight
}

onUnmounted(() => {
  if (dockerStreamWs) { dockerStreamWs.onmessage = null; dockerStreamWs.onerror = null; dockerStreamWs.close(); dockerStreamWs = null }
})

const { wsStatus, wsError, retryCount, reconnect } = useWebSocket('/api/v1/ws/docker', (payload) => {
  if (payload.type !== 'docker') return
  containers.value = payload.containers || []
  composeProjects.value = payload.compose_projects || []
  versionComparisons.value = payload.version_comparisons || []
})
</script>

<style scoped>
.docker-layout {
  display: flex;
  gap: 1rem;
  align-items: flex-start;
}

.docker-main {
  flex: 1;
  min-width: 0;
}

.docker-console {
  width: 38%;
  min-width: 380px;
  height: calc(100vh - 160px);
  display: flex;
  flex-direction: column;
  position: sticky;
  top: 1rem;
}

@media (max-width: 991px) {
  .docker-layout { flex-direction: column; align-items: stretch; }
  .docker-console { width: 100%; min-width: 0; height: 60vh; position: static; }
}
</style>
