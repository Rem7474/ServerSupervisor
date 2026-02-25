<template>
  <div class="host-detail-page">
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <div class="text-secondary small">
            <router-link to="/" class="text-decoration-none">Dashboard</router-link>
            <span class="mx-1">/</span>
            <span>Hôte</span>
          </div>
          <h2 class="page-title">{{ host?.name || host?.hostname || 'Chargement...' }}</h2>
          <div class="text-secondary">
            {{ host?.hostname || 'Non connecté' }} — {{ host?.os || 'OS inconnu' }} • {{ host?.ip_address }}
            <span v-if="host?.last_seen">• Dernière activité: <RelativeTime :date="host.last_seen" /></span>
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
        <span v-if="host" :class="hostStatusClass(host.status)">
          {{ formatHostStatus(host.status) }}
        </span>
        </div>
      </div>
    </div>

    <WsStatusBar :status="wsStatus" :error="wsError" :retry-count="retryCount" @reconnect="reconnect" />

    <div class="host-layout">
      <!-- Colonne gauche: Informations hôte -->
      <div class="host-panel-main">

    <div v-if="isEditing" class="card mb-4">
      <div class="card-header">
        <h3 class="card-title">Modifier l'hôte</h3>
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
          <div class="col-12">
            <div class="border-top pt-3 mt-2">
              <div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-2">
                <div>
                  <div class="fw-semibold">API Key agent</div>
                  <div class="text-secondary small">Regenerer la cle pour un hote existant.</div>
                </div>
                <button type="button" class="btn btn-outline-warning" :disabled="rotateKeyLoading" @click="rotateHostKey">
                  {{ rotateKeyLoading ? 'Rotation...' : 'Regenerer la cle' }}
                </button>
              </div>
              <div v-if="rotateKeyResult" class="alert alert-info mt-3 mb-0" role="alert">
                <div class="fw-semibold mb-2">Nouvelle cle generee</div>
                <div class="text-secondary small mb-2">Copiez-la maintenant, elle ne sera plus affichee.</div>
                <div class="d-flex align-items-center gap-2 mb-2">
                  <div class="bg-dark rounded p-2 flex-fill">
                    <code class="text-light">{{ rotateKeyResult.api_key }}</code>
                  </div>
                  <button type="button" class="btn btn-outline-light" @click="copyRotatedKey">
                    {{ rotateCopiedKey ? 'Copie' : 'Copier' }}
                  </button>
                </div>
                <div class="d-flex align-items-center justify-content-between mb-1">
                  <div class="text-secondary small">Configuration agent :</div>
                  <button type="button" class="btn btn-outline-light btn-sm" @click="copyRotatedConfig">
                    {{ rotateCopiedConfig ? 'Copie' : 'Copier la config' }}
                  </button>
                </div>
                <pre class="bg-dark text-light p-2 rounded small mb-0">{{ rotatedAgentConfig }}</pre>
              </div>
            </div>
          </div>
        </form>
      </div>
    </div>

    <div v-if="metrics" class="row row-cards mb-4 g-3">
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">CPU ({{ metrics.cpu_cores }} CORES)</div>
            <div class="h2 mb-0" :class="cpuColor(metrics.cpu_usage_percent)">
              {{ metrics.cpu_usage_percent?.toFixed(1) }}%
            </div>
            <div class="text-secondary small">{{ metrics.cpu_model }}</div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
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
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">UPTIME</div>
            <div class="h2 mb-0 text-primary">{{ formatUptime(metrics.uptime) }}</div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">LOAD AVG</div>
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
            <h3 class="card-title">CPU</h3>
            <div class="btn-group btn-group-sm">
              <button v-for="opt in timeRangeOptions" :key="opt.hours" @click="loadHistory(opt.hours)"
                :class="chartHours === opt.hours ? 'btn btn-primary' : 'btn btn-outline-secondary'">
                {{ opt.label }}
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
            <h3 class="card-title">Memoire</h3>
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

    <!-- Disk Metrics Card - Filesystem Usage -->
    <DiskMetricsCard :hostId="hostId" class="mb-4" />

    <!-- Disk Health Card - SMART Data -->
    <DiskHealthCard :hostId="hostId" class="mb-4" />

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
            <tr v-for="cmd in displayedAptHistory" :key="cmd.id">
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
      <div v-if="aptHistory.length > 3 && !showFullAptHistory" class="card-footer text-center">
        <button @click="showFullAptHistory = true" class="btn btn-outline-primary btn-sm">
          Afficher plus ({{ aptHistory.length - 3 }} autres)
        </button>
      </div>
    </div>

    <!-- Logs système (journalctl) -->
    <div class="card mt-4">
      <div class="card-header">
        <h3 class="card-title">Logs système (journalctl)</h3>
      </div>
      <div class="card-body">
        <div class="d-flex gap-2">
          <input
            v-model="journalService"
            type="text"
            class="form-control"
            placeholder="Nom du service (ex: nginx, ssh, docker)"
            @keyup.enter="loadJournalLogs"
            style="max-width: 320px;"
          />
          <button
            class="btn btn-primary"
            @click="loadJournalLogs"
            :disabled="!journalService.trim() || journalLoading"
          >
            <span v-if="journalLoading" class="spinner-border spinner-border-sm me-1"></span>
            {{ journalLoading ? 'Chargement...' : 'Charger les logs' }}
          </button>
        </div>
        <div v-if="journalError" class="alert alert-danger mt-3 mb-0">{{ journalError }}</div>
        <div v-if="journalCmdId" class="text-secondary small mt-2">
          Stream → commande #{{ journalCmdId }} — les logs apparaissent dans la Console Live →
        </div>
      </div>
    </div>

      </div>

      <!-- Colonne droite: Console Live -->
      <div 
        v-show="showConsole"
        class="host-panel-right"
      >
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
            <div class="d-flex gap-2">
              <button
                @click="closeLiveConsole(); showConsole = false"
                class="btn btn-sm btn-ghost-secondary"
                title="Masquer la console"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="icon" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                  <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                  <path d="M17 6l-10 10" />
                  <path d="M7 6l10 10" />
                </svg>
              </button>
            </div>
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
              >{{ renderedConsoleOutput || 'En attente de sortie...' }}</pre>
            </div>
          </div>
        </div>
      </div>

      <!-- Bouton pour afficher la console quand cachée -->
      <button 
        v-show="!showConsole"
        @click="showConsole = true" 
        class="btn btn-sm btn-outline-secondary ms-auto"
        style="position: absolute; bottom: 1rem; right: 1rem; z-index: 10;"
        title="Afficher la console"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="icon me-1" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
          <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
          <path d="M8 9l3 3l-3 3" />
          <path d="M13 15l3 0" />
          <path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z" />
        </svg>
        Console
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, shallowRef, onMounted, onUnmounted, computed, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Line } from 'vue-chartjs'
import { Chart as ChartJS, LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip } from 'chart.js'
import RelativeTime from '../components/RelativeTime.vue'
import CVEList from '../components/CVEList.vue'
import DiskMetricsCard from '../components/DiskMetricsCard.vue'
import DiskHealthCard from '../components/DiskHealthCard.vue'
import apiClient from '../api'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import { useWebSocket } from '../composables/useWebSocket'
import WsStatusBar from '../components/WsStatusBar.vue'
import { formatHostStatus, hostStatusClass } from '../utils/formatHostStatus'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import utc from 'dayjs/plugin/utc'
import 'dayjs/locale/fr'

