<template>
  <div class="d-flex flex-column" style="height: calc(100vh - 100px);">
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <div class="text-secondary small">
            <router-link to="/" class="text-decoration-none">Dashboard</router-link>
            <span class="mx-1">/</span>
            <span>Hote</span>
          </div>
          <h2 class="page-title">{{ host?.name || host?.hostname || 'Chargement...' }}</h2>
          <div class="text-secondary">
            {{ host?.hostname || 'Non connecte' }} — {{ host?.os || 'OS inconnu' }} • {{ host?.ip_address }}
            <span v-if="host?.last_seen">• Derniere activite: <RelativeTime :date="host.last_seen" /></span>
            <span v-if="host?.agent_version">• Agent v{{ host.agent_version }}</span>
          </div>
        </div>
        <div class="d-flex align-items-center gap-2">
          <router-link to="/" class="btn btn-outline-secondary">Retour au dashboard</router-link>
        <button @click="startEdit" class="btn btn-outline-secondary">Modifier</button>
        <button @click="deleteHost" class="btn btn-outline-danger">Supprimer</button>
        <span v-if="host?.agent_version" :class="isAgentUpToDate(host.agent_version) ? 'badge bg-green-lt text-green' : 'badge bg-yellow-lt text-yellow'">
          Agent v{{ host.agent_version }}
        </span>
        <span v-if="host" :class="host.status === 'online' ? 'badge bg-green-lt text-green' : 'badge bg-red-lt text-red'">
          {{ host.status === 'online' ? 'En ligne' : 'Hors ligne' }}
        </span>
        </div>
      </div>
    </div>

    <div class="d-flex flex-fill" style="gap: 1rem; overflow: hidden; min-height: 0;">
      <!-- Colonne gauche: Informations hôte -->
      <div style="flex: 1; overflow-y: auto; min-width: 0;">

    <div v-if="isEditing" class="card mb-4">
      <div class="card-header">
        <h3 class="card-title">Modifier l'hote</h3>
      </div>
      <div class="card-body">
        <form @submit.prevent="saveEdit" class="row g-3">
          <div class="col-md-6">
            <label class="form-label">Nom</label>
            <input v-model="editForm.name" type="text" class="form-control" required />
          </div>
          <div class="col-md-6">
            <label class="form-label">Hostname</label>
            <input v-model="editForm.hostname" type="text" class="form-control" required />
          </div>
          <div class="col-md-6">
            <label class="form-label">Adresse IP</label>
            <input v-model="editForm.ip_address" type="text" class="form-control" required />
          </div>
          <div class="col-md-6">
            <label class="form-label">OS</label>
            <input v-model="editForm.os" type="text" class="form-control" required />
          </div>
          <div class="col-12 d-flex justify-content-end gap-2">
            <button type="button" @click="cancelEdit" class="btn btn-outline-secondary" :disabled="saving">Annuler</button>
            <button type="submit" class="btn btn-primary" :disabled="saving">
              {{ saving ? 'Enregistrement...' : 'Enregistrer' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <div v-if="metrics" class="row row-cards mb-4">
      <div class="col-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="subheader">CPU ({{ metrics.cpu_cores }} cores)</div>
            <div class="h2 mb-0" :class="cpuColor(metrics.cpu_usage_percent)">
              {{ metrics.cpu_usage_percent?.toFixed(1) }}%
            </div>
            <div class="text-secondary small">{{ metrics.cpu_model }}</div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="subheader">RAM</div>
            <div class="h2 mb-0" :class="memColor(metrics.memory_percent)">
              {{ metrics.memory_percent?.toFixed(1) }}%
            </div>
            <div class="text-secondary small">{{ formatBytes(metrics.memory_used) }} / {{ formatBytes(metrics.memory_total) }}</div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="subheader">Uptime</div>
            <div class="h2 mb-0 text-primary">{{ formatUptime(metrics.uptime) }}</div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="subheader">Load Avg</div>
            <div class="h2 mb-0">{{ metrics.load_avg_1?.toFixed(2) }}</div>
            <div class="text-secondary small">{{ metrics.load_avg_5?.toFixed(2) }} / {{ metrics.load_avg_15?.toFixed(2) }}</div>
          </div>
        </div>
      </div>
    </div>

    <div class="row row-cards mb-4">
      <div class="col-lg-6">
        <div class="card">
          <div class="card-header d-flex align-items-center justify-content-between">
            <h3 class="card-title">CPU ({{ chartHours }}h)</h3>
            <div class="btn-group btn-group-sm">
              <button v-for="h in [1, 6, 24, 168, 720, 8760]" :key="h" @click="loadHistory(h)"
                :class="chartHours === h ? 'btn btn-primary' : 'btn btn-outline-secondary'">
                {{ h >= 24 ? (h / 24) + 'j' : h + 'h' }}
              </button>
            </div>
          </div>
          <div class="card-body" style="height: 12rem;">
            <Line v-if="cpuChartData" :data="cpuChartData" :options="chartOptions" class="h-100" />
            <div v-else class="h-100 d-flex align-items-center justify-content-center text-secondary">Aucune donnee</div>
          </div>
        </div>
      </div>
      <div class="col-lg-6">
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">Memoire ({{ chartHours }}h)</h3>
          </div>
          <div class="card-body" style="height: 12rem;">
            <Line v-if="memChartData" :data="memChartData" :options="chartOptions" class="h-100" />
            <div v-else class="h-100 d-flex align-items-center justify-content-center text-secondary">Aucune donnee</div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="metrics?.disks?.length" class="card mb-4">
      <div class="card-header">
        <h3 class="card-title">Disques</h3>
      </div>
      <div class="card-body">
        <div class="row row-cards">
          <div v-for="disk in metrics.disks" :key="disk.mount_point" class="col-md-6">
            <div class="border rounded p-3">
              <div class="d-flex justify-content-between mb-2">
                <div class="fw-semibold">{{ disk.mount_point }}</div>
                <div class="text-secondary small">{{ disk.device }} ({{ disk.fs_type }})</div>
              </div>
              <div class="progress mb-2">
                <div
                  class="progress-bar"
                  :class="disk.used_percent > 90 ? 'bg-red' : disk.used_percent > 75 ? 'bg-yellow' : 'bg-primary'"
                  :style="{ width: disk.used_percent + '%' }"
                ></div>
              </div>
              <div class="d-flex justify-content-between text-secondary small">
                <span>{{ formatBytes(disk.used_bytes) }} utilises</span>
                <span>{{ disk.used_percent?.toFixed(1) }}%</span>
                <span>{{ formatBytes(disk.total_bytes) }} total</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="containers.length" class="card mb-4">
      <div class="card-header">
        <h3 class="card-title">Conteneurs Docker ({{ containers.length }})</h3>
      </div>
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Nom</th>
              <th>Image</th>
              <th>Tag</th>
              <th>Etat</th>
              <th>Status</th>
              <th>Ports</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in containers" :key="c.id">
              <td class="fw-semibold">{{ c.name }}</td>
              <td class="text-secondary">{{ c.image }}</td>
              <td><code>{{ c.image_tag }}</code></td>
              <td>
                <span :class="c.state === 'running' ? 'badge bg-green-lt text-green' : 'badge bg-secondary-lt text-secondary'">
                  {{ c.state }}
                </span>
              </td>
              <td class="text-secondary small">{{ c.status }}</td>
              <td class="text-secondary small font-monospace">{{ c.ports || '-' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="aptStatus" class="card">
      <div class="card-header d-flex align-items-center justify-content-between">
        <h3 class="card-title">APT - Mises a jour systeme</h3>
        <div class="btn-group btn-group-sm" v-if="canRunApt">
          <button @click="sendAptCmd('update')" class="btn btn-outline-secondary">apt update</button>
          <button @click="sendAptCmd('upgrade')" class="btn btn-primary">apt upgrade</button>
          <button @click="sendAptCmd('dist-upgrade')" class="btn btn-outline-danger">apt dist-upgrade</button>
        </div>
        <span v-else class="text-secondary small">Mode lecture seule</span>
      </div>
      <div class="card-body">
        <div class="row row-cards">
          <div class="col-md-4">
            <div class="card card-sm">
              <div class="card-body text-center">
                <div class="h2 mb-0" :class="aptStatus.pending_packages > 0 ? 'text-yellow' : 'text-green'">
                  {{ aptStatus.pending_packages }}
                </div>
                <div class="text-secondary small">Paquets en attente</div>
              </div>
            </div>
          </div>
          <div class="col-md-4">
            <div class="card card-sm">
              <div class="card-body text-center">
                <div class="h2 mb-0 text-red">{{ aptStatus.security_updates }}</div>
                <div class="text-secondary small">Mises a jour securite</div>
              </div>
            </div>
          </div>
          <div class="col-md-4">
            <div class="card card-sm">
              <div class="card-body text-center">
                <div class="fw-semibold">{{ formatDate(aptStatus.last_update) }}</div>
                <div class="text-secondary small">Dernier apt update</div>
              </div>
            </div>
          </div>
        </div>
        
        <!-- CVE Information -->
        <div v-if="aptStatus.cve_list" class="mt-3">
          <CVEList 
            :cveList="aptStatus.cve_list" 
            :showMaxSeverity="true"
            :alwaysExpanded="false"
            :limit="5"
          />
        </div>
      </div>
    </div>

    <div v-if="aptHistory.length" class="card mt-4">
      <div class="card-header">
        <h3 class="card-title">Historique APT</h3>
      </div>
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Date</th>
              <th>Commande</th>
              <th>Statut</th>
              <th>Utilisateur</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="cmd in aptHistory" :key="cmd.id">
              <td>{{ formatDate(cmd.created_at) }}</td>
              <td><code>apt {{ cmd.command }}</code></td>
              <td>
                <span :class="statusClass(cmd.status)">{{ cmd.status }}</span>
              </td>
              <td>
                <div class="d-flex align-items-center justify-content-between">
                  <span>{{ cmd.triggered_by || '-' }}</span>
                  <button
                    @click="watchCommand(cmd)"
                    class="btn btn-sm btn-outline-primary ms-2"
                  >
                    Voir les logs
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="auditLogs.length" class="card mt-4">
      <div class="card-header">
        <h3 class="card-title">Audit APT (hote)</h3>
      </div>
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Date</th>
              <th>Commande</th>
              <th>Statut</th>
              <th>Utilisateur</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="log in auditLogs" :key="log.id">
              <td>{{ formatDate(log.created_at) }}</td>
              <td><code>{{ log.action }}</code></td>
              <td>
                <span :class="statusClass(log.status)">{{ log.status }}</span>
              </td>
              <td>{{ log.username }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
      </div>

      <!-- Colonne droite: Console Live -->
      <div style="width: 38%; min-width: 450px; display: flex; flex-direction: column;">
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
            <button 
              v-if="liveCommand" 
              @click="closeLiveConsole" 
              class="btn btn-sm btn-ghost-secondary"
              title="Fermer la console"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="icon" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                <path d="M18 6l-12 12" />
                <path d="M6 6l12 12" />
              </svg>
            </button>
          </div>
          <div class="card-body d-flex flex-column" style="flex: 1; min-height: 0; padding: 0;">
            <!-- État vide -->
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

            <!-- Console active -->
            <div v-else style="display: flex; flex-direction: column; height: 100%;">
              <div class="px-3 pt-3 pb-2" style="background: #1e293b; border-bottom: 1px solid rgba(255,255,255,0.1);">
                <div class="d-flex align-items-start justify-content-between mb-2">
                  <div class="flex-fill" style="min-width: 0;">
                    <div class="fw-semibold text-light" style="font-size: 0.95rem;">{{ host?.hostname || 'Hôte' }}</div>
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
              >{{ liveCommand.output || 'En attente de sortie...' }}</pre>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Line } from 'vue-chartjs'
import { Chart as ChartJS, LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip } from 'chart.js'
import RelativeTime from '../components/RelativeTime.vue'
import CVEList from '../components/CVEList.vue'
import apiClient from '../api'
import { useAuthStore } from '../stores/auth'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import utc from 'dayjs/plugin/utc'
import 'dayjs/locale/fr'

ChartJS.register(LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip)
dayjs.extend(relativeTime)
dayjs.extend(utc)
dayjs.locale('fr')

const LATEST_AGENT_VERSION = '1.2.0'

const route = useRoute()
const router = useRouter()
const hostId = route.params.id

const host = ref(null)
const metrics = ref(null)
const containers = ref([])
const aptStatus = ref(null)
const aptHistory = ref([])
const auditLogs = ref([])
const metricsHistory = ref([])
const chartHours = ref(24)
const cpuChartData = ref(null)
const memChartData = ref(null)
const isEditing = ref(false)
const saving = ref(false)
const editForm = ref({ name: '', hostname: '', ip_address: '', os: '' })
const liveCommand = ref(null)
const consoleOutput = ref(null)
let ws = null
let streamWs = null
const auth = useAuthStore()
const canRunApt = computed(() => auth.role === 'admin' || auth.role === 'operator')

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      enabled: true,
      mode: 'index',
      intersect: false,
      backgroundColor: 'rgba(0, 0, 0, 0.8)',
      titleColor: '#fff',
      bodyColor: '#fff',
      borderColor: '#555',
      borderWidth: 1,
      padding: 10,
      displayColors: false,
      callbacks: {
        title: (items) => {
          return items[0]?.label || ''
        },
        label: (context) => {
          const value = context.parsed.y.toFixed(1)
          return `${value}%`
        },
      },
    },
  },
  scales: {
    x: { display: true, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', maxTicksLimit: 10 } },
    y: { display: true, min: 0, max: 100, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280' } },
  },
  elements: { point: { radius: 0, hitRadius: 10, hoverRadius: 5 }, line: { tension: 0.3 } },
  interaction: { mode: 'nearest', axis: 'x', intersect: false },
}

function connectWebSocket() {
  if (!auth.token) return
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const wsUrl = `${protocol}://${window.location.host}/api/v1/ws/hosts/${hostId}?token=${auth.token}`
  ws = new WebSocket(wsUrl)

  ws.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data)
      if (payload.type !== 'host_detail') return
      host.value = payload.host
      metrics.value = payload.metrics
      containers.value = payload.containers || []
      aptStatus.value = payload.apt_status
      aptHistory.value = payload.apt_history || []
      auditLogs.value = payload.audit_logs || []
    } catch (e) {
      // Ignore malformed payloads
    }
  }

  ws.onclose = () => {
    setTimeout(connectWebSocket, 2000)
  }
}

