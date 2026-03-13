<template>
  <div class="host-detail-page">
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <div class="page-pretitle">
            <router-link to="/" class="text-decoration-none">Dashboard</router-link>
            <span class="text-muted mx-1">/</span>
            <span>Hôte</span>
          </div>
          <h2 class="page-title">{{ host?.name || host?.hostname || 'Chargement...' }}</h2>
          <div class="text-secondary">
            {{ host?.hostname || 'Non connecté' }} - {{ host?.os || 'OS inconnu' }} - {{ host?.ip_address }}
            <span v-if="host?.last_seen">- Dernière activité: <RelativeTime :date="host.last_seen" /></span>
          </div>
        </div>
        <div class="d-flex align-items-center gap-2">
          <button @click="isEditing = true" class="btn btn-outline-secondary">
            <svg class="icon me-1" width="16" height="16" viewBox="0 0 24 24" stroke="currentColor" fill="none" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/>
              <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/>
            </svg>
            Modifier
          </button>
          <button @click="deleteHost" class="btn btn-outline-danger">
            <svg class="icon me-1" width="16" height="16" viewBox="0 0 24 24" stroke="currentColor" fill="none" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M3 6h18"/><path d="M8 6V4h8v2"/><path d="M19 6l-1 14a2 2 0 01-2 2H8a2 2 0 01-2-2L5 6"/>
            </svg>
            Supprimer
          </button>
          <span v-if="host" :class="hostStatusClass(host.status)">{{ formatHostStatus(host.status) }}</span>
          <span
            v-if="host?.agent_version"
            :class="isAgentUpToDate(host.agent_version) ? 'badge bg-green-lt text-green' : 'badge bg-yellow-lt text-yellow'"
            :title="isAgentUpToDate(host.agent_version) ? 'Agent à jour' : 'Mise à jour disponible'"
          >
            Agent v{{ host.agent_version }}
          </span>
        </div>
      </div>
    </div>

    <WsStatusBar :status="wsStatus" :error="wsError" :retry-count="retryCount" @reconnect="reconnect" />

    <div class="host-layout">
      <div class="host-panel-main">
        <HostEditForm
          v-if="isEditing"
          :host-id="hostId"
          :host="host"
          @close="isEditing = false"
          @updated="host = $event"
        />

        <HostDetailTabs
          v-model="activeTab"
          :can-run-apt="canRunApt"
          :containers-count="containers.length"
          :pending-packages="aptStatus?.pending_packages || 0"
          :commands-count="cmdHistory.length"
          :tasks-count="tasksCount"
        />

        <div v-show="activeTab === 'metrics'">
          <HostMetricsPanel :hostId="hostId" :metrics="metrics" />
          <DiskMetricsCard :hostId="hostId" :initialMetrics="diskMetrics" class="mb-4" />
          <DiskHealthCard :hostId="hostId" :initialHealth="diskHealth" class="mb-4" />
        </div>

        <div v-show="activeTab === 'docker'">
          <HostDockerTab :containers="containers" :version-comparisons="versionComparisons" />
        </div>

        <div v-show="activeTab === 'apt'">
          <HostAptTab
            :apt-status="aptStatus"
            :can-run-apt="canRunApt"
            :apt-cmd-loading="aptCmdLoading"
            @run-apt-command="sendAptCmd"
          />
        </div>

        <div v-show="activeTab === 'commandes'">
          <HostCommandsTab :commands="cmdHistory" @watch-command="openCommand" />
        </div>

        <div v-show="activeTab === 'systeme'">
          <HostSystemTab v-if="canRunApt" :host-id="hostId" :can-run-apt="canRunApt" @open-command="openCommand" @history-changed="loadCmdHistoryRefresh" />
        </div>

        <div v-show="activeTab === 'processus'">
          <HostProcessesPanel v-if="canRunApt" :hostId="hostId" :can-run="canRunApt" @history-changed="loadCmdHistoryRefresh" />
        </div>

        <div v-show="activeTab === 'planifiees'">
          <HostTasksTab
            :host-id="hostId"
            :can-run-apt="canRunApt"
            :active="activeTab === 'planifiees'"
            @open-command="openCommand"
            @tasks-count="tasksCount = $event"
            @history-changed="loadCmdHistoryRefresh"
          />
        </div>
      </div>

      <CommandLogPanel
        :command="liveCommand"
        :show="showConsole"
        title="Console Live"
        empty-text="Aucune console active"
        wrapper-class="host-panel-right"
        :clearable="true"
        @open="showConsole = true"
        @close="closeConsoleAndStream"
        @clear="clearConsoleOutput"
      />
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import RelativeTime from '../components/RelativeTime.vue'
import DiskMetricsCard from '../components/DiskMetricsCard.vue'
import DiskHealthCard from '../components/DiskHealthCard.vue'
import HostMetricsPanel from '../components/HostMetricsPanel.vue'
import HostProcessesPanel from '../components/HostProcessesPanel.vue'
import WsStatusBar from '../components/WsStatusBar.vue'
import HostAptTab from '../components/host/HostAptTab.vue'
import HostCommandsTab from '../components/host/HostCommandsTab.vue'
import HostDetailTabs from '../components/host/HostDetailTabs.vue'
import HostDockerTab from '../components/host/HostDockerTab.vue'
import HostEditForm from '../components/host/HostEditForm.vue'
import HostSystemTab from '../components/host/HostSystemTab.vue'
import HostTasksTab from '../components/host/HostTasksTab.vue'
import CommandLogPanel from '../components/CommandLogPanel.vue'
import apiClient from '../api'
import { useHostCommandConsole } from '../composables/useHostCommandConsole'
import { useCommandStream } from '../composables/useCommandStream'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import { useWebSocket } from '../composables/useWebSocket'
import { useAuthStore } from '../stores/auth'
import { formatHostStatus, hostStatusClass } from '../utils/formatHostStatus'

