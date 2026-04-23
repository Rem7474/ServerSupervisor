<template>
  <!-- Metric cards -->
  <div
    v-if="metrics"
    class="row row-cards mb-4 g-3"
  >
    <div class="col-6 col-lg-3">
      <div class="card card-sm h-100">
        <div class="card-body">
          <div class="subheader d-flex align-items-center gap-2">
            CPU ({{ metrics.cpu_cores }} CORES)
            <MetricsSourceBadge
              v-if="metricsSource === 'proxmox'"
              source="proxmox"
            />
          </div>
          <div
            class="h2 mb-0"
            :class="cpuColor(metrics.cpu_usage_percent)"
          >
            {{ metrics.cpu_usage_percent?.toFixed(1) }}%
          </div>
          <div
            v-if="metricsSource !== 'proxmox'"
            class="text-secondary small"
          >
            {{ metrics.cpu_model }}
          </div>
          <div
            v-if="hasCpuTemp"
            class="mt-2 pt-2 border-top d-flex align-items-center gap-2"
          >
            <span class="text-muted small">Temp:</span>
            <span :class="['text-sm fw-semibold', tempColor(metrics.cpu_temperature)]">
              {{ `${metrics.cpu_temperature.toFixed(1)}°C` }}
            </span>
          </div>
        </div>
      </div>
    </div>
    <div class="col-6 col-lg-3">
      <div class="card card-sm h-100">
        <div class="card-body">
          <div class="subheader d-flex align-items-center gap-2">
            RAM
            <MetricsSourceBadge
              v-if="metricsSource === 'proxmox'"
              source="proxmox"
            />
          </div>
          <div
            class="h2 mb-0"
            :class="memColor(metrics.memory_percent)"
          >
            {{ metrics.memory_percent?.toFixed(1) }}%
          </div>
          <div class="text-secondary small">
            {{ formatBytes(metrics.memory_used) }} / {{ formatBytes(metrics.memory_total) }}
          </div>
        </div>
      </div>
    </div>
    <div class="col-6 col-lg-3">
      <div class="card card-sm h-100">
        <div class="card-body">
          <div class="subheader">
            UPTIME
          </div>
          <div class="h2 mb-0 text-primary">
            {{ formatUptime(metrics.uptime) }}
          </div>
        </div>
      </div>
    </div>
    <div class="col-6 col-lg-3">
      <div class="card card-sm h-100">
        <div class="card-body">
          <div class="subheader">
            LOAD AVG
          </div>
          <div class="h2 mb-0">
            {{ metrics.load_avg_1?.toFixed(2) }}
          </div>
          <div class="text-secondary small">
            {{ metrics.load_avg_5?.toFixed(2) }} / {{ metrics.load_avg_15?.toFixed(2) }}
          </div>
        </div>
      </div>
    </div>
  </div>

  <!-- CPU / Memory charts -->
  <div class="row row-cards mb-4">
    <div class="col-lg-6">
      <div class="card">
        <div class="card-header d-flex align-items-center justify-content-between">
          <h3 class="card-title">
            CPU
          </h3>
          <div class="btn-group btn-group-sm">
            <button
              v-for="opt in timeRangeOptions"
              :key="opt.hours"
              :class="chartHours === opt.hours ? 'btn btn-primary' : 'btn btn-outline-secondary'"
              @click="loadHistory(opt.hours)"
            >
              {{ opt.label }}
            </button>
          </div>
        </div>
        <div
          class="card-body"
          style="height: 12rem;"
        >
          <Line
            v-if="cpuChartData"
            :data="cpuChartData"
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
    <div class="col-lg-6">
      <div class="card">
        <div class="card-header">
          <h3 class="card-title">
            Mémoire
          </h3>
        </div>
        <div
          class="card-body"
          style="height: 12rem;"
        >
          <Line
            v-if="memChartData"
            :data="memChartData"
            :options="memChartOptions"
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

<script setup>
import { computed, ref, shallowRef, defineAsyncComponent, onMounted, watch, toRef } from 'vue'
import apiClient from '../api'
import MetricsSourceBadge from './common/MetricsSourceBadge.vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import utc from 'dayjs/plugin/utc'
import 'dayjs/locale/fr'

dayjs.extend(relativeTime)
dayjs.extend(utc)
dayjs.locale('fr')

const Line = defineAsyncComponent(async () => {
  const [{ Line }, { Chart: ChartJS, LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip }] = await Promise.all([
    import('vue-chartjs'),
    import('chart.js'),
  ])
  ChartJS.register(LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip)
  return Line
})

const props = defineProps({
  hostId: { type: String, required: true },
  metrics: { type: Object, default: null },
  metricsSource: { type: String, default: 'agent' }, // 'agent' | 'proxmox'
  proxmoxGuestId: { type: String, default: null },
})

const chartHours = ref(24)
const metricsHistory = ref([])
const cpuChartData = shallowRef(null)
const memChartData = shallowRef(null)

const hasCpuTemp = computed(() => Number(props.metrics?.cpu_temperature) > 0)

const timeRangeOptions = [
  { hours: 1,    label: '1h' },
  { hours: 6,    label: '6h' },
  { hours: 24,   label: '24h' },
  { hours: 168,  label: '7d' },
  { hours: 720,  label: '30d' },
  { hours: 2160, label: '90d' },
  { hours: 8760, label: '1y' },
]

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      enabled: true, mode: 'index', intersect: false,
      backgroundColor: 'rgba(0,0,0,0.8)', titleColor: '#fff', bodyColor: '#fff',
      borderColor: '#555', borderWidth: 1, padding: 10, displayColors: false,
      callbacks: {
        title: (items) => formatChartTime(items[0]?.parsed?.x),
        label: (ctx) => `${ctx.parsed.y.toFixed(1)}%`,
      },
    },
  },
  scales: {
    x: {
      type: 'linear',
      display: true,
      grid: { color: 'rgba(255,255,255,0.05)' },
      ticks: {
        color: '#6b7280',
        maxTicksLimit: 10,
        callback: (value) => formatChartTime(Number(value)),
      },
    },
    y: { display: true, min: 0, max: 100, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280' } },
  },
  elements: { point: { radius: 0, hitRadius: 10, hoverRadius: 5 }, line: { tension: 0.3 } },
  interaction: { mode: 'nearest', axis: 'x', intersect: false },
}

const memChartOptions = {
  ...chartOptions,
  plugins: {
    ...chartOptions.plugins,
    tooltip: {
      ...chartOptions.plugins.tooltip,
      callbacks: {
        title: (items) => items[0]?.label || '',
        label: (ctx) => {
          const pct = ctx.parsed.y.toFixed(1)
          const m = metricsHistory.value[ctx.dataIndex]
          if (m?.memory_used && m?.memory_total) {
            return `${pct}%  (${formatBytes(m.memory_used)} / ${formatBytes(m.memory_total)})`
          }
          return `${pct}%`
        },
      },
    },
  },
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

function tempColor(temp) {
  if (!temp) return 'text-secondary'
  if (temp >= 85) return 'text-red'
  if (temp >= 70) return 'text-yellow'
  return 'text-green'
}

function formatChartTime(timestamp) {
  if (!timestamp) return ''
  const date = dayjs(timestamp)
  if (!date.isValid()) return ''
  if (chartHours.value <= 24) return date.format('HH:mm')
  if (chartHours.value <= 720) return date.format('DD/MM HH:mm')
  return date.format('DD/MM')
}

function toChartPoint(metric, field) {
  const timestamp = dayjs(metric.timestamp).valueOf()
  const value = metric[field]
  if (!Number.isFinite(timestamp) || value == null) return null
  return { x: timestamp, y: value }
}

async function loadHistory(hours) {
  chartHours.value = hours
  try {
    let history
    if (props.metricsSource === 'proxmox' && props.proxmoxGuestId) {
      // Proxmox-sourced: fetch per-guest time-series from the Proxmox poller snapshots.
      // Response shape: [{timestamp, cpu_avg, memory_avg}] — remap to chart field names.
      const bucketMinutes = hours <= 1 ? 1 : hours <= 6 ? 2 : hours <= 24 ? 5 : hours <= 168 ? 30 : 60
      const res = await apiClient.getProxmoxGuestMetrics(props.proxmoxGuestId, hours, bucketMinutes)
      const raw = Array.isArray(res.data) ? res.data : []
      history = raw.map(p => ({
        timestamp: p.timestamp,
        cpu_usage_percent: p.cpu_avg,
        memory_percent: p.memory_avg,
      }))
    } else if (hours > 24) {
      const res = await apiClient.getMetricsAggregated(props.hostId, hours)
      history = Array.isArray(res.data?.metrics) ? res.data.metrics : []
    } else {
      const res = await apiClient.getMetricsHistory(props.hostId, hours)
      history = Array.isArray(res.data) ? res.data : []
    }
    metricsHistory.value = history
    if (!history.length) { cpuChartData.value = null; memChartData.value = null; return }
    buildCharts()
  } catch (e) {
    console.error(`Failed to fetch metrics history (${hours}h):`, e.response?.data || e.message)
  }
}

function buildCharts() {
  const cpuPoints = metricsHistory.value
    .map(m => toChartPoint(m, 'cpu_usage_percent'))
    .filter(Boolean)
  const memPoints = metricsHistory.value
    .map(m => toChartPoint(m, 'memory_percent'))
    .filter(Boolean)
  cpuChartData.value = {
    datasets: [{
      data: cpuPoints,
      borderColor: '#3b82f6',
      backgroundColor: 'rgba(59,130,246,0.1)',
      fill: true,
      tension: 0.3,
      spanGaps: false,
    }],
  }
  memChartData.value = {
    datasets: [{
      data: memPoints,
      borderColor: '#10b981',
      backgroundColor: 'rgba(16,185,129,0.1)',
      fill: true,
      tension: 0.3,
      spanGaps: false,
    }],
  }
}

// Reload chart when the metrics source changes (e.g. user switches agent ↔ proxmox).
watch(toRef(props, 'metricsSource'), () => loadHistory(chartHours.value))

onMounted(() => loadHistory(24))
</script>