async function loadHistory(hours) {
  chartHours.value = hours
  try {
    const res = await apiClient.getMetricsHistory(hostId, hours)
    const history = Array.isArray(res.data) ? res.data : []
    metricsHistory.value = history
    if (!history.length) {
      cpuChartData.value = null
      memChartData.value = null
      return
    }
    buildCharts()
  } catch (e) {
    console.error('Failed to fetch metrics history:', e)
  }
}

function buildCharts() {
  const labels = metricsHistory.value.map(m =>
    chartHours.value >= 24 ? dayjs(m.timestamp).format('DD/MM HH:mm') : dayjs(m.timestamp).format('HH:mm')
  )
  cpuChartData.value = {
    labels,
    datasets: [{
      data: metricsHistory.value.map(m => m.cpu_usage_percent),
      borderColor: '#3b82f6',
      backgroundColor: 'rgba(59, 130, 246, 0.1)',
      fill: true,
    }],
  }
  memChartData.value = {
    labels,
    datasets: [{
      data: metricsHistory.value.map(m => m.memory_percent),
      borderColor: '#10b981',
      backgroundColor: 'rgba(16, 185, 129, 0.1)',
      fill: true,
    }],
  }
}

function startEdit() {
  if (!host.value) return
  editForm.value = {
    name: host.value.name || '',
    hostname: host.value.hostname || '',
    ip_address: host.value.ip_address || '',
    os: host.value.os || '',
  }
  isEditing.value = true
}

