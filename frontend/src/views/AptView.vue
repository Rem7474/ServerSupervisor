<template>
  <div class="apt-page">
    <div class="page-header mb-3">
      <h2 class="page-title">APT — Mises à jour système</h2>
      <div class="text-secondary">Gérer les mises à jour APT sur tous les hôtes</div>
    </div>

    <WsStatusBar :status="wsStatus" :error="wsError" :retry-count="retryCount" @reconnect="reconnect" />

    <!-- Onglets -->
    <div class="mb-3">
      <div class="btn-group">
        <button
          class="btn"
          :class="activeTab === 'hosts' ? 'btn-primary' : 'btn-outline-secondary'"
          @click="activeTab = 'hosts'"
        >
          Hôtes
        </button>
        <button
          class="btn"
          :class="activeTab === 'history' ? 'btn-primary' : 'btn-outline-secondary'"
          @click="activeTab = 'history'"
        >
          Historique
          <span v-if="allHistory.length" class="badge bg-secondary ms-1">{{ allHistory.length }}</span>
        </button>
      </div>
    </div>

    <!-- === Vue Hôtes === -->
    <div v-if="activeTab === 'hosts'" class="apt-layout">
      <!-- Colonne gauche: Liste des hôtes -->
      <div class="apt-hosts">
        <div class="card mb-3">
          <div class="card-body">
            <div class="d-flex flex-wrap align-items-center gap-3">
              <label class="form-check">
                <input type="checkbox" class="form-check-input" v-model="selectAll" @change="toggleSelectAll" />
                <span class="form-check-label">Sélectionner tous les hôtes</span>
              </label>
              <div class="ms-auto d-flex flex-wrap gap-2">
                <template v-if="canRunApt">
                  <button @click="bulkAptCmd('update')" class="btn btn-outline-secondary" :disabled="selectedHosts.length === 0">
                    apt update ({{ selectedHosts.length }})
                  </button>
                  <button @click="bulkAptCmd('upgrade')" class="btn btn-primary" :disabled="selectedHosts.length === 0">
                    apt upgrade ({{ selectedHosts.length }})
                  </button>
                  <button @click="bulkAptCmd('dist-upgrade')" class="btn btn-outline-danger" :disabled="selectedHosts.length === 0">
                    apt dist-upgrade ({{ selectedHosts.length }})
                  </button>
                </template>
                <div v-else class="text-secondary small">Mode lecture seule</div>
              </div>
            </div>
          </div>
        </div>

        <div class="row row-cards">
          <div v-for="host in hosts" :key="host.id" class="col-12">
            <div class="card">
              <div class="card-body">
                <div class="d-flex align-items-center gap-3 mb-3">
                  <label class="form-check">
                    <input type="checkbox" class="form-check-input" :value="host.id" v-model="selectedHosts" />
                    <span class="form-check-label"></span>
                  </label>
                  <div class="flex-fill">
                    <div class="fw-semibold">{{ host.hostname || host.name }}</div>
                    <div class="text-secondary small">{{ host.ip_address }}</div>
                  </div>
                  <span :class="host.status === 'online' ? 'badge bg-green-lt text-green' : 'badge bg-red-lt text-red'">
                    {{ host.status === 'online' ? 'En ligne' : 'Hors ligne' }}
                  </span>
                </div>

                <div v-if="aptStatuses[host.id]" class="row row-cards mb-3">
                  <div class="col-sm-6 col-md-3">
                    <div class="card card-sm">
                      <div class="card-body text-center">
                        <div class="h2 mb-0" :class="aptStatuses[host.id].pending_packages > 0 ? 'text-yellow' : 'text-green'">
                          {{ aptStatuses[host.id].pending_packages }}
                        </div>
                        <div class="text-secondary small">En attente</div>
                      </div>
                    </div>
                  </div>
                  <div class="col-sm-6 col-md-3">
                    <div class="card card-sm">
                      <div class="card-body text-center">
                        <div class="h2 mb-0 text-red">{{ aptStatuses[host.id].security_updates }}</div>
                        <div class="text-secondary small">Sécurité</div>
                      </div>
                    </div>
                  </div>
                  <div class="col-sm-6 col-md-3">
                    <div class="card card-sm">
                      <div class="card-body text-center">
                        <div class="fw-semibold">{{ formatDate(aptStatuses[host.id].last_update) }}</div>
                        <div class="text-secondary small">Dernier update</div>
                      </div>
                    </div>
                  </div>
                  <div class="col-sm-6 col-md-3">
                    <div class="card card-sm">
                      <div class="card-body text-center">
                        <div class="fw-semibold">{{ formatDate(aptStatuses[host.id].last_upgrade) }}</div>
                        <div class="text-secondary small">Dernière mise à jour</div>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- CVE Information -->
                <div v-if="aptStatuses[host.id]?.cve_list" class="mb-3">
                  <CVEList
                    :cveList="aptStatuses[host.id].cve_list"
                    :showMaxSeverity="true"
                    :alwaysExpanded="false"
                    :limit="5"
                  />
                </div>

                <!-- Package List -->
                <div v-if="getPackages(aptStatuses[host.id]).length > 0" class="mb-3">
                  <div class="d-flex align-items-center mb-2">
                    <span class="fw-semibold me-2">Paquets en attente :</span>
                    <span class="badge bg-yellow-lt text-yellow">{{ getPackages(aptStatuses[host.id]).length }}</span>
                  </div>
                  <div v-if="packagesExpanded[host.id]" class="d-flex flex-wrap gap-1 mb-1">
                    <span
                      v-for="pkg in (packagesShowAll[host.id] ? getPackages(aptStatuses[host.id]) : getPackages(aptStatuses[host.id]).slice(0, 12))"
                      :key="pkg"
                      class="badge bg-blue-lt text-blue"
                      style="font-family: monospace; font-size: 0.72rem;"
                    >{{ pkg }}</span>
                    <button
                      v-if="getPackages(aptStatuses[host.id]).length > 12 && !packagesShowAll[host.id]"
                      @click="packagesShowAll[host.id] = true"
                      class="btn btn-sm btn-link p-0 ms-1"
                    >+{{ getPackages(aptStatuses[host.id]).length - 12 }} plus...</button>
                  </div>
                  <button
                    @click="packagesExpanded[host.id] = !packagesExpanded[host.id]"
                    class="btn btn-sm btn-link p-0"
                  >
                    {{ packagesExpanded[host.id]
                      ? 'Masquer'
                      : `Afficher ${getPackages(aptStatuses[host.id]).length} paquet${getPackages(aptStatuses[host.id]).length > 1 ? 's' : ''}` }}
                  </button>
                </div>

                <div v-if="aptHistories[host.id]?.length">
                  <button @click="toggleHistory(host.id)" class="btn btn-link p-0">
                    {{ expandedHistories[host.id] ? 'Masquer' : 'Voir' }} l'historique ({{ aptHistories[host.id].length }})
                  </button>
                  <div v-if="expandedHistories[host.id]" class="mt-3">
                    <div v-for="cmd in aptHistories[host.id]" :key="cmd.id" class="border rounded p-3 mb-2">
                      <div class="d-flex align-items-center justify-content-between">
                        <div class="fw-semibold">apt {{ cmd.command }}</div>
                        <div class="d-flex align-items-center gap-2">
                          <span :class="statusClass(cmd.status)">{{ cmd.status }}</span>
                          <span class="text-secondary small">{{ formatDuration(cmd.started_at, cmd.ended_at) }}</span>
                          <button @click="watchCommand(cmd, host)" class="btn btn-sm btn-outline-primary">
                            Voir les logs
                          </button>
                        </div>
                      </div>
                      <div class="text-secondary small mt-1">
                        {{ formatDate(cmd.created_at) }}
                        <span v-if="cmd.triggered_by">• par {{ cmd.triggered_by }}</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Colonne droite: Console Live -->
      <div class="apt-console" :class="{ 'apt-console--active': liveCommand }" id="apt-console-mobile">
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
            <button v-if="liveCommand" @click="closeLiveConsole" class="btn btn-sm btn-ghost-secondary" title="Fermer la console">
              <svg xmlns="http://www.w3.org/2000/svg" class="icon" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                <path d="M18 6l-12 12" />
                <path d="M6 6l12 12" />
              </svg>
            </button>
          </div>
          <div class="card-body d-flex flex-column" style="flex: 1; min-height: 0; padding: 0;">
            <div v-if="!liveCommand" class="d-flex align-items-center justify-content-center flex-fill text-secondary" style="background: #1e293b; border-radius: 0 0 0.5rem 0.5rem;">
              <div class="text-center p-4">
                <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler mb-2" width="48" height="48" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round" style="opacity: 0.5;">
                  <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                  <path d="M8 9l3 3l-3 3" />
                  <path d="M13 15l3 0" />
                  <path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z" />
                </svg>
                <div style="opacity: 0.7;">Aucune console active</div>
                <div class="small mt-1" style="opacity: 0.5;">Cliquez sur "Voir les logs" pour afficher la sortie d'une commande</div>
              </div>
            </div>

            <div v-else style="display: flex; flex-direction: column; height: 100%;">
              <div class="px-3 pt-3 pb-2" style="background: #1e293b; border-bottom: 1px solid rgba(255,255,255,0.1);">
                <div class="d-flex align-items-start justify-content-between mb-2">
                  <div class="flex-fill" style="min-width: 0;">
                    <div class="fw-semibold text-light" style="font-size: 0.95rem;">{{ liveCommand.hostname }}</div>
                    <div class="text-secondary small mt-1">
                      <code style="background: rgba(0,0,0,0.3); padding: 0.15rem 0.4rem; border-radius: 0.25rem; color: #94a3b8;">apt {{ liveCommand.command }}</code>
                    </div>
                  </div>
                  <span :class="statusClass(liveCommand.status)" style="margin-left: 0.5rem;">{{ liveCommand.status }}</span>
                </div>
              </div>
              <pre
                ref="consoleOutput"
                class="mb-0 flex-fill"
                style="
                  background: #0f172a;
                  color: #e2e8f0;
                  padding: 1rem;
                  margin: 0;
                  overflow-y: auto;
                  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
                  font-size: 0.813rem;
                  line-height: 1.5;
                  border-radius: 0 0 0.5rem 0.5rem;
                "
              >{{ renderedConsoleOutput || 'En attente de sortie...' }}</pre>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- === Vue Historique global === -->
    <div v-else class="card">
      <div class="card-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3">
        <h3 class="card-title mb-0">Historique des mises à jour</h3>
        <div class="d-flex flex-wrap gap-2">
          <!-- Filtre hôte -->
          <select v-model="historyHostFilter" class="form-select form-select-sm" style="min-width: 160px;">
            <option value="all">Tous les hôtes</option>
            <option v-for="host in hosts" :key="host.id" :value="host.id">
              {{ host.hostname || host.name }}
            </option>
          </select>
          <!-- Filtre période -->
          <div class="btn-group btn-group-sm">
            <button
              v-for="p in periodOptions"
              :key="p.value"
              class="btn"
              :class="historyPeriod === p.value ? 'btn-primary' : 'btn-outline-secondary'"
              @click="historyPeriod = p.value"
            >
              {{ p.label }}
            </button>
          </div>
        </div>
      </div>
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Date</th>
              <th>Hôte</th>
              <th>Commande</th>
              <th>Statut</th>
              <th>Utilisateur</th>
              <th>Durée</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="filteredHistory.length === 0">
              <td colspan="7" class="text-center text-secondary py-4">Aucun historique pour cette période</td>
            </tr>
            <tr v-for="cmd in filteredHistory" :key="cmd.id">
              <td class="text-secondary small">{{ formatDateExact(cmd.created_at) }}</td>
              <td>
                <div class="fw-semibold">{{ cmd.hostName }}</div>
              </td>
              <td><code>apt {{ cmd.command }}</code></td>
              <td><span :class="statusClass(cmd.status)">{{ cmd.status }}</span></td>
              <td class="text-secondary">{{ cmd.triggered_by || '—' }}</td>
              <td class="text-secondary small">{{ formatDuration(cmd.started_at, cmd.ended_at) }}</td>
              <td>
                <button
                  @click="watchCommand(cmd, { hostname: cmd.hostName, id: cmd.hostId })"
                  class="btn btn-sm btn-outline-primary"
                >
                  Voir les logs
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Console flottante pour l'onglet Historique -->
    <div v-if="activeTab === 'history' && liveCommand" class="card mt-3">
      <div class="card-header d-flex align-items-center justify-content-between" style="background: #1e293b; border-color: rgba(255,255,255,0.1);">
        <div class="d-flex align-items-center gap-2">
          <code style="color: #94a3b8;">apt {{ liveCommand.command }}</code>
          <span class="text-secondary small">— {{ liveCommand.hostname }}</span>
          <span :class="statusClass(liveCommand.status)">{{ liveCommand.status }}</span>
        </div>
        <button @click="closeLiveConsole" class="btn btn-sm btn-ghost-secondary">✕</button>
      </div>
      <pre
        ref="consoleOutputHistory"
        style="
          background: #0f172a;
          color: #e2e8f0;
          padding: 1rem;
          margin: 0;
          max-height: 400px;
          overflow-y: auto;
          font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
          font-size: 0.813rem;
          line-height: 1.5;
          border-radius: 0 0 0.5rem 0.5rem;
        "
      >{{ renderedConsoleOutput || 'En attente de sortie...' }}</pre>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed, nextTick } from 'vue'
