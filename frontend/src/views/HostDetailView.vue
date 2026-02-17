<template>
  <div>
    <div class="flex items-center gap-4 mb-8">
      <router-link to="/" class="text-gray-400 hover:text-gray-200">
        <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18"/>
        </svg>
      </router-link>
      <div>
        <h1 class="text-2xl font-bold">{{ host?.hostname || 'Chargement...' }}</h1>
        <p class="text-gray-400 mt-1">{{ host?.ip_address }} &mdash; {{ host?.os }}</p>
      </div>
      <span v-if="host" :class="host.status === 'online' ? 'badge-online' : 'badge-offline'" class="ml-auto">
        {{ host.status === 'online' ? 'En ligne' : 'Hors ligne' }}
      </span>
    </div>

    <!-- Metrics Cards -->
    <div v-if="metrics" class="grid grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
      <div class="stat-card">
        <div class="text-3xl font-bold" :class="cpuColor(metrics.cpu_usage_percent)">
          {{ metrics.cpu_usage_percent?.toFixed(1) }}%
        </div>
        <div class="text-gray-400 mt-1">CPU ({{ metrics.cpu_cores }} cores)</div>
        <div class="text-xs text-gray-500 mt-1">{{ metrics.cpu_model }}</div>
      </div>
      <div class="stat-card">
        <div class="text-3xl font-bold" :class="memColor(metrics.memory_percent)">
          {{ metrics.memory_percent?.toFixed(1) }}%
        </div>
        <div class="text-gray-400 mt-1">RAM</div>
        <div class="text-xs text-gray-500 mt-1">{{ formatBytes(metrics.memory_used) }} / {{ formatBytes(metrics.memory_total) }}</div>
      </div>
      <div class="stat-card">
        <div class="text-3xl font-bold text-primary-400">{{ formatUptime(metrics.uptime) }}</div>
        <div class="text-gray-400 mt-1">Uptime</div>
      </div>
      <div class="stat-card">
        <div class="text-3xl font-bold text-gray-300">
          {{ metrics.load_avg_1?.toFixed(2) }}
        </div>
        <div class="text-gray-400 mt-1">Load Avg</div>
        <div class="text-xs text-gray-500 mt-1">{{ metrics.load_avg_5?.toFixed(2) }} / {{ metrics.load_avg_15?.toFixed(2) }}</div>
      </div>
    </div>

    <!-- Charts -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
      <div class="card">
        <h3 class="text-lg font-semibold mb-4">CPU ({{ chartHours }}h)</h3>
        <div class="flex gap-2 mb-4">
          <button v-for="h in [1, 6, 24, 72]" :key="h" @click="loadHistory(h)"
            :class="chartHours === h ? 'btn-primary' : 'btn-secondary'" class="text-xs px-3 py-1">
            {{ h }}h
          </button>
        </div>
        <div class="h-48">
          <Line v-if="cpuChartData" :data="cpuChartData" :options="chartOptions" class="h-full w-full" />
          <div v-else class="h-full flex items-center justify-center text-sm text-gray-500">Aucune donnee</div>
        </div>
      </div>
      <div class="card">
        <h3 class="text-lg font-semibold mb-4">Mémoire ({{ chartHours }}h)</h3>
        <div class="h-48">
          <Line v-if="memChartData" :data="memChartData" :options="chartOptions" class="h-full w-full" />
          <div v-else class="h-full flex items-center justify-center text-sm text-gray-500">Aucune donnee</div>
        </div>
      </div>
    </div>

    <!-- Disks -->
    <div v-if="metrics?.disks?.length" class="card mb-8">
      <h3 class="text-lg font-semibold mb-4">Disques</h3>
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div v-for="disk in metrics.disks" :key="disk.mount_point" class="bg-dark-900 rounded-lg p-4">
          <div class="flex justify-between mb-2">
            <span class="font-medium">{{ disk.mount_point }}</span>
            <span class="text-gray-400 text-sm">{{ disk.device }} ({{ disk.fs_type }})</span>
          </div>
          <div class="w-full bg-dark-700 rounded-full h-3 mb-2">
            <div class="h-3 rounded-full transition-all duration-500"
              :class="disk.used_percent > 90 ? 'bg-red-500' : disk.used_percent > 75 ? 'bg-yellow-500' : 'bg-primary-500'"
              :style="{ width: disk.used_percent + '%' }"></div>
          </div>
          <div class="flex justify-between text-xs text-gray-400">
            <span>{{ formatBytes(disk.used_bytes) }} utilisés</span>
            <span>{{ disk.used_percent?.toFixed(1) }}%</span>
            <span>{{ formatBytes(disk.total_bytes) }} total</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Docker Containers -->
    <div v-if="containers.length" class="card mb-8">
      <h3 class="text-lg font-semibold mb-4">Conteneurs Docker ({{ containers.length }})</h3>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-gray-400 border-b border-dark-700">
              <th class="text-left py-3 px-4">Nom</th>
              <th class="text-left py-3 px-4">Image</th>
              <th class="text-left py-3 px-4">Tag</th>
              <th class="text-left py-3 px-4">État</th>
              <th class="text-left py-3 px-4">Status</th>
              <th class="text-left py-3 px-4">Ports</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in containers" :key="c.id" class="border-b border-dark-700/50">
              <td class="py-3 px-4 font-medium">{{ c.name }}</td>
              <td class="py-3 px-4 text-gray-400">{{ c.image }}</td>
              <td class="py-3 px-4"><code class="text-xs bg-dark-700 px-2 py-1 rounded">{{ c.image_tag }}</code></td>
              <td class="py-3 px-4">
                <span :class="c.state === 'running' ? 'badge-running' : 'badge-stopped'">{{ c.state }}</span>
              </td>
              <td class="py-3 px-4 text-gray-400 text-xs">{{ c.status }}</td>
              <td class="py-3 px-4 text-gray-400 text-xs font-mono">{{ c.ports || '-' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- APT Status -->
    <div v-if="aptStatus" class="card">
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-lg font-semibold">APT - Mises à jour système</h3>
        <div class="flex gap-2">
          <button @click="sendAptCmd('update')" class="btn-secondary text-sm">apt update</button>
          <button @click="sendAptCmd('upgrade')" class="btn-primary text-sm">apt upgrade</button>
        </div>
      </div>
      <div class="grid grid-cols-3 gap-4 mb-4">
        <div class="bg-dark-900 rounded-lg p-4 text-center">
          <div class="text-2xl font-bold" :class="aptStatus.pending_packages > 0 ? 'text-yellow-400' : 'text-emerald-400'">
            {{ aptStatus.pending_packages }}
          </div>
          <div class="text-gray-400 text-sm">Paquets en attente</div>
        </div>
        <div class="bg-dark-900 rounded-lg p-4 text-center">
          <div class="text-2xl font-bold text-red-400">{{ aptStatus.security_updates }}</div>
          <div class="text-gray-400 text-sm">Mises à jour sécurité</div>
        </div>
        <div class="bg-dark-900 rounded-lg p-4 text-center">
          <div class="text-sm font-medium text-gray-300">{{ formatDate(aptStatus.last_update) }}</div>
          <div class="text-gray-400 text-sm">Dernier apt update</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { Line } from 'vue-chartjs'
import { Chart as ChartJS, LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip } from 'chart.js'
import apiClient from '../api'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/fr'

ChartJS.register(LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip)
dayjs.extend(relativeTime)
dayjs.locale('fr')

const route = useRoute()
const hostId = route.params.id

const host = ref(null)
const metrics = ref(null)
const containers = ref([])
const aptStatus = ref(null)
const metricsHistory = ref([])
const chartHours = ref(24)
const cpuChartData = ref(null)
const memChartData = ref(null)
let refreshInterval = null

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: { legend: { display: false } },
  scales: {
    x: { display: true, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', maxTicksLimit: 10 } },
    y: { display: true, min: 0, max: 100, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280' } },
  },
  elements: { point: { radius: 0 }, line: { tension: 0.3 } },
}

async function fetchData() {
  try {
    const res = await apiClient.getHostDashboard(hostId)
    host.value = res.data.host
    metrics.value = res.data.metrics
    containers.value = res.data.containers || []
    aptStatus.value = res.data.apt_status
  } catch (e) {
    console.error('Failed to fetch host data:', e)
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
  const labels = metricsHistory.value.map(m => dayjs(m.timestamp).format('HH:mm'))
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

async function sendAptCmd(command) {
  if (!confirm(`Exécuter 'apt ${command}' sur ${host.value?.hostname} ?`)) return
  try {
    await apiClient.sendAptCommand([hostId], command)
    alert(`Commande 'apt ${command}' envoyée. L'agent l'exécutera au prochain rapport.`)
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
  return dayjs(date).fromNow()
}

function cpuColor(pct) {
  if (!pct) return 'text-gray-300'
  if (pct > 90) return 'text-red-400'
  if (pct > 70) return 'text-yellow-400'
  return 'text-emerald-400'
}

function memColor(pct) {
  if (!pct) return 'text-gray-300'
  if (pct > 90) return 'text-red-400'
  if (pct > 75) return 'text-yellow-400'
  return 'text-emerald-400'
}

onMounted(() => {
  fetchData()
  loadHistory(24)
  refreshInterval = setInterval(fetchData, 30000)
})

onUnmounted(() => {
  if (refreshInterval) clearInterval(refreshInterval)
})
</script>