function cancelEdit() {
  isEditing.value = false
}

async function saveEdit() {
  saving.value = true
  try {
    const res = await apiClient.updateHost(hostId, editForm.value)
    host.value = res.data
    isEditing.value = false
  } catch (e) {
    alert('Erreur: ' + (e.response?.data?.error || e.message))
  } finally {
    saving.value = false
  }
}

async function sendAptCmd(command) {
  if (!confirm(`Exécuter 'apt ${command}' sur ${host.value?.hostname} ?`)) return
  try {
    const response = await apiClient.sendAptCommand([hostId], command)
    alert(`Commande 'apt ${command}' envoyée. L'agent l'exécutera au prochain rapport.`)
    
    // Auto-open console with command
    if (response.data?.commands?.length > 0) {
      const cmd = response.data.commands[0]
      if (cmd.command_id) {
        watchCommand({
          id: cmd.command_id,
          command: command,
          status: 'pending',
          output: ''
        })
      }
    }
  } catch (e) {
    alert('Erreur: ' + (e.response?.data?.error || e.message))
  }
}

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const k = 1024
  const sizes = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function formatUptime(seconds) {
  if (!seconds) return 'N/A'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  if (days > 0) return `${days}j ${hours}h`
  const mins = Math.floor((seconds % 3600) / 60)
  return `${hours}h ${mins}m`
}