import CVEList from '../components/CVEList.vue'
import apiClient from '../api'
import { useAuthStore } from '../stores/auth'
import { useWebSocket } from '../composables/useWebSocket'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import WsStatusBar from '../components/WsStatusBar.vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import utc from 'dayjs/plugin/utc'
import 'dayjs/locale/fr'

dayjs.extend(relativeTime)
dayjs.extend(utc)
dayjs.locale('fr')

// ── Tab ──────────────────────────────────────────────────────────────────────
const activeTab = ref('hosts')

// ── Hosts / APT state ────────────────────────────────────────────────────────
const hosts = ref([])
const selectedHosts = ref([])
const selectAll = ref(false)
const aptStatuses = ref({})
const aptHistories = ref({})
const expandedHistories = ref({})
const packagesExpanded = ref({})
const packagesShowAll = ref({})
const auth = useAuthStore()
const dialog = useConfirmDialog()
const canRunApt = computed(() => auth.role === 'admin' || auth.role === 'operator')

// ── Console ───────────────────────────────────────────────────────────────────
const renderedConsoleOutput = computed(() => {
  if (!liveCommand.value) return ''
  return renderConsoleOutput(liveCommand.value.output || '')
})
const liveCommand = ref(null)
const consoleOutput = ref(null)
const consoleOutputHistory = ref(null)
let streamWs = null

