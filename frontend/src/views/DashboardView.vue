<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
        <h2 class="page-title">Dashboard</h2>
        <div class="text-secondary">Vue d'ensemble de l'infrastructure</div>
      </div>
      <div class="d-flex align-items-center gap-2">
        <div class="btn-list">
          <button class="btn btn-outline-secondary" @click="selectAllFiltered">Tout selectionner</button>
          <button class="btn btn-outline-secondary" @click="clearSelection" :disabled="selectedCount === 0">Vider</button>
          <button class="btn btn-outline-secondary" @click="sendBulkApt('update')" :disabled="selectedCount === 0">
            apt update ({{ selectedCount }})
          </button>
          <button class="btn btn-primary" @click="sendBulkApt('upgrade')" :disabled="selectedCount === 0">
            apt upgrade ({{ selectedCount }})
          </button>
        </div>
        <router-link to="/hosts/new" class="btn btn-primary">
          <svg class="icon me-2" width="20" height="20" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"/>
          </svg>
          Ajouter un hote
        </router-link>
      </div>
    </div>

    <div class="row row-cards mb-4">
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="subheader">Hotes</div>
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
            <div class="subheader">Mises a jour dispo</div>
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

    <div class="card">
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th style="width: 1%"></th>
              <th>Nom</th>
              <th>Etat</th>
              <th>Latence</th>
              <th>IP / OS</th>
              <th>CPU</th>
              <th>RAM</th>
              <th>Uptime</th>
              <th>Derniere activite</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="host in sortedHosts" :key="host.id">
              <td>
                <input
                  class="form-check-input"
                  type="checkbox"
                  :checked="isSelected(host.id)"
                  @click.stop.prevent="toggleHostSelection(host.id)"
                />
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
                <span :class="latencyClass(host.last_seen)">
                  {{ formatLatency(host.last_seen) }}
                </span>
              </td>
              <td>
                <div class="text-body">{{ host.ip_address }}</div>
                <div class="text-secondary small">{{ host.os || 'N/A' }}</div>
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
              <td>{{ formatDate(host.last_seen) }}</td>
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
import apiClient from '../api'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/fr'

dayjs.extend(relativeTime)
dayjs.locale('fr')

const hosts = ref([])
const hostMetrics = ref({})
const versionComparisons = ref([])
const loading = ref(true)
const searchQuery = ref('')
const statusFilter = ref('all')
const sortKey = ref('name')
const sortDir = ref('asc')
const selectedHostIds = ref([])
let refreshInterval = null

const onlineCount = computed(() => hosts.value.filter(h => h.status === 'online').length)
const offlineCount = computed(() => hosts.value.filter(h => h.status !== 'online').length)
const outdatedVersions = computed(() => versionComparisons.value.filter(v => !v.is_up_to_date).length)
const selectedCount = computed(() => selectedHostIds.value.length)

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

async function fetchData() {
  try {
    const [hostsRes, versionsRes] = await Promise.all([
      apiClient.getHosts(),
      apiClient.getVersionComparisons().catch(() => ({ data: [] })),
    ])
    hosts.value = hostsRes.data
    versionComparisons.value = versionsRes.data || []
    selectedHostIds.value = selectedHostIds.value.filter(id => hosts.value.some(h => h.id === id))

    // Fetch latest metrics for each online host
    for (const host of hosts.value.filter(h => h.status === 'online')) {
      try {
        const res = await apiClient.getHostDashboard(host.id)
        if (res.data.metrics) {
          hostMetrics.value[host.id] = res.data.metrics
        }
      } catch (e) { /* ignore */ }
    }
  } catch (e) {
    console.error('Failed to fetch dashboard data:', e)
  } finally {
    loading.value = false
  }
}

function isSelected(hostId) {
  return selectedHostIds.value.includes(hostId)
}

function toggleHostSelection(hostId) {
  if (isSelected(hostId)) {
    selectedHostIds.value = selectedHostIds.value.filter(id => id !== hostId)
  } else {
    selectedHostIds.value = [...selectedHostIds.value, hostId]
  }
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
  if (!confirm(`Exécuter 'apt ${command}' sur ${selectedHostIds.value.length} hote(s) ?`)) return
  try {
    await apiClient.sendAptCommand(selectedHostIds.value, command)
    alert(`Commande 'apt ${command}' envoyee.`)
  } catch (e) {
    alert('Erreur: ' + (e.response?.data?.error || e.message))
  }
}

function formatDate(date) {
  return dayjs(date).fromNow()
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

function formatLatency(lastSeen) {
  if (!lastSeen) return '-'
  const seconds = dayjs().diff(dayjs(lastSeen), 'second')
  if (seconds < 0) return '-'
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  return `${minutes}m`
}

function latencyClass(lastSeen) {
  if (!lastSeen) return 'text-secondary'
  const seconds = dayjs().diff(dayjs(lastSeen), 'second')
  if (seconds < 0) return 'text-secondary'
  if (seconds <= 60) return 'text-green'
  if (seconds <= 180) return 'text-yellow'
  return 'text-red'
}

onMounted(() => {
  fetchData()
  refreshInterval = setInterval(fetchData, 30000)
})

onUnmounted(() => {
  if (refreshInterval) clearInterval(refreshInterval)
})
</script>