ChartJS.register(LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip)
dayjs.extend(relativeTime)
dayjs.extend(utc)
dayjs.locale('fr')

const LATEST_AGENT_VERSION = '1.3.0'

const route = useRoute()
const router = useRouter()
const hostId = route.params.id

const host = ref(null)
const metrics = ref(null)
const containers = ref([])
const aptStatus = ref(null)
const aptHistory = ref([])
const showFullAptHistory = ref(false)
const auditLogs = ref([])
const metricsHistory = ref([])
const chartHours = ref(24)
const cpuChartData = shallowRef(null)
const memChartData = shallowRef(null)
const isEditing = ref(false)
const saving = ref(false)
const editForm = ref({ name: '', hostname: '', ip_address: '', os: '' })
const rotateKeyLoading = ref(false)
const rotateKeyResult = ref(null)
const rotateCopiedKey = ref(false)
const rotateCopiedConfig = ref(false)
const liveCommand = ref(null)
const consoleOutput = ref(null)
const showConsole = ref(false)
let streamWs = null
const journalService = ref('')
const journalLoading = ref(false)
const journalError = ref('')
const journalCmdId = ref(null)
const auth = useAuthStore()
const dialog = useConfirmDialog()
const canRunApt = computed(() => auth.role === 'admin' || auth.role === 'operator')