// ── Historique filters ────────────────────────────────────────────────────────
const historyHostFilter = ref('all')
const historyPeriod = ref('7d')

const periodOptions = [
  { label: '7j',  value: '7d'  },
  { label: '30j', value: '30d' },
  { label: '90j', value: '90d' },
  { label: 'Tout', value: 'all' },
]

// Flatten all histories into a single array, enriched with host info
const allHistory = computed(() => {
  return Object.entries(aptHistories.value).flatMap(([hostId, cmds]) => {
    const host = hosts.value.find(h => h.id === hostId)
    const hostName = host?.hostname || host?.name || hostId
    return (cmds || []).map(cmd => ({ ...cmd, hostId, hostName }))
  }).sort((a, b) => new Date(b.created_at) - new Date(a.created_at))
})

const filteredHistory = computed(() => {
  let list = allHistory.value

  // Filter by host
  if (historyHostFilter.value !== 'all') {
    list = list.filter(cmd => cmd.hostId === historyHostFilter.value)
  }

  // Filter by period
  if (historyPeriod.value !== 'all') {
    const days = parseInt(historyPeriod.value)
    const cutoff = dayjs().subtract(days, 'day')
    list = list.filter(cmd => dayjs(cmd.created_at).isAfter(cutoff))
  }

  return list
})

