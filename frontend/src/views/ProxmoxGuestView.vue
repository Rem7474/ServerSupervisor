<template>
  <div>
    <div
      v-if="loading"
      class="text-center py-5 text-muted"
    >
      Chargement...
    </div>
    <div
      v-else-if="error"
      class="alert alert-danger"
    >
      {{ error }}
    </div>
    <div v-else-if="guest">
      <div class="page-header mb-4">
        <div class="page-pretitle">
          <router-link
            to="/"
            class="text-decoration-none"
          >
            Dashboard
          </router-link>
          <span class="text-muted mx-1">/</span>
          <router-link
            to="/proxmox"
            class="text-decoration-none"
          >
            Proxmox VE
          </router-link>
          <span class="text-muted mx-1">/</span>
          <router-link
            :to="`/proxmox/nodes/${guestNodeId}`"
            class="text-decoration-none"
          >
            {{ guest.node_name }}
          </router-link>
          <span class="text-muted mx-1">/</span>
          <span>{{ guest.guest_type.toUpperCase() }} {{ guest.vmid }}</span>
        </div>
        <div class="d-flex align-items-center gap-3 flex-wrap">
          <h2 class="page-title mb-0">
            {{ guest.name || `${guest.guest_type.toUpperCase()} ${guest.vmid}` }}
          </h2>
          <span :class="statusBadgeClass(guest.status)">{{ guest.status }}</span>
          <span class="badge bg-azure-lt text-azure">{{ guest.guest_type.toUpperCase() }}</span>
          <template v-if="guestLink?.host_id">
            <MetricsSourceBadge source="proxmox" />
            <router-link
              :to="`/hosts/${guestLink.host_id}`"
              class="ms-1"
            >
              {{ guestLink.host_hostname || guestLink.host_name }}
            </router-link>
          </template>
        </div>
        <div class="text-secondary">
          Nœud {{ guest.node_name }} · VMID {{ guest.vmid }}
        </div>
      </div>

      <div class="row row-cards mb-4">
        <div class="col-6 col-lg-3">
          <div class="card card-sm h-100">
            <div class="card-body">
              <div class="subheader">
                CPU alloué
              </div>
              <div class="h2 mt-2 mb-0">
                {{ guest.cpu_alloc }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-6 col-lg-3">
          <div class="card card-sm h-100">
            <div class="card-body">
              <div class="subheader">
                CPU utilisé
              </div>
              <div class="h2 mt-2 mb-0">
                {{ (guest.cpu_usage * 100).toFixed(1) }}%
              </div>
            </div>
          </div>
        </div>
        <div class="col-6 col-lg-3">
          <div class="card card-sm h-100">
            <div class="card-body">
              <div class="subheader">
                RAM
              </div>
              <div class="h2 mt-2 mb-0">
                {{ formatBytes(guest.mem_usage) }}
              </div>
              <div class="text-secondary small">
                / {{ formatBytes(guest.mem_alloc) }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-6 col-lg-3">
          <div class="card card-sm h-100">
            <div class="card-body">
              <div class="subheader">
                Disque
              </div>
              <div class="h2 mt-2 mb-0">
                {{ formatBytes(guest.disk_alloc) }}
              </div>
              <div class="text-secondary small">
                Uptime {{ formatUptime(guest.uptime) }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-header d-flex align-items-center justify-content-between gap-2 flex-wrap">
          <h3 class="card-title mb-0">
            Historique CPU / RAM
          </h3>
          <div
            class="btn-group btn-group-sm guest-range-group"
            role="group"
            aria-label="Plage temporelle"
          >
            <button
              v-for="h in [1, 6, 24, 168, 720]"
              :key="h"
              :class="hours === h ? 'btn btn-primary' : 'btn btn-outline-secondary'"
              @click="changeRange(h)"
            >
              {{ h >= 24 ? (h / 24) + 'j' : h + 'h' }}
            </button>
          </div>
        </div>
        <div
          class="card-body"
          style="height: 14rem;"
        >
          <div
            v-if="summaryLoading"
            class="h-100 d-flex align-items-center justify-content-center"
          >
            <div
              class="spinner-border text-secondary"
              role="status"
            />
          </div>
          <Line
            v-else-if="chartData"
            :data="chartData"
            :options="chartOptions"
            class="h-100"
          />
          <div
            v-else
            class="h-100 d-flex align-items-center justify-content-center text-secondary"
          >
            Aucune donnée
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import dayjs from 'dayjs'
import api from '../api'
import MetricsSourceBadge from '../components/common/MetricsSourceBadge.vue'

interface ProxmoxGuest {
  id: string
  node_name: string
  guest_type: string
  vmid: number
  name: string
  status: string
  cpu_alloc: number
  cpu_usage: number
  mem_alloc: number
  mem_usage: number
  disk_alloc: number
  uptime: number
}

const route = useRoute()
const guest = ref<ProxmoxGuest | null>(null)
const guestLink = ref<any>(null)
const loading = ref(true)
const summaryLoading = ref(false)
const error = ref('')
const hours = ref(24)
const chartData = ref<any>(null)

const Line = defineAsyncComponent(async () => {
  const [{ Line }, { Chart: ChartJS, LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip, Legend }] = await Promise.all([
    import('vue-chartjs'),
    import('chart.js'),
  ])
  ChartJS.register(LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip, Legend)
  return Line
})

const chartOptions: any = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: true, position: 'top', labels: { color: '#6b7280', boxWidth: 12, padding: 12 } },
    tooltip: {
      enabled: true,
      mode: 'index',
      intersect: false,
      backgroundColor: 'rgba(0,0,0,0.8)',
      titleColor: '#fff',
      bodyColor: '#fff',
      borderColor: '#555',
      borderWidth: 1,
      padding: 10,
      callbacks: {
        label: (ctx: any) => `${ctx.dataset.label}: ${ctx.parsed.y.toFixed(1)}%`,
      },
    },
  },
  scales: {
    x: { display: true, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', maxTicksLimit: 10 } },
    y: { display: true, min: 0, max: 100, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280' } },
  },
  elements: { point: { radius: 0, hitRadius: 10, hoverRadius: 5 }, line: { tension: 0.3 } },
  interaction: { mode: 'nearest' as const, axis: 'x' as const, intersect: false },
}))

