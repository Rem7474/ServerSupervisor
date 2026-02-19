<template>
  <div>
    <div class="flex items-center justify-between mb-8">
      <div>
        <h1 class="text-2xl font-bold">Dashboard</h1>
        <p class="text-gray-400 mt-1">Vue d'ensemble de l'infrastructure</p>
      </div>
      <router-link to="/hosts/new" class="btn-primary flex items-center gap-2">
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"/>
        </svg>
        Ajouter un hôte
      </router-link>
    </div>

    <!-- Summary Cards -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
      <div class="stat-card">
        <div class="text-3xl font-bold text-primary-400">{{ hosts.length }}</div>
        <div class="text-gray-400 mt-1">Hôtes</div>
      </div>
      <div class="stat-card">
        <div class="text-3xl font-bold text-emerald-400">{{ onlineCount }}</div>
        <div class="text-gray-400 mt-1">En ligne</div>
      </div>
      <div class="stat-card">
        <div class="text-3xl font-bold text-red-400">{{ offlineCount }}</div>
        <div class="text-gray-400 mt-1">Hors ligne</div>
      </div>
      <div class="stat-card">
        <div class="text-3xl font-bold text-yellow-400">{{ outdatedVersions }}</div>
        <div class="text-gray-400 mt-1">Mises à jour dispo</div>
      </div>
    </div>

    <!-- Hosts Grid -->
    <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6 mb-8">
      <router-link v-for="host in hosts" :key="host.id" :to="`/hosts/${host.id}`"
        class="card hover:border-primary-500/50 transition-colors cursor-pointer">
        <div class="flex items-center justify-between mb-4">
          <div>
            <h3 class="text-lg font-semibold">{{ host.name }}</h3>
            <p class="text-xs text-gray-500">{{ host.hostname || 'Non connecté' }}</p>
          </div>
          <span :class="host.status === 'online' ? 'badge-online' : 'badge-offline'">
            {{ host.status === 'online' ? 'En ligne' : 'Hors ligne' }}
          </span>
        </div>

        <div class="space-y-3">
          <div class="flex justify-between text-sm">
            <span class="text-gray-400">IP</span>
            <span>{{ host.ip_address }}</span>
          </div>
          <div class="flex justify-between text-sm">
            <span class="text-gray-400">OS</span>
            <span>{{ host.os || 'N/A' }}</span>
          </div>
          <div class="flex justify-between text-sm">
            <span class="text-gray-400">Dernière activité</span>
            <span>{{ formatDate(host.last_seen) }}</span>
          </div>
        </div>

        <!-- Quick metrics if available -->
        <div v-if="hostMetrics[host.id]" class="mt-4 pt-4 border-t border-dark-700">
          <div class="grid grid-cols-3 gap-3">
            <div class="text-center">
              <div class="text-sm font-medium" :class="cpuColor(hostMetrics[host.id].cpu_usage_percent)">
                {{ hostMetrics[host.id].cpu_usage_percent?.toFixed(1) }}%
              </div>
              <div class="text-xs text-gray-500">CPU</div>
            </div>
            <div class="text-center">
              <div class="text-sm font-medium" :class="memColor(hostMetrics[host.id].memory_percent)">
                {{ hostMetrics[host.id].memory_percent?.toFixed(1) }}%
              </div>
              <div class="text-xs text-gray-500">RAM</div>
            </div>
            <div class="text-center">
              <div class="text-sm font-medium text-gray-300">
                {{ formatUptime(hostMetrics[host.id].uptime) }}
              </div>
              <div class="text-xs text-gray-500">Uptime</div>
            </div>
          </div>
        </div>
      </router-link>
    </div>

    <!-- Last Updates Section -->
    <div v-if="versionComparisons.length > 0" class="card">
      <h2 class="text-lg font-semibold mb-4 flex items-center gap-2">
        <svg class="w-5 h-5 text-yellow-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"/>
        </svg>
        Versions & Mises à jour
      </h2>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-gray-400 border-b border-dark-700">
              <th class="text-left py-3 px-4">Image</th>
              <th class="text-left py-3 px-4">Hôte</th>
              <th class="text-left py-3 px-4">En cours</th>
              <th class="text-left py-3 px-4">Dernière version</th>
              <th class="text-left py-3 px-4">Statut</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="v in versionComparisons" :key="v.docker_image + v.host_id" class="border-b border-dark-700/50">
              <td class="py-3 px-4 font-medium">{{ v.docker_image }}</td>
              <td class="py-3 px-4 text-gray-400">{{ v.hostname }}</td>
              <td class="py-3 px-4"><code class="text-xs bg-dark-700 px-2 py-1 rounded">{{ v.running_version }}</code></td>
              <td class="py-3 px-4">
                <a v-if="v.release_url" :href="v.release_url" target="_blank" class="text-primary-400 hover:underline">
                  {{ v.latest_version }}
                </a>
                <span v-else>{{ v.latest_version }}</span>
              </td>
              <td class="py-3 px-4">
                <span v-if="v.is_up_to_date" class="badge-online">À jour</span>
                <span v-else class="badge-warning">Mise à jour disponible</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="flex justify-center py-12">
      <div class="animate-spin rounded-full h-10 w-10 border-b-2 border-primary-400"></div>
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
let refreshInterval = null

const onlineCount = computed(() => hosts.value.filter(h => h.status === 'online').length)
const offlineCount = computed(() => hosts.value.filter(h => h.status !== 'online').length)
const outdatedVersions = computed(() => versionComparisons.value.filter(v => !v.is_up_to_date).length)

async function fetchData() {
  try {
    const [hostsRes, versionsRes] = await Promise.all([
      apiClient.getHosts(),
      apiClient.getVersionComparisons().catch(() => ({ data: [] })),
    ])
    hosts.value = hostsRes.data
    versionComparisons.value = versionsRes.data || []

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
  refreshInterval = setInterval(fetchData, 30000)
})

onUnmounted(() => {
  if (refreshInterval) clearInterval(refreshInterval)
})
</script>