// ── Helpers ───────────────────────────────────────────────────────────────────
function toggleSelectAll() {
  if (selectAll.value) {
    selectedHosts.value = hosts.value.map(h => h.id)
  } else {
    selectedHosts.value = []
  }
}

function toggleHistory(hostId) {
  expandedHistories.value[hostId] = !expandedHistories.value[hostId]
}

function getPackages(aptStatus) {
  if (!aptStatus?.package_list) return []
  try {
    const parsed = typeof aptStatus.package_list === 'string'
      ? JSON.parse(aptStatus.package_list)
      : aptStatus.package_list
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function watchCommand(cmd, host) {
  liveCommand.value = {
    id: cmd.id,
    command: cmd.command,
    status: cmd.status,
    hostname: host?.hostname || host?.name || '—',
    output: cmd.output || '',
  }
  connectStreamWebSocket(cmd.id)
  nextTick(() => scrollToBottom())
}

function closeLiveConsole() {
  if (streamWs) {
    streamWs.close()
    streamWs = null
  }
  liveCommand.value = null
}

function scrollToBottom() {
  const el = consoleOutput.value || consoleOutputHistory.value
  if (el) el.scrollTop = el.scrollHeight
}

function connectStreamWebSocket(commandId) {
  if (streamWs) streamWs.close()
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const wsUrl = `${protocol}://${window.location.host}/api/v1/ws/apt/stream/${commandId}`
  streamWs = new WebSocket(wsUrl)

  streamWs.onopen = () => {
    streamWs.send(JSON.stringify({ type: 'auth', token: auth.token }))
  }

  streamWs.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data)
      if (payload.type === 'apt_stream_init') {
        liveCommand.value.status = payload.status
        liveCommand.value.output = payload.output || ''
        nextTick(() => scrollToBottom())
      } else if (payload.type === 'apt_stream') {
        liveCommand.value.output += payload.chunk
        nextTick(() => scrollToBottom())
      } else if (payload.type === 'apt_status_update') {
        liveCommand.value.status = payload.status
      }
    } catch {
      // Ignore malformed payloads
    }
  }

  streamWs.onclose = () => {}
}

