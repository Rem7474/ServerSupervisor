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

    <!-- Proxmox link panel -->
    <div v-if="proxmoxLink && proxmoxLink.status !== 'ignored'" class="card mb-3 border-0 shadow-sm">
      <div class="card-body py-2 px-3 d-flex flex-wrap align-items-center gap-3">
        <!-- Guest info -->
        <div class="d-flex align-items-center gap-2">
          <span class="badge bg-purple-lt text-purple">Proxmox</span>
          <span class="fw-medium">{{ proxmoxLink.guest_name || `VMID ${proxmoxLink.vmid}` }}</span>
          <span class="text-muted small">({{ proxmoxLink.guest_type?.toUpperCase() }} · {{ proxmoxLink.node_name }})</span>
        </div>

        <!-- Status badge + suggestion actions -->
        <div class="d-flex align-items-center gap-2">
          <span v-if="proxmoxLink.status === 'suggested'" class="badge bg-warning-lt text-warning">Suggestion</span>
          <span v-else class="badge bg-success-lt text-success">Lié</span>
          <template v-if="proxmoxLink.status === 'suggested'">
            <button class="btn btn-sm btn-success" :disabled="linkSaving" @click="confirmLink">Confirmer</button>
            <button class="btn btn-sm btn-outline-secondary" :disabled="linkSaving" @click="ignoreLink">Ignorer</button>
          </template>
        </div>

        <!-- Metrics source selector (shown only when confirmed) -->
        <div v-if="proxmoxLink.status === 'confirmed'" class="d-flex align-items-center gap-2 ms-auto">
          <label class="form-label mb-0 text-muted small">Source métriques :</label>
          <select class="form-select form-select-sm" style="width:auto" :value="proxmoxLink.metrics_source" @change="changeMetricsSource($event.target.value)">
            <option value="auto">Automatique</option>
            <option value="agent">Agent</option>
            <option value="proxmox">Proxmox</option>
          </select>
          <button class="btn btn-sm btn-outline-danger" :disabled="linkSaving" @click="deleteLink" title="Supprimer le lien">
            <svg class="icon icon-sm" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 01-2 2H8a2 2 0 01-2-2L5 6"/>
              <path d="M10 11v6M14 11v6M9 6V4h6v2"/>
            </svg>
          </button>
        </div>

        <!-- Guest live metrics (source = proxmox) -->
        <template v-if="proxmoxLink.status === 'confirmed' && proxmoxLink.metrics_source !== 'agent'">
          <div class="d-flex align-items-center gap-3 ms-2 border-start ps-3">
            <div class="text-muted small">
              CPU <strong class="text-body">{{ ((proxmoxLink.cpu_usage ?? 0) * 100).toFixed(1) }}%</strong>
            </div>
            <div class="text-muted small">
              RAM <strong class="text-body">{{ formatBytesLink(proxmoxLink.mem_usage) }}</strong> / {{ formatBytesLink(proxmoxLink.mem_alloc) }}
            </div>
          </div>
        </template>
      </div>
    </div>

    <!-- No link banner + manual link button -->
    <div v-else-if="!proxmoxLink && showLinkButton" class="d-flex align-items-center gap-2 mb-3">
      <button class="btn btn-sm btn-outline-purple" @click="openLinkForm">
        <svg class="icon icon-sm me-1" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/>
          <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>
        </svg>
        Lier à Proxmox
      </button>
    </div>

    <!-- Manual link form -->
    <div v-if="showLinkForm" class="card mb-3">
      <div class="card-body">
        <div class="fw-medium mb-2">Lier cet hôte à un guest Proxmox</div>
        <div v-if="linkCandidatesLoading" class="text-muted small">Chargement...</div>
        <div v-else-if="linkCandidates.length === 0" class="text-muted small">Aucun guest Proxmox disponible (non encore lié).</div>
        <div v-else class="d-flex align-items-center gap-2">
          <select v-model="selectedCandidate" class="form-select form-select-sm" style="max-width:320px">
            <option value="">-- Choisir un guest --</option>
            <option v-for="g in linkCandidates" :key="g.id" :value="g.id">
              {{ g.name || `VMID ${g.vmid}` }} ({{ g.guest_type?.toUpperCase() }} · {{ g.node_name }})
            </option>
          </select>
          <button class="btn btn-sm btn-primary" :disabled="!selectedCandidate || linkSaving" @click="createManualLink">Lier</button>
          <button class="btn btn-sm btn-outline-secondary" @click="showLinkForm = false; selectedCandidate = ''">Annuler</button>
        </div>
      </div>
    </div>

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
          <HostMetricsPanel :hostId="hostId" :metrics="effectiveMetrics" :metricsSource="effectiveMetricsSource" :proxmoxGuestId="proxmoxLink?.guest_id ?? null" />
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

// Proxmox link state
const proxmoxLink = ref(null)
const linkSaving = ref(false)