const serverHostname =
  typeof window !== 'undefined' && window.location?.hostname
    ? window.location.hostname
    : 'localhost'

const rotatedAgentConfig = computed(() => {
  if (!rotateKeyResult.value) return ''
  return `server_url: "http://${serverHostname}:8080"\napi_key: "${rotateKeyResult.value.api_key}"\nreport_interval: 30\ncollect_docker: true\ncollect_apt: true`
})

// Time range options for metrics
const timeRangeOptions = [
  { hours: 1, label: '1h' },
  { hours: 6, label: '6h' },
  { hours: 24, label: '24h' },
  { hours: 168, label: '7d' },
  { hours: 720, label: '30d' },
  { hours: 2160, label: '90d' },
  { hours: 8760, label: '1y' },
]

const renderedConsoleOutput = computed(() => {
  if (!liveCommand.value) return ''
  const raw = liveCommand.value.output || ''
  return renderConsoleOutput(raw)
})

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

const { wsStatus, wsError, retryCount, reconnect } = useWebSocket(`/api/v1/ws/hosts/${hostId}`, (payload) => {
  if (payload.type !== 'host_detail') return
  host.value = payload.host
  metrics.value = payload.metrics
  containers.value = payload.containers || []
  aptStatus.value = payload.apt_status
  aptHistory.value = payload.apt_history || []
  auditLogs.value = payload.audit_logs || []
}, { debounceMs: 200 })

async function loadHistory(hours) {
  chartHours.value = hours
  try {
    let history
    
    // Use aggregated metrics for periods > 24 hours
    if (hours > 24) {
      const res = await apiClient.getMetricsAggregated(hostId, hours)
      history = Array.isArray(res.data?.metrics) ? res.data.metrics : []
    } else {
      const res = await apiClient.getMetricsHistory(hostId, hours)
      history = Array.isArray(res.data) ? res.data : []
    }
    
    metricsHistory.value = history
    if (!history.length) {
      cpuChartData.value = null
      memChartData.value = null
      return
    }
    buildCharts()
  } catch (e) {
    console.error(`Failed to fetch metrics history (${hours}h):`, e.response?.data || e.message)
  }
}

