<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center
                justify-content-between gap-3 mb-4">
      <div>
        <h2 class="page-title">Dashboard</h2>
        <div class="text-secondary">Vue d'ensemble de l'infrastructure</div>
      </div>

      <div class="d-flex flex-wrap align-items-center gap-2">
        <!-- Sélection + APT : groupés, wrappables -->
        <template v-if="canRunApt">
          <div class="d-flex flex-wrap gap-2">
            <button class="btn btn-outline-secondary btn-sm" @click="selectAllFiltered">
              Tout sélectionner
            </button>
            <button class="btn btn-outline-secondary btn-sm" @click="clearSelection" :disabled="selectedCount === 0">
              Vider
            </button>
            <button class="btn btn-outline-secondary btn-sm" @click="sendBulkApt('update')" :disabled="selectedCount === 0">
              apt update ({{ selectedCount }})
            </button>
            <button class="btn btn-primary btn-sm" @click="sendBulkApt('upgrade')" :disabled="selectedCount === 0">
              apt upgrade ({{ selectedCount }})
            </button>
          </div>
        </template>
        <div v-else class="text-secondary small">Mode lecture seule</div>

        <!-- Bouton Ajouter : icône seule sur xs, texte complet sur sm+ -->
        <router-link to="/hosts/new" class="btn btn-primary">
          <svg class="icon" width="20" height="20" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"/>
          </svg>
          <span class="d-none d-sm-inline ms-1">Ajouter un hôte</span>
        </router-link>
      </div>
    </div>

    <WsStatusBar :status="wsStatus" :error="wsError" :retry-count="retryCount" @reconnect="reconnect" />

    <div class="row row-cards mb-4">
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="subheader">Hôtes</div>
            <div class="h1 mb-0">{{ hosts.length }}</div>
          </div>
        </div>
      </div>
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="subheader">En ligne</div>
            <div class="h1 mb-0 text-green">{{ onlineCount }}</div>
          </div>
        </div>
      </div>
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="subheader">Hors ligne</div>
            <div class="h1 mb-0 text-red">{{ offlineCount }}</div>
          </div>
        </div>
      </div>
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="subheader">Mises à jour disponibles</div>
            <div class="h1 mb-0 text-yellow">{{ outdatedVersions }}</div>
          </div>
        </div>
      </div>
    </div>

    <div class="card mb-4">
      <div class="card-body">
        <div class="row g-3 align-items-center">
          <div class="col-12 col-lg">
            <input v-model="searchQuery" type="text" class="form-control" placeholder="Rechercher un hote..." />
          </div>
          <div class="col-12 col-md-4 col-lg-2">
            <select v-model="statusFilter" class="form-select">
              <option value="all">Tous</option>
              <option value="online">En ligne</option>
              <option value="offline">Hors ligne</option>
              <option value="warning">Warning</option>
            </select>
          </div>
          <div class="col-12 col-md-4 col-lg-3">
            <select v-model="sortKey" class="form-select">
              <option value="name">Trier par nom</option>
              <option value="status">Trier par statut</option>
              <option value="cpu">Trier par CPU</option>
              <option value="last_seen">Trier par derniere activite</option>
            </select>
          </div>
          <div class="col-12 col-md-4 col-lg-2">
            <button class="btn btn-outline-secondary w-100" @click="sortDir = sortDir === 'asc' ? 'desc' : 'asc'">
              {{ sortDir === 'asc' ? 'Asc' : 'Desc' }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <div class="card mb-4">
      <div class="card-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3">
        <div>
          <h3 class="card-title">Tendance globale CPU / RAM</h3>
          <div class="text-secondary small">Moyenne sur tous les hotes</div>
        </div>
        <div class="btn-group btn-group-sm">
          <button
            v-for="h in [1, 6, 24, 168, 720]"
            :key="h"
            @click="changeSummaryRange(h)"
            :class="summaryHours === h ? 'btn btn-primary' : 'btn btn-outline-secondary'"
          >
            {{ h >= 24 ? (h / 24) + 'j' : h + 'h' }}
          </button>
        </div>
      </div>
      <div class="card-body" style="height: 14rem;">
        <Line v-if="summaryChartData" :data="summaryChartData" :options="summaryChartOptions" class="h-100" />
        <div v-else class="h-100 d-flex align-items-center justify-content-center text-secondary">Aucune donnee</div>
      </div>
    </div>

    <div class="card">
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th style="width: 1%"></th>
              <th>Nom</th>
              <th>Etat</th>
              <th>IP / OS</th>
              <th>Agent</th>
              <th>CPU</th>
              <th>RAM</th>
              <th>Uptime</th>
              <th>Derniere activite</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="host in sortedHosts" :key="host.id">
              <td>
                <label class="form-check">
                  <input
                    class="form-check-input"
                    type="checkbox"
                    :value="host.id"
                    v-model="selectedHostIds"
                  />
                  <span class="form-check-label"></span>
                </label>
              </td>
              <td>
                <router-link :to="`/hosts/${host.id}`" class="fw-semibold text-decoration-none">
                  {{ host.name || host.hostname || 'Sans nom' }}
                </router-link>
                <div class="text-secondary small">{{ host.hostname || 'Non connecte' }}</div>
              </td>
              <td>
                <span :class="host.status === 'online' ? 'badge bg-green-lt text-green' : host.status === 'warning' ? 'badge bg-yellow-lt text-yellow' : 'badge bg-red-lt text-red'">
                  {{ host.status === 'online' ? 'En ligne' : host.status === 'warning' ? 'Warning' : 'Hors ligne' }}
                </span>
              </td>
              <td>
                <div class="text-body">{{ host.ip_address }}</div>
                <div class="text-secondary small">{{ host.os || 'N/A' }}</div>
              </td>
              <td>
                <span v-if="host.agent_version" :class="isAgentUpToDate(host.agent_version) ? 'badge bg-green-lt text-green' : 'badge bg-yellow-lt text-yellow'">
                  v{{ host.agent_version }}
                </span>
                <span v-else class="text-secondary small">-</span>
              </td>
              <td>
                <span :class="cpuColor(hostMetrics[host.id]?.cpu_usage_percent)">
                  {{ hostMetrics[host.id]?.cpu_usage_percent?.toFixed(1) ?? '-' }}
                </span>
              </td>
              <td>
                <span :class="memColor(hostMetrics[host.id]?.memory_percent)">
                  {{ hostMetrics[host.id]?.memory_percent?.toFixed(1) ?? '-' }}
                </span>
              </td>
              <td>
                {{ hostMetrics[host.id] ? formatUptime(hostMetrics[host.id].uptime) : '-' }}
              </td>
              <td><RelativeTime :date="host.last_seen" /></td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="loading" class="text-center py-4">
        <div class="spinner-border" role="status"></div>
      </div>
    </div>

    <div v-if="versionComparisons.length > 0" class="card mt-4">
      <div class="card-header">
        <h3 class="card-title">Versions & Mises a jour</h3>
      </div>
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Image</th>
              <th>Hote</th>
              <th>En cours</th>
              <th>Derniere version</th>
              <th>Statut</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="v in versionComparisons" :key="v.docker_image + v.host_id">
              <td class="fw-semibold">{{ v.docker_image }}</td>
              <td class="text-secondary">{{ v.hostname }}</td>
              <td><code>{{ v.running_version }}</code></td>
              <td>
                <a v-if="v.release_url" :href="v.release_url" target="_blank" class="link-primary">
                  {{ v.latest_version }}
                </a>
                <span v-else>{{ v.latest_version }}</span>
              </td>
              <td>
                <span v-if="v.is_up_to_date" class="badge bg-green-lt text-green">A jour</span>
                <span v-else class="badge bg-yellow-lt text-yellow">Mise a jour disponible</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import RelativeTime from '../components/RelativeTime.vue'
import WsStatusBar from '../components/WsStatusBar.vue'
import apiClient from '../api'
import { useAuthStore } from '../stores/auth'
import { useWebSocket } from '../composables/useWebSocket'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import { Line } from 'vue-chartjs'
import { Chart as ChartJS, LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip } from 'chart.js'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import utc from 'dayjs/plugin/utc'
import 'dayjs/locale/fr'

ChartJS.register(LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip)
dayjs.extend(relativeTime)
dayjs.extend(utc)
dayjs.locale('fr')

const LATEST_AGENT_VERSION = '1.2.0'

const hosts = ref([])
const hostMetrics = ref({})
const versionComparisons = ref([])
const loading = ref(true)
const searchQuery = ref('')
const statusFilter = ref('all')
const sortKey = ref('name')
const sortDir = ref('asc')
const selectedHostIds = ref([])
const auth = useAuthStore()
const dialog = useConfirmDialog()
const summaryHours = ref(24)
const summaryChartData = ref(null)
const summaryChartOptions = {
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
      displayColors: true,
      callbacks: {
        title: (items) => {
          return items[0]?.label || ''
        },
        label: (context) => {
          const label = context.datasetIndex === 0 ? 'CPU' : 'RAM'
          const value = context.parsed.y.toFixed(1)
          return `${label}: ${value}%`
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

const onlineCount = computed(() => hosts.value.filter(h => h.status === 'online').length)
const offlineCount = computed(() => hosts.value.filter(h => h.status !== 'online').length)
const outdatedVersions = computed(() => versionComparisons.value.filter(v => !v.is_up_to_date).length)
const selectedCount = computed(() => selectedHostIds.value.length)
const canRunApt = computed(() => auth.role === 'admin' || auth.role === 'operator')

const filteredHosts = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  return hosts.value.filter((host) => {
    if (statusFilter.value !== 'all' && host.status !== statusFilter.value) {
      return false
    }
    if (!query) return true
    const haystack = [
      host.name,
      host.hostname,
      host.ip_address,
      host.os,
    ].filter(Boolean).join(' ').toLowerCase()
    return haystack.includes(query)
  })
})

const sortedHosts = computed(() => {
  const list = [...filteredHosts.value]
  const direction = sortDir.value === 'asc' ? 1 : -1

  const statusOrder = { online: 0, warning: 1, offline: 2 }
  const getCpu = (host) => hostMetrics.value[host.id]?.cpu_usage_percent ?? -1

  list.sort((a, b) => {
    let aVal
    let bVal
    switch (sortKey.value) {
      case 'status':
        aVal = statusOrder[a.status] ?? 99
        bVal = statusOrder[b.status] ?? 99
        break
      case 'cpu':
        aVal = getCpu(a)
        bVal = getCpu(b)
        break
      case 'last_seen':
        aVal = a.last_seen ? new Date(a.last_seen).getTime() : 0
        bVal = b.last_seen ? new Date(b.last_seen).getTime() : 0
        break
      case 'name':
      default:
        aVal = (a.name || a.hostname || '').toLowerCase()
        bVal = (b.name || b.hostname || '').toLowerCase()
        break
    }
    if (aVal < bVal) return -1 * direction
    if (aVal > bVal) return 1 * direction
    return 0
  })
  return list
})

const { wsStatus, wsError, retryCount, reconnect } = useWebSocket('/api/v1/ws/dashboard', (payload) => {
  if (payload.type !== 'dashboard') return
  hosts.value = payload.hosts || []
  hostMetrics.value = payload.host_metrics || {}
  versionComparisons.value = payload.version_comparisons || []
  selectedHostIds.value = selectedHostIds.value.filter(id => hosts.value.some(h => h.id === id))
  loading.value = false
})

function bucketMinutesFor(hours) {
  if (hours <= 6) return 1
  if (hours <= 24) return 5
  if (hours <= 168) return 15
  return 60
}

async function fetchSummary() {
  try {
    const bucketMinutes = bucketMinutesFor(summaryHours.value)
    const res = await apiClient.getMetricsSummary(summaryHours.value, bucketMinutes)
    const points = Array.isArray(res.data) ? res.data : []
    if (!points.length) {
      summaryChartData.value = null
      return
    }
    const labels = points.map(p =>
      summaryHours.value >= 24 ? dayjs(p.timestamp).format('DD/MM HH:mm') : dayjs(p.timestamp).format('HH:mm')
    )
    summaryChartData.value = {
      labels,
      datasets: [
        {
          data: points.map(p => p.cpu_avg),
          borderColor: '#3b82f6',
          backgroundColor: 'rgba(59, 130, 246, 0.1)',
          fill: true,
        },
        {
          data: points.map(p => p.memory_avg),
          borderColor: '#10b981',
          backgroundColor: 'rgba(16, 185, 129, 0.1)',
          fill: true,
        },
      ],
    }
  } catch (e) {
    summaryChartData.value = null
  }
}

function changeSummaryRange(hours) {
  summaryHours.value = hours
  fetchSummary()
}

function selectAllFiltered() {
  const ids = sortedHosts.value.map(h => h.id)
  selectedHostIds.value = Array.from(new Set([...selectedHostIds.value, ...ids]))
}

function clearSelection() {
  selectedHostIds.value = []
}

async function sendBulkApt(command) {
  if (!selectedHostIds.value.length) return
  
  const hostnames = hosts.value
    .filter(h => selectedHostIds.value.includes(h.id))
    .map(h => h.hostname || h.name)
    .join(', ')
  
  const confirmed = await dialog.confirm({
    title: `apt ${command}`,
    message: `Exécuter sur ${selectedHostIds.value.length} hôte(s) :\n${hostnames}`,
    variant: 'warning'
  })
  
  if (!confirmed) return
  
  try {
    await apiClient.sendAptCommand(selectedHostIds.value, command)
  } catch (e) {
    await dialog.confirm({
      title: 'Erreur',
      message: e.response?.data?.error || e.message,
      variant: 'danger'
    })
  }
}

function formatDate(date) {
  return dayjs.utc(date).local().fromNow()
}

function formatUptime(seconds) {
  if (!seconds) return 'N/A'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  if (days > 0) return `${days}j ${hours}h`
  const mins = Math.floor((seconds % 3600) / 60)
  return `${hours}h ${mins}m`
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

function isAgentUpToDate(version) {
  if (!version) return false
  // Simple version comparison (assumes semantic versioning)
  return version === LATEST_AGENT_VERSION
}


onMounted(() => {
  loading.value = true
  fetchSummary()
})

onUnmounted(() => {
})
</script>