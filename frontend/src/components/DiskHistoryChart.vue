<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between flex-wrap gap-2">
      <div class="d-flex align-items-center gap-3">
        <h3 class="card-title mb-0">
          Historique disques
        </h3>
        <select
          v-if="mounts.length > 1"
          v-model="selectedMount"
          class="form-select form-select-sm"
          style="width: auto;"
          @change="loadHistory(chartHours)"
        >
          <option
            v-for="m in mounts"
            :key="m"
            :value="m"
          >
            {{ m }}
          </option>
        </select>
        <span
          v-else-if="mounts.length === 1"
          class="text-muted small"
        >{{ mounts[0] }}</span>
      </div>
      <div class="d-flex align-items-center gap-2">
        <span
          v-if="fillPrediction"
          :class="['badge', fillPrediction.days <= 30 ? 'bg-danger' : 'bg-warning text-dark']"
          :title="`Basé sur la tendance des ${chartHours}h`"
        >
          Plein dans ~{{ fillPrediction.days }}j
        </span>
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
    </div>
    <div
      class="card-body"
      style="height: 13rem;"
    >
      <Line
        v-if="chartData"
        :data="chartData"
        :options="chartOptions"
        class="h-100"
      />
      <div
        v-else-if="loading"
        class="h-100 d-flex align-items-center justify-content-center text-secondary"
      >
        <div class="spinner-border spinner-border-sm me-2" />
        Chargement…
      </div>
      <div
        v-else
        class="h-100 d-flex align-items-center justify-content-center text-secondary"
      >
        Aucune donnée
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, shallowRef, watch, onMounted, computed, defineAsyncComponent } from 'vue'
import apiClient from '../api'
import dayjs from 'dayjs'
import utc from 'dayjs/plugin/utc'
import 'dayjs/locale/fr'

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
  mounts: { type: Array, default: () => [] },
  refreshTick: { type: Number, default: 0 },
})

const chartHours = ref(24)
const selectedMount = ref(props.mounts[0] ?? '')
const points = ref([])
const chartData = shallowRef(null)
const loading = ref(false)

const timeRangeOptions = [
  { hours: 1,    label: '1h' },
  { hours: 6,    label: '6h' },
  { hours: 24,   label: '24h' },
  { hours: 168,  label: '7j' },
  { hours: 720,  label: '30j' },
  { hours: 2160, label: '90j' },
  { hours: 8760, label: '1a' },
]

// Linear regression on {x: timestamp ms, y: used_gb}
const fillPrediction = computed(() => {
  if (points.value.length < 10) return null
  const data = points.value
  const n = data.length
  let sumX = 0, sumY = 0, sumXY = 0, sumX2 = 0
  for (const p of data) {
    sumX += p.x; sumY += p.y; sumXY += p.x * p.y; sumX2 += p.x * p.x
  }
  const slope = (n * sumXY - sumX * sumY) / (n * sumX2 - sumX * sumX)
  if (slope <= 0) return null
  const intercept = (sumY - slope * sumX) / n
  const sizeGB = points.value[points.value.length - 1]?.size_gb
  if (!sizeGB) return null
  const msLeft = (sizeGB - intercept) / slope
  const days = Math.round(msLeft / 86400000)
  if (days <= 0 || days > 365) return null
  return { days }
})

function formatChartTime(timestamp) {
  if (!timestamp) return ''
  const d = dayjs(timestamp)
  if (!d.isValid()) return ''
  if (chartHours.value <= 24) return d.format('HH:mm')
  if (chartHours.value <= 720) return d.format('DD/MM HH:mm')
  return d.format('DD/MM')
}

function clampTimestamp(timestampMs) {
  if (!Number.isFinite(timestampMs)) return NaN
  const now = Date.now()
  return Math.min(timestampMs, now)
}

function getMaxPointTimestamp(list) {
  let max = -Infinity
  for (const point of list || []) {
    if (Number.isFinite(point?.x) && point.x > max) max = point.x
  }
  if (!Number.isFinite(max)) return undefined
  return Math.min(Date.now(), max)
}

const chartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
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
      displayColors: false,
      callbacks: {
        title: (items) => formatChartTime(items[0]?.parsed?.x),
        label: (ctx) => {
          const p = points.value[ctx.dataIndex]
          if (p?.used_gb != null && p?.size_gb) {
            return `${ctx.parsed.y.toFixed(1)}%  (${p.used_gb.toFixed(1)} / ${p.size_gb.toFixed(1)} Go)`
          }
          return `${ctx.parsed.y.toFixed(1)}%`
        },
      },
    },
  },
  scales: {
    x: {
      type: 'linear',
      display: true,
      grid: { color: 'rgba(255,255,255,0.05)' },
      max: getMaxPointTimestamp(points.value),
      ticks: { color: '#6b7280', maxTicksLimit: 8, callback: (v) => formatChartTime(Number(v)) },
    },
    y: {
      display: true,
      min: 0,
      max: 100,
      grid: { color: 'rgba(255,255,255,0.05)' },
      ticks: { color: '#6b7280', callback: (v) => `${v}%` },
    },
  },
  elements: { point: { radius: 0, hitRadius: 10, hoverRadius: 4 }, line: { tension: 0.3 } },
  interaction: { mode: 'nearest', axis: 'x', intersect: false },
}))

async function loadHistory(hours) {
  if (!selectedMount.value) return
  chartHours.value = hours
  loading.value = true
  chartData.value = null
  try {
    const res = await apiClient.getDiskMetricsAggregated(props.hostId, selectedMount.value, hours)
    const raw = Array.isArray(res.data?.points) ? res.data.points : []
    points.value = raw.map(p => ({
      x: clampTimestamp(dayjs(p.timestamp).valueOf()),
      y: p.used_percent,
      used_gb: p.used_gb,
      size_gb: p.size_gb,
    })).filter(p => Number.isFinite(p.x) && p.y != null)

    if (!points.value.length) { chartData.value = null; return }

    chartData.value = {
      datasets: [{
        data: points.value,
        borderColor: '#f59e0b',
        backgroundColor: 'rgba(245,158,11,0.12)',
        fill: true,
        tension: 0.3,
        spanGaps: false,
      }],
    }
  } catch (e) {
    console.error('Failed to load disk history:', e.response?.data || e.message)
    chartData.value = null
  } finally {
    loading.value = false
  }
}

watch(() => props.mounts, (v) => {
  if (v.length && !selectedMount.value) {
    selectedMount.value = v[0]
    loadHistory(chartHours.value)
  }
}, { immediate: false })

let refreshTimer = null
watch(() => props.refreshTick, () => {
  if (refreshTimer) clearTimeout(refreshTimer)
  refreshTimer = setTimeout(() => {
    refreshTimer = null
    loadHistory(chartHours.value)
  }, 400)
})

onMounted(() => {
  if (props.mounts.length) {
    selectedMount.value = props.mounts[0]
    loadHistory(24)
  }
})
</script>