function buildCharts() {
  const labels = metricsHistory.value.map(m => {
    const date = dayjs(m.timestamp)
    // Format labels based on time range
    if (chartHours.value <= 24) {
      return date.format('HH:mm')
    } else if (chartHours.value <= 720) { // 30 days
      return date.format('DD/MM HH:mm')
    } else {
      return date.format('DD/MM')
    }
  })
  
  cpuChartData.value = {
    labels,
    datasets: [{
      data: metricsHistory.value.map(m => m.cpu_usage_percent),
      borderColor: '#3b82f6',
      backgroundColor: 'rgba(59, 130, 246, 0.1)',
      fill: true,
      tension: 0.3,
    }],
  }
  memChartData.value = {
    labels,
    datasets: [{
      data: metricsHistory.value.map(m => m.memory_percent),
      borderColor: '#10b981',
      backgroundColor: 'rgba(16, 185, 129, 0.1)',
      fill: true,
      tension: 0.3,
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
  rotateKeyResult.value = null
  isEditing.value = true
}

function cancelEdit() {
  isEditing.value = false
  rotateKeyResult.value = null
}

async function rotateHostKey() {
  rotateKeyLoading.value = true
  rotateKeyResult.value = null
  try {
    const res = await apiClient.rotateHostKey(hostId)
    rotateKeyResult.value = res.data
  } catch (e) {
    console.error('Failed to rotate API key:', e.response?.data || e.message)
  } finally {
    rotateKeyLoading.value = false
  }
}

async function copyRotatedKey() {
  if (!rotateKeyResult.value?.api_key) return
  await navigator.clipboard.writeText(rotateKeyResult.value.api_key)
  rotateCopiedKey.value = true
  setTimeout(() => {
    rotateCopiedKey.value = false
  }, 1500)
}

async function copyRotatedConfig() {
  if (!rotatedAgentConfig.value) return
  await navigator.clipboard.writeText(rotatedAgentConfig.value)
  rotateCopiedConfig.value = true
  setTimeout(() => {
    rotateCopiedConfig.value = false
  }, 1500)
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
  const confirmed = await dialog.confirm({
    title: `apt ${command}`,
    message: `Exécuter sur : ${host.value?.hostname}`,
    variant: command === 'dist-upgrade' ? 'danger' : 'warning'
  })
  
  if (!confirmed) return
  
  try {
    const response = await apiClient.sendAptCommand([hostId], command)
    
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
    await dialog.confirm({
      title: 'Erreur',
      message: e.response?.data?.error || e.message,
      variant: 'danger'
    })
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

const displayedAptHistory = computed(() => {
  if (showFullAptHistory.value) {
    return aptHistory.value
  }
  return aptHistory.value.slice(0, 3)
})

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

function isAgentUpToDate(version) {
  if (!version) return false
  return version === LATEST_AGENT_VERSION
}

async function loadJournalLogs() {
  const svc = journalService.value.trim()
  if (!svc) return
  journalLoading.value = true
  journalError.value = ''
  journalCmdId.value = null
  try {
    const res = await apiClient.sendJournalCommand(hostId, svc)
    const cmdId = res.data.command_id
    journalCmdId.value = cmdId
    liveCommand.value = {
      id: cmdId,
      command: `journalctl -u ${svc}`,
      status: 'running',
      output: '',
    }
    showConsole.value = true
    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const wsUrl = `${protocol}://${window.location.host}/api/v1/ws/apt/stream/${cmdId}?cmd_type=docker`
    if (streamWs) streamWs.close()
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
      } catch (e) { /* ignore */ }
    }
    streamWs.onclose = () => { journalLoading.value = false }
  } catch (e) {
    journalError.value = e.response?.data?.error || 'Impossible d\'envoyer la commande'
    journalLoading.value = false
  }
}

function watchCommand(cmd) {
  liveCommand.value = {
    id: cmd.id,
    command: cmd.command,
    status: cmd.status,
    output: cmd.output || '',
  }
  showConsole.value = true // Auto-show console si masquée
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
    } catch (e) {
      // Ignore malformed payloads
    }
  }

  streamWs.onclose = () => {
    // Connection closed, don't reconnect automatically
  }
}

async function deleteHost() {
  const confirmed = await dialog.confirm({
    title: 'Supprimer l\'hôte',
    message: `Êtes-vous sûr de vouloir supprimer ${host.value?.hostname} ?\nCette action est irréversible.`,
    variant: 'danger'
  })
  
  if (!confirmed) return
  
  try {
    await apiClient.deleteHost(hostId)
    router.push('/')
  } catch (e) {
    await dialog.confirm({
      title: 'Erreur',
      message: e.response?.data?.error || e.message,
      variant: 'danger'
    })
  }
}

onMounted(() => {
  // Wait for auth to be properly initialized before loading data
  if (auth.token) {
    loadHistory(24)
  } else {
    // Retry after a short delay if token not yet loaded
    const timer = setTimeout(() => {
      if (auth.token) loadHistory(24)
    }, 100)
    onUnmounted(() => clearTimeout(timer))
  }
})

onUnmounted(() => {
  if (streamWs) streamWs.close()
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
    max-height: 50vh;
  }
}
</style>