async function bulkAptCmd(command) {
  const hostnames = hosts.value.filter(h => selectedHosts.value.includes(h.id)).map(h => h.hostname).join(', ')

  const isDangerous = command === 'dist-upgrade'
  const confirmed = await dialog.confirm({
    title: `apt ${command}`,
    message: isDangerous
      ? `⚠️ apt dist-upgrade peut supprimer des paquets existants.\nExécuter sur : ${hostnames} ?`
      : `Exécuter sur : ${hostnames} ?`,
    variant: isDangerous ? 'danger' : 'warning'
  })

  if (!confirmed) return

  try {
    const response = await apiClient.sendAptCommand(selectedHosts.value, command)
    if (selectedHosts.value.length === 1 && response.data?.commands?.length > 0) {
      const cmd = response.data.commands[0]
      const host = hosts.value.find(h => h.id === selectedHosts.value[0])
      if (cmd.command_id && host) {
        watchCommand({ id: cmd.command_id, command, status: 'pending', output: '' }, host)
      }
    }
  } catch (e) {
    await dialog.confirm({
      title: 'Erreur',
      message: e.response?.data?.error || e.message,
      variant: 'danger'
    })
  }
}

function formatDate(date) {
  if (!date || date === '0001-01-01T00:00:00Z') return 'Jamais'
  return dayjs.utc(date).local().fromNow()
}

function formatDateExact(date) {
  if (!date || date === '0001-01-01T00:00:00Z') return '—'
  return dayjs.utc(date).local().format('DD/MM/YYYY HH:mm')
}

function formatDuration(startedAt, endedAt) {
  if (!startedAt || !endedAt) return '—'
  const start = dayjs(startedAt)
  const end = dayjs(endedAt)
  if (!start.isValid() || !end.isValid()) return '—'
  const totalSeconds = end.diff(start, 'second')
  if (totalSeconds < 0) return '—'
  if (totalSeconds < 60) return `${totalSeconds}s`
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  return seconds > 0 ? `${minutes}m ${seconds}s` : `${minutes}m`
}

function statusClass(status) {
  if (status === 'completed') return 'badge bg-green-lt text-green'
  if (status === 'failed') return 'badge bg-red-lt text-red'
  return 'badge bg-yellow-lt text-yellow'
}

function renderConsoleOutput(raw) {
  if (!raw) return ''
  const lines = ['']
  let currentLine = ''

  for (let i = 0; i < raw.length; i++) {
    const ch = raw[i]
    if (ch === '\r') {
      currentLine = ''
      lines[lines.length - 1] = ''
      continue
    }
    if (ch === '\n') {
      currentLine = ''
      lines.push('')
      continue
    }
    currentLine += ch
    lines[lines.length - 1] = currentLine
  }

  return lines.join('\n')
}

const { wsStatus, wsError, retryCount, reconnect } = useWebSocket('/api/v1/ws/apt', (payload) => {
  if (payload.type !== 'apt') return
  hosts.value = payload.hosts || []
  aptStatuses.value = payload.apt_statuses || {}
  aptHistories.value = payload.apt_histories || {}
})

onMounted(() => {})

onUnmounted(() => {
  if (streamWs) streamWs.close()
})
</script>

<style scoped>
.apt-page {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 120px);
}

.apt-layout {
  display: flex;
  flex: 1;
  gap: 1rem;
  overflow: hidden;
  min-height: 0;
}

.apt-hosts {
  flex: 1;
  overflow-y: auto;
  min-width: 0;
}

.apt-console {
  width: 38%;
  min-width: 380px;
  display: flex;
  flex-direction: column;
}

@media (max-width: 991px) {
  .apt-page {
    height: auto;
  }

  .apt-layout {
    flex-direction: column;
    overflow: visible;
    height: auto;
  }

  .apt-hosts {
    overflow-y: visible;
  }

  .apt-console {
    width: 100%;
    min-width: 0;
    max-height: 60vh;
    display: none;
  }

  .apt-console--active {
    display: flex;
  }
}
</style>