function formatDate(date) {
  if (!date || date === '0001-01-01T00:00:00Z') return 'Jamais'
  return dayjs.utc(date).local().fromNow()
}

function cpuColor(pct) {
  if (!pct) return 'text-secondary'
  if (pct > 90) return 'text-red'
  if (pct > 70) return 'text-yellow'
  return 'text-green'
}

function memColor(pct) {
  if (!pct) return 'text-secondary'
  if (pct > 90) return 'text-red'
  if (pct > 75) return 'text-yellow'
  return 'text-green'
}

function statusClass(status) {
  if (status === 'completed') return 'badge bg-green-lt text-green'
  if (status === 'failed') return 'badge bg-red-lt text-red'
  return 'badge bg-yellow-lt text-yellow'
}

function isAgentUpToDate(version) {
  if (!version) return false
  return version === LATEST_AGENT_VERSION
}

function watchCommand(cmd) {
  liveCommand.value = {
    id: cmd.id,
    command: cmd.command,
    status: cmd.status,
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
  if (consoleOutput.value) {
    consoleOutput.value.scrollTop = consoleOutput.value.scrollHeight
  }
}

function connectStreamWebSocket(commandId) {
  if (streamWs) {
    streamWs.close()
  }
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const wsUrl = `${protocol}://${window.location.host}/api/v1/ws/apt/stream/${commandId}?token=${auth.token}`
  streamWs = new WebSocket(wsUrl)

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
    } catch (e) {
      // Ignore malformed payloads
    }
  }

  streamWs.onclose = () => {
    // Connection closed, don't reconnect automatically
  }
}

async function deleteHost() {
  if (!confirm(`Êtes-vous sûr de vouloir supprimer ${host.value?.hostname} ? Cette action est irréversible.`)) {
    return
  }
  try {
    await apiClient.deleteHost(hostId)
    alert(`Hôte ${host.value?.hostname} supprimé avec succès.`)
    router.push('/')
  } catch (e) {
    alert('Erreur: ' + (e.response?.data?.error || e.message))
  }
}

onMounted(() => {
  connectWebSocket()
  loadHistory(24)
})

onUnmounted(() => {
  if (ws) ws.close()
  if (streamWs) streamWs.close()
})
</script>