const guestNodeId = computed(() => route.query.nodeId || '')

function bucketMinutesFor(inputHours: number): number {
  if (inputHours <= 6) return 1
  if (inputHours <= 24) return 5
  if (inputHours <= 168) return 15
  return 60
}

async function loadGuest(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const res = await api.getProxmoxGuests()
    const list = Array.isArray(res.data) ? res.data : []
    const found = list.find((g: ProxmoxGuest) => g.id === route.params.id)
    if (!found) {
      error.value = 'Guest introuvable'
      return
    }
    guest.value = found
    const linkRes = await api.getProxmoxGuestLink(found.id)
    guestLink.value = linkRes.data
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Erreur lors du chargement du guest Proxmox.'
  } finally {
    loading.value = false
  }
}

async function loadGuestSummary(): Promise<void> {
  if (!guest.value) return
  summaryLoading.value = true
  try {
    const bucketMinutes = bucketMinutesFor(hours.value)
    const res = await api.getProxmoxGuestMetrics(guest.value.id, hours.value, bucketMinutes)
    const points = Array.isArray(res.data) ? res.data : []
    if (!points.length) {
      chartData.value = null
      return
    }
    const labels = points.map((p: any) =>
      hours.value >= 24 ? dayjs(p.timestamp).format('DD/MM HH:mm') : dayjs(p.timestamp).format('HH:mm')
    )
    chartData.value = {
      labels,
      datasets: [
        {
          label: 'CPU %',
          data: points.map((p: any) => Number(p.cpu_avg ?? 0)),
          borderColor: '#3b82f6',
          backgroundColor: 'rgba(59,130,246,0.10)',
          fill: true,
        },
        {
          label: 'RAM %',
          data: points.map((p: any) => Number(p.memory_avg ?? 0)),
          borderColor: '#10b981',
          backgroundColor: 'rgba(16,185,129,0.10)',
          fill: true,
        },
      ],
    }
  } catch {
    chartData.value = null
  } finally {
    summaryLoading.value = false
  }
}

function changeRange(value: number): void {
  hours.value = value
  loadGuestSummary()
}

function formatBytes(bytes: number): string {
  if (!bytes) return '0 B'
  const units = ['B', 'Ko', 'Mo', 'Go', 'To']
  let i = 0
  let v = bytes
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

function formatUptime(seconds: number): string {
  if (!seconds) return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}j ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function statusBadgeClass(status: string): string {
  const map: Record<string, string> = {
    running: 'badge bg-success-lt text-success',
    stopped: 'badge bg-secondary-lt text-secondary',
    paused: 'badge bg-warning-lt text-warning',
  }
  return map[status] || 'badge bg-secondary-lt text-secondary'
}

onMounted(async () => {
  await loadGuest()
  await loadGuestSummary()
})
</script>

<style scoped>
.guest-range-group {
  flex-wrap: wrap;
}

@media (max-width: 768px) {
  .guest-range-group {
    width: 100%;
  }

  .guest-range-group .btn {
    flex: 1 1 0;
    min-width: 56px;
  }
}
</style>