// Effective metrics — substitutes Proxmox CPU/RAM when metrics_source demands it.
// proxmoxLink.cpu_usage is a 0–1 fraction; mem_usage / mem_alloc are bytes.
const effectiveMetrics = computed(() => {
  const m = metrics.value
  const link = proxmoxLink.value
  if (!m || !link || link.status !== 'confirmed') return m

  const src = link.metrics_source ?? 'auto'
  const useProxmox =
    src === 'proxmox' ||
    (src === 'auto' && (link.mem_alloc ?? 0) > 0)

  if (!useProxmox) return m

  const cpuPct = (link.cpu_usage ?? 0) * 100
  const memUsed = link.mem_usage ?? 0
  const memTotal = link.mem_alloc ?? 0
  return {
    ...m,
    cpu_usage_percent: cpuPct,
    memory_used: memUsed,
    memory_total: memTotal,
    memory_percent: memTotal > 0 ? (memUsed / memTotal) * 100 : 0,
  }
})

const effectiveMetricsSource = computed(() => {
  const link = proxmoxLink.value
  if (!link || link.status !== 'confirmed') return 'agent'
  const src = link.metrics_source ?? 'auto'
  if (src === 'proxmox') return 'proxmox'
  if (src === 'auto' && (link.mem_alloc ?? 0) > 0) return 'proxmox'
  return 'agent'
})
const showLinkForm = ref(false)
const showLinkButton = ref(false)
const linkCandidates = ref([])
const linkCandidatesLoading = ref(false)
const selectedCandidate = ref('')

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
  if ('proxmox_link' in payload) {
    proxmoxLink.value = payload.proxmox_link
  }
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

// ─── Proxmox link helpers ─────────────────────────────────────────────────────

async function loadProxmoxLink() {
  try {
    const res = await apiClient.getHostProxmoxLink(hostId)
    proxmoxLink.value = res.data  // null when no link exists (server returns 200 null)
    if (!res.data) {
      // No link — show the button only if there are linkable candidates.
      const cands = await apiClient.getHostProxmoxCandidates(hostId).catch(() => ({ data: [] }))
      showLinkButton.value = (cands.data?.length ?? 0) > 0
    }
  } catch {
    proxmoxLink.value = null
    showLinkButton.value = false
  }
}

async function confirmLink() {
  if (!proxmoxLink.value) return
  linkSaving.value = true
  try {
    const res = await apiClient.updateProxmoxLink(proxmoxLink.value.id, { status: 'confirmed' })
    proxmoxLink.value = res.data
  } finally {
    linkSaving.value = false
  }
}

async function ignoreLink() {
  if (!proxmoxLink.value) return
  linkSaving.value = true
  try {
    await apiClient.updateProxmoxLink(proxmoxLink.value.id, { status: 'ignored' })
    proxmoxLink.value = null
    showLinkButton.value = true
  } finally {
    linkSaving.value = false
  }
}

async function changeMetricsSource(source) {
  if (!proxmoxLink.value) return
  linkSaving.value = true
  try {
    const res = await apiClient.updateProxmoxLink(proxmoxLink.value.id, { metrics_source: source })
    proxmoxLink.value = res.data
  } finally {
    linkSaving.value = false
  }
}

async function deleteLink() {
  if (!proxmoxLink.value) return
  linkSaving.value = true
  try {
    await apiClient.deleteProxmoxLink(proxmoxLink.value.id)
    proxmoxLink.value = null
    showLinkButton.value = true
  } finally {
    linkSaving.value = false
  }
}

async function openLinkForm() {
  showLinkForm.value = true
  if (linkCandidates.value.length > 0) return
  linkCandidatesLoading.value = true
  try {
    const res = await apiClient.getHostProxmoxCandidates(hostId)
    linkCandidates.value = res.data || []
  } finally {
    linkCandidatesLoading.value = false
  }
}

async function createManualLink() {
  if (!selectedCandidate.value) return
  linkSaving.value = true
  try {
    const res = await apiClient.createProxmoxLink({
      guest_id: selectedCandidate.value,
      host_id: hostId,
      status: 'confirmed',
      metrics_source: 'auto',
    })
    proxmoxLink.value = res.data
    showLinkForm.value = false
    showLinkButton.value = false
    selectedCandidate.value = ''
  } finally {
    linkSaving.value = false
  }
}

function formatBytesLink(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'Ko', 'Mo', 'Go', 'To']
  let i = 0, v = bytes
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

onMounted(() => {
  loadComplete()
  loadProxmoxLink()
})
</script>

<style scoped>
.host-layout {
  display: flex;
  gap: 1rem;
  align-items: flex-start;
}

.host-panel-main {
  flex: 1;
  min-width: 0;
}

:deep(.host-panel-right) {
  width: 40%;
  min-width: 380px;
  display: flex;
  flex-direction: column;
  height: calc(100vh - 160px);
  position: sticky;
  top: 1rem;
  transition: width 0.3s ease-in-out;
  overflow: hidden;
}

@media (max-width: 991px) {
  .host-layout {
    flex-direction: column;
    align-items: stretch;
  }

  :deep(.host-panel-right) {
    width: 100%;
    min-width: 0;
    height: 60vh;
    position: static;
  }
}
</style>
