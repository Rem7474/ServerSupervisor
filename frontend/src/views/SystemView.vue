<template>
  <div>
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <div class="text-secondary small">
            <router-link to="/" class="text-decoration-none">Dashboard</router-link>
            <span class="mx-1">/</span>
            <router-link :to="`/hosts/${hostId}`" class="text-decoration-none">Hôte</router-link>
            <span class="mx-1">/</span>
            <span>Système</span>
          </div>
          <h2 class="page-title">Gestion système</h2>
        </div>
        <div>
          <router-link :to="`/hosts/${hostId}`" class="btn btn-outline-secondary">
            Retour à l'hôte
          </router-link>
        </div>
      </div>
    </div>

    <div v-if="!canManage" class="alert alert-warning">
      Vous n'avez pas les permissions nécessaires pour gérer les services système.
    </div>

    <template v-else>
      <!-- Services Systemd -->
      <div class="card mb-3">
        <div class="card-header d-flex align-items-center justify-content-between">
          <h3 class="card-title">Services systemd</h3>
          <div class="d-flex align-items-center gap-2">
            <div class="btn-group btn-group-sm">
              <button
                :class="systemdFilter === 'active' ? 'btn btn-primary' : 'btn btn-outline-secondary'"
                @click="systemdFilter = 'active'"
              >Actifs</button>
              <button
                :class="systemdFilter === 'all' ? 'btn btn-primary' : 'btn btn-outline-secondary'"
                @click="systemdFilter = 'all'"
              >Tous</button>
            </div>
            <button
              class="btn btn-sm btn-outline-secondary"
              @click="loadSystemdServices"
              :disabled="systemdLoading"
            >
              <span v-if="systemdLoading" class="spinner-border spinner-border-sm me-1"></span>
              {{ systemdLoading ? 'Chargement...' : 'Charger les services' }}
            </button>
          </div>
        </div>
        <div v-if="systemdError" class="card-body pb-0">
          <div class="alert alert-danger mb-0">{{ systemdError }}</div>
        </div>
        <div v-if="!systemdServices.length && !systemdLoading && !systemdError" class="card-body">
          <div class="text-secondary small">Cliquez sur "Charger les services" pour afficher les services systemd.</div>
        </div>
        <div v-if="filteredSystemdServices.length" class="table-responsive">
          <table class="table table-vcenter table-hover card-table mb-0">
            <thead>
              <tr>
                <th>Service</th>
                <th>État</th>
                <th>Mode</th>
                <th>Description</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="svc in filteredSystemdServices" :key="svc.name">
                <td class="font-monospace small">{{ svc.name }}</td>
                <td>
                  <span :class="systemdStateClass(svc.active_state)">{{ svc.active_state }}</span>
                </td>
                <td class="text-secondary small">{{ svc.sub_state }}</td>
                <td
                  class="text-secondary small"
                  style="max-width: 250px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;"
                  :title="svc.description"
                >{{ svc.description || '—' }}</td>
                <td class="text-nowrap">
                  <div class="btn-group btn-group-sm">
                    <button
                      v-if="svc.active_state !== 'active'"
                      class="btn btn-outline-success"
                      @click="executeSystemdAction(svc.name, 'start')"
                    >Start</button>
                    <button
                      v-if="svc.active_state === 'active'"
                      class="btn btn-outline-danger"
                      @click="executeSystemdAction(svc.name, 'stop')"
                    >Stop</button>
                    <button
                      class="btn btn-outline-secondary"
                      @click="executeSystemdAction(svc.name, 'restart')"
                    >Restart</button>
                    <button
                      class="btn btn-outline-secondary"
                      @click="streamJournal(svc.name)"
                      title="Voir les logs"
                    >Logs</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Journalctl -->
      <div class="card mb-3">
        <div class="card-header">
          <h3 class="card-title">Logs système (journalctl)</h3>
        </div>
        <div class="card-body">
          <div class="d-flex gap-2">
            <input
              v-model="journalService"
              type="text"
              class="form-control"
              placeholder="Nom du service (ex: nginx, sshd)"
              @keyup.enter="streamJournal(journalService)"
            />
            <button
              class="btn btn-outline-secondary"
              @click="streamJournal(journalService)"
              :disabled="!journalService.trim() || journalLoading"
            >
              <span v-if="journalLoading" class="spinner-border spinner-border-sm me-1"></span>
              {{ journalLoading ? 'Chargement...' : 'Charger les logs' }}
            </button>
          </div>
          <div v-if="journalError" class="alert alert-danger mt-2 mb-0">{{ journalError }}</div>
        </div>
      </div>

      <!-- Processus -->
      <div class="card mb-3">
        <div class="card-header d-flex align-items-center justify-content-between">
          <h3 class="card-title">Processus</h3>
          <div class="d-flex align-items-center gap-2">
            <input
              v-model="processFilter"
              type="text"
              class="form-control form-control-sm"
              placeholder="Filtrer..."
              style="width: 160px;"
            />
            <button
              class="btn btn-sm btn-outline-secondary"
              @click="loadProcesses"
              :disabled="processesLoading"
            >
              <span v-if="processesLoading" class="spinner-border spinner-border-sm me-1"></span>
              {{ processesLoading ? 'Chargement...' : (processes.length ? 'Actualiser' : 'Charger') }}
            </button>
          </div>
        </div>
        <div v-if="processesError" class="card-body pb-0">
          <div class="alert alert-danger mb-0">{{ processesError }}</div>
        </div>
        <div v-if="!processes.length && !processesLoading && !processesError" class="card-body">
          <div class="text-secondary small">Cliquez sur "Charger" pour afficher les processus actifs.</div>
        </div>
        <div v-if="filteredProcesses.length" class="table-responsive">
          <table class="table table-vcenter table-hover card-table mb-0" style="font-size: 0.82rem;">
            <thead>
              <tr>
                <th class="cursor-pointer" @click="sortProcesses('pid')">PID <span class="text-secondary">{{ sortIcon('pid') }}</span></th>
                <th class="cursor-pointer" @click="sortProcesses('name')">Nom <span class="text-secondary">{{ sortIcon('name') }}</span></th>
                <th>Utilisateur</th>
                <th class="cursor-pointer" @click="sortProcesses('cpu_pct')">CPU% <span class="text-secondary">{{ sortIcon('cpu_pct') }}</span></th>
                <th class="cursor-pointer" @click="sortProcesses('mem_pct')">MEM% <span class="text-secondary">{{ sortIcon('mem_pct') }}</span></th>
                <th class="cursor-pointer" @click="sortProcesses('mem_rss_kb')">RSS (KB) <span class="text-secondary">{{ sortIcon('mem_rss_kb') }}</span></th>
                <th>État</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="proc in filteredProcesses" :key="proc.pid">
                <td class="text-secondary font-monospace">{{ proc.pid }}</td>
                <td class="fw-semibold font-monospace">{{ proc.name }}</td>
                <td class="text-secondary">{{ proc.user }}</td>
                <td>
                  <span :class="proc.cpu_pct > 50 ? 'text-danger fw-bold' : proc.cpu_pct > 10 ? 'text-warning' : ''">
                    {{ proc.cpu_pct.toFixed(1) }}%
                  </span>
                </td>
                <td>
                  <span :class="proc.mem_pct > 50 ? 'text-danger fw-bold' : proc.mem_pct > 20 ? 'text-warning' : ''">
                    {{ proc.mem_pct.toFixed(1) }}%
                  </span>
                </td>
                <td class="text-secondary">{{ proc.mem_rss_kb.toLocaleString() }}</td>
                <td>
                  <span
                    class="badge"
                    :class="proc.state.startsWith('S') || proc.state.startsWith('I') ? 'bg-secondary-lt text-secondary'
                      : proc.state.startsWith('R') ? 'bg-success-lt text-success'
                      : proc.state.startsWith('Z') ? 'bg-danger-lt text-danger'
                      : 'bg-yellow-lt text-yellow'"
                  >{{ proc.state }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="processes.length" class="card-footer text-secondary small">
          {{ filteredProcesses.length }} / {{ processes.length }} processus
        </div>
      </div>

      <!-- Console Live -->
      <div v-if="liveCommand" class="card mb-3">
        <div class="card-header d-flex align-items-center justify-content-between">
          <h3 class="card-title font-monospace small">
            <span class="text-secondary">$ </span>{{ liveCommand.label }}
          </h3>
          <div class="d-flex align-items-center gap-2">
            <span
              class="badge"
              :class="liveCommand.status === 'completed' ? 'bg-success' : liveCommand.status === 'failed' ? 'bg-danger' : 'bg-yellow text-yellow-fg'"
            >{{ liveCommand.status }}</span>
            <button class="btn btn-sm btn-outline-secondary" @click="closeConsole">Fermer</button>
          </div>
        </div>
        <div
          ref="consoleEl"
          class="card-body font-monospace small"
          style="background:#1e1e2e; color:#cdd6f4; max-height:400px; overflow-y:auto; white-space:pre-wrap; word-break:break-all;"
        >{{ liveCommand.output || '(en attente de sortie...)' }}</div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onUnmounted, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import apiClient from '../api/index.js'

const route = useRoute()
const auth = useAuthStore()
const hostId = route.params.id

const canManage = computed(() => auth.role === 'admin' || auth.role === 'operator')

// ===== Systemd =====
const systemdServices = ref([])
const systemdLoading = ref(false)
const systemdError = ref('')
const systemdFilter = ref('active')

const filteredSystemdServices = computed(() => {
  if (systemdFilter.value === 'all') return systemdServices.value
  return systemdServices.value.filter(s => s.active_state === 'active')
})

function systemdStateClass(state) {
  if (state === 'active') return 'badge bg-success-lt text-success'
  if (state === 'failed') return 'badge bg-danger-lt text-danger'
  if (state === 'inactive') return 'badge bg-secondary-lt text-secondary'
  return 'badge bg-yellow-lt text-yellow'
}

async function loadSystemdServices() {
  systemdLoading.value = true
  systemdError.value = ''
  try {
    const res = await apiClient.sendSystemdCommand(hostId, '', 'list')
    const cmdId = res.data.command_id
    const output = await waitForCommandOutput(cmdId, 35000)
    try {
      systemdServices.value = JSON.parse(output)
    } catch {
      systemdError.value = 'Impossible de parser la liste des services'
    }
  } catch (e) {
    systemdError.value = e.message || e.response?.data?.error || 'Erreur'
  } finally {
    systemdLoading.value = false
  }
}

async function executeSystemdAction(serviceName, action) {
  try {
    const res = await apiClient.sendSystemdCommand(hostId, serviceName, action)
    const cmdId = res.data.command_id
    openConsole(`systemctl ${action} ${serviceName}`, cmdId)
  } catch (e) {
    alert(e.response?.data?.error || 'Impossible d\'envoyer la commande')
  }
}

// ===== Journal =====
const journalService = ref('')
const journalLoading = ref(false)
const journalError = ref('')

async function streamJournal(serviceName) {
  const svc = (serviceName || '').trim()
  if (!svc) return
  journalLoading.value = true
  journalError.value = ''
  journalService.value = svc
  try {
    const res = await apiClient.sendJournalCommand(hostId, svc)
    const cmdId = res.data.command_id
    openConsole(`journalctl -u ${svc}`, cmdId)
  } catch (e) {
    journalError.value = e.response?.data?.error || 'Impossible d\'envoyer la commande'
  } finally {
    journalLoading.value = false
  }
}

// ===== Processes =====
const processes = ref([])
const processesLoading = ref(false)
const processesError = ref('')
const processFilter = ref('')
const processSortKey = ref('cpu_pct')
const processSortDir = ref(-1)

const filteredProcesses = computed(() => {
  let list = processes.value
  if (processFilter.value) {
    const q = processFilter.value.toLowerCase()
    list = list.filter(p => p.name.toLowerCase().includes(q) || p.user.toLowerCase().includes(q))
  }
  return [...list].sort((a, b) => {
    const av = a[processSortKey.value]
    const bv = b[processSortKey.value]
    if (typeof av === 'string') return processSortDir.value * av.localeCompare(bv)
    return processSortDir.value * (bv - av)
  })
})

function sortProcesses(key) {
  if (processSortKey.value === key) {
    processSortDir.value *= -1
  } else {
    processSortKey.value = key
    processSortDir.value = key === 'name' || key === 'user' ? 1 : -1
  }
}

function sortIcon(key) {
  if (processSortKey.value !== key) return ''
  return processSortDir.value === -1 ? '▼' : '▲'
}

async function loadProcesses() {
  processesLoading.value = true
  processesError.value = ''
  try {
    const res = await apiClient.sendProcessesCommand(hostId)
    const cmdId = res.data.command_id
    const output = await waitForCommandOutput(cmdId, 20000)
    try {
      processes.value = JSON.parse(output)
    } catch {
      processesError.value = 'Impossible de parser la liste des processus'
    }
  } catch (e) {
    processesError.value = e.message || e.response?.data?.error || 'Erreur'
  } finally {
    processesLoading.value = false
  }
}

// ===== Live Console =====
const liveCommand = ref(null)
const consoleEl = ref(null)
let streamWs = null

function openConsole(label, cmdId) {
  closeConsole()
  liveCommand.value = { label, status: 'running', output: '' }
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const wsUrl = `${protocol}://${window.location.host}/api/v1/ws/commands/stream/${cmdId}`
  streamWs = new WebSocket(wsUrl)
  streamWs.onopen = () => { streamWs.send(JSON.stringify({ type: 'auth', token: auth.token })) }
  streamWs.onmessage = (event) => {
    try {
      const p = JSON.parse(event.data)
      if (p.type === 'cmd_stream_init') {
        if (p.output) liveCommand.value.output = p.output
        if (p.status === 'completed' || p.status === 'failed') {
          liveCommand.value.status = p.status
          streamWs.close()
        }
      } else if (p.type === 'cmd_stream') {
        liveCommand.value.output += p.chunk || ''
        nextTick(() => { if (consoleEl.value) consoleEl.value.scrollTop = consoleEl.value.scrollHeight })
      } else if (p.type === 'cmd_status_update') {
        if (p.output) liveCommand.value.output = p.output
        if (p.status) liveCommand.value.status = p.status
        if (p.status === 'completed' || p.status === 'failed') streamWs.close()
      }
    } catch { /* ignore */ }
  }
  streamWs.onerror = () => { if (liveCommand.value) liveCommand.value.status = 'failed' }
}

function closeConsole() {
  if (streamWs) { streamWs.onclose = null; streamWs.onmessage = null; streamWs.close(); streamWs = null }
  liveCommand.value = null
}

// Helper: wait for a command result via WebSocket (for fire-and-read commands)
function waitForCommandOutput(cmdId, timeoutMs) {
  return new Promise((resolve, reject) => {
    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const ws = new WebSocket(`${protocol}://${window.location.host}/api/v1/ws/commands/stream/${cmdId}`)
    let output = ''
    const timer = setTimeout(() => { ws.close(); reject(new Error('Timeout')) }, timeoutMs)
    ws.onopen = () => { ws.send(JSON.stringify({ type: 'auth', token: auth.token })) }
    ws.onmessage = (event) => {
      try {
        const p = JSON.parse(event.data)
        if (p.type === 'cmd_stream_init') {
          output = p.output || ''
          if (p.status === 'completed') { clearTimeout(timer); ws.close(); resolve(output) }
          else if (p.status === 'failed') { clearTimeout(timer); ws.close(); reject(new Error(output || 'Command failed')) }
        } else if (p.type === 'cmd_stream') {
          output += p.chunk || ''
        } else if (p.type === 'cmd_status_update') {
          if (p.output) output = p.output
          if (p.status === 'completed') { clearTimeout(timer); ws.close(); resolve(output) }
          else if (p.status === 'failed') { clearTimeout(timer); ws.close(); reject(new Error(output || 'Command failed')) }
        }
      } catch { /* ignore */ }
    }
    ws.onclose = () => { clearTimeout(timer); if (output) resolve(output) }
    ws.onerror = () => { clearTimeout(timer); reject(new Error('WebSocket error')) }
  })
}

onUnmounted(() => { if (streamWs) streamWs.close() })
</script>