const route = useRoute()
const router = useRouter()
const hostId = route.params.id

const auth = useAuthStore()
const dialog = useConfirmDialog()
const canRunApt = computed(() => auth.canManage)

const activeTab = ref('metrics')
const isEditing = ref(false)
const tasksCount = ref(0)
const aptCmdLoading = ref('')

const host = ref(null)
const metrics = ref(null)
const containers = ref([])
const versionComparisons = ref([])
const aptStatus = ref(null)
const cmdHistory = ref([])
const diskMetrics = ref(null)
const diskHealth = ref(null)
const latestAgentVersion = ref('')

const { liveCommand, showConsole, openCommand: _openCommand, closeConsole, updateCommand } = useHostCommandConsole()
const { openCommandStream, closeStream } = useCommandStream({ token: () => auth.token })

function openCommand(rawCmd) {
  _openCommand({ ...rawCmd, host_name: host.value?.hostname })
}

function connectStream(commandId) {
  openCommandStream(commandId, {
    onInit: (p) => {
      updateCommand({ ...liveCommand.value, status: p.status, output: p.output || '' })
      nextTick(() => {})
    },
    onChunk: (p) => {
      updateCommand({ ...liveCommand.value, output: (liveCommand.value?.output || '') + p.chunk })
    },
    onStatus: (p) => {
      updateCommand({ ...liveCommand.value, status: p.status })
      if (p.status === 'completed' || p.status === 'failed') {
        loadCmdHistoryRefresh()
      }
    },
  })
}

watch(() => liveCommand.value?.id, (id) => {
  if (!id || !showConsole.value) return
  connectStream(id)
})

watch(showConsole, (show) => {
  if (!show) {
    closeStream()
  } else if (liveCommand.value?.id) {
    connectStream(liveCommand.value.id)
  }
})

function closeConsoleAndStream() {
  closeStream()
  closeConsole()
}

function clearConsoleOutput() {
  if (liveCommand.value) updateCommand({ ...liveCommand.value, output: '' })
}

const { wsStatus, wsError, retryCount, reconnect } = useWebSocket(`/api/v1/ws/hosts/${hostId}`, (payload) => {
  if (payload.type !== 'host_detail') return
  host.value = payload.host
  metrics.value = payload.metrics
  containers.value = payload.containers || []
  versionComparisons.value = payload.version_comparisons || []
  aptStatus.value = payload.apt_status
}, { debounceMs: 200 })

async function sendAptCmd(command) {
  const confirmed = await dialog.confirm({
    title: `apt ${command}`,
    message: `Exécuter sur : ${host.value?.hostname}`,
    variant: command === 'dist-upgrade' ? 'danger' : 'warning',
  })

  if (!confirmed) return

  aptCmdLoading.value = command
  try {
    const response = await apiClient.sendAptCommand([hostId], command)
    if (response.data?.commands?.length > 0) {
      const cmd = response.data.commands[0]
      if (cmd.command_id) {
        openCommand({ id: cmd.command_id, module: 'apt', action: command, status: 'pending', output: '' })
      }
    }
  } catch (e) {
    await dialog.confirm({
      title: 'Erreur',
      message: e.response?.data?.error || e.message,
      variant: 'danger',
    })
  } finally {
    aptCmdLoading.value = ''
  }
}

function isAgentUpToDate(version) {
  if (!version || !latestAgentVersion.value) return false
  return version === latestAgentVersion.value
}

async function loadComplete() {
  try {
    const res = await apiClient.getHostComplete(hostId)
    const d = res.data
    if (d.host) host.value = d.host
    if (d.metrics) metrics.value = d.metrics
    if (d.containers) containers.value = d.containers
    if (d.apt_status) aptStatus.value = d.apt_status
    if (d.disk_metrics) diskMetrics.value = d.disk_metrics
    if (d.disk_health) diskHealth.value = d.disk_health
    if (d.command_history) cmdHistory.value = d.command_history
    if (d.latest_agent_version) latestAgentVersion.value = d.latest_agent_version
  } catch {
    // Non-critical — WS will populate live data
  }
}

async function loadCmdHistoryRefresh() {
  try {
    const res = await apiClient.getHostCommandHistory(hostId)
    cmdHistory.value = res.data?.commands || []
  } catch {
    cmdHistory.value = []
  }
}

async function deleteHost() {
  const confirmed = await dialog.confirm({
    title: "Supprimer l'hôte",
    message: "Cette action est irréversible. Toutes les données associées seront supprimées.",
    variant: 'danger',
    requiredText: host.value?.hostname || host.value?.name,
  })

  if (!confirmed) return

  try {
    await apiClient.deleteHost(hostId)
    router.push('/')
  } catch (e) {
    await dialog.confirm({
      title: 'Erreur',
      message: e.response?.data?.error || e.message,
      variant: 'danger',
    })
  }
}

onMounted(() => {
  loadComplete()
})
</script>

<style scoped>
.host-detail-page {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 100px);
}

.host-layout {
  display: flex;
  flex: 1;
  gap: 1rem;
  overflow: hidden;
  min-height: 0;
}

.host-panel-main {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  min-width: 0;
}

.host-panel-right {
  width: 38%;
  min-width: 380px;
  display: flex;
  flex-direction: column;
  transition: all 0.3s ease-in-out;
  overflow: hidden;
}

@media (max-width: 991px) {
  .host-detail-page {
    height: auto;
  }

  .host-layout {
    flex-direction: column;
    overflow: visible;
    height: auto;
  }

  .host-panel-main {
    overflow-y: visible;
  }

  .host-panel-right {
    width: 100%;
    min-width: 0;
    max-height: 70vh;
  }
}
</style>
