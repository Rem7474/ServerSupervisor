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
              {{ `${(metrics.cpu_temperature ?? 0).toFixed(1)}°C` }}
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
          <div class="d-flex align-items-center gap-2">
            <span
              v-if="historyLoading"
              class="spinner-border spinner-border-sm text-muted"
            />
            <div class="btn-group btn-group-sm">
              <button
                v-for="opt in timeRangeOptions"
                :key="opt.hours"
                type="button"
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
          style="height: 12rem;"
        >
          <ApexChart
            v-if="cpuSeries"
            type="area"
            height="100%"
            :options="cpuChartOptions"
            :series="cpuSeries"
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
          <ApexChart
            v-if="memSeries"
            type="area"
            height="100%"
            :options="memChartOptions"
            :series="memSeries"
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
import { computed, ref, shallowRef, defineAsyncComponent, onMounted, watch, toRef } from 'vue'
import type { ApexOptions } from 'apexcharts'
import MetricsSourceBadge from '../common/MetricsSourceBadge.vue'
import { fetchMetricsHistory, type MetricsHistoryPoint } from '../../composables/useHostMetricsHistory'
import dayjs from '../../utils/dayjs'
import { getApexChartPalette } from '../../utils/apexChartTheme'
import { clampTimestamp, getMinPointTimestamp, getMaxPointTimestamp } from '../../utils/chartTimeAxis'

interface MetricsData {
  cpu_cores?: number
  cpu_usage_percent?: number
  cpu_model?: string
  cpu_temperature?: number
  memory_percent?: number
  memory_used?: number
  memory_total?: number
  uptime?: number
  load_avg_1?: number
  load_avg_5?: number
  load_avg_15?: number
}

type HistoryPoint = MetricsHistoryPoint

interface ChartPoint { x: number; y: number }
interface MemChartPoint extends ChartPoint { memory_used?: number; memory_total?: number }

const ApexChart = defineAsyncComponent(() => import('vue3-apexcharts').then((m) => m.default))

const props = withDefaults(defineProps<{
  hostId: string
  metrics?: MetricsData | null
  metricsSource?: string
  proxmoxGuestId?: string | null
  refreshTick?: number
}>(), {
  metrics: null,
  metricsSource: 'agent',
  proxmoxGuestId: null,
  refreshTick: 0,
})

const chartHours = ref(24)
const historyLoading = ref(false)
const metricsHistory = ref<HistoryPoint[]>([])
const cpuPoints = ref<ChartPoint[]>([])
const memPoints = ref<MemChartPoint[]>([])
const cpuSeries = shallowRef<{ data: ChartPoint[] }[] | null>(null)
const memSeries = shallowRef<{ data: MemChartPoint[] }[] | null>(null)

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

function tooltipHtml(palette: ReturnType<typeof getApexChartPalette>, title: string, body: string): string {
  return `<div style="background:${palette.tooltipBackground};color:${palette.tooltipText};border:1px solid ${palette.tooltipBorder};border-radius:4px;padding:8px 10px;font-size:12px;">`
    + `<div style="font-weight:600;margin-bottom:2px;">${title}</div>`
    + `<div>${body}</div>`
    + '</div>'
}

const cpuChartOptions = computed((): ApexOptions => {
  const palette = getApexChartPalette()
  return {
    chart: { type: 'area', toolbar: { show: false }, zoom: { enabled: false }, animations: { enabled: false }, parentHeightOffset: 0 },
    colors: [palette.cpu],
    fill: { type: 'solid', opacity: 0.1 },
    stroke: { curve: 'smooth', width: 2 },
    markers: { size: 0, hover: { size: 5 } },
    dataLabels: { enabled: false },
    legend: { show: false },
    grid: { borderColor: palette.grid },
    xaxis: {
      type: 'datetime',
      min: getMinPointTimestamp(cpuPoints.value),
      max: getMaxPointTimestamp(cpuPoints.value),
      labels: { style: { colors: palette.tickText }, formatter: (v: string) => formatChartTime(Number(v)) },
      axisBorder: { show: false },
      axisTicks: { show: false },
      tooltip: { enabled: false },
    },
    yaxis: { min: 0, max: 100, labels: { style: { colors: palette.tickText }, formatter: (v: number) => `${v.toFixed(0)}%` } },
    tooltip: {
      custom: ({ dataPointIndex }) => {
        const p = cpuPoints.value[dataPointIndex]
        if (!p) return ''
        return tooltipHtml(palette, formatChartTime(p.x), `${p.y.toFixed(1)}%`)
      },
    },
  }
})

const memChartOptions = computed((): ApexOptions => {
  const palette = getApexChartPalette()
  return {
    chart: { type: 'area', toolbar: { show: false }, zoom: { enabled: false }, animations: { enabled: false }, parentHeightOffset: 0 },
    colors: [palette.ram],
    fill: { type: 'solid', opacity: 0.1 },
    stroke: { curve: 'smooth', width: 2 },
    markers: { size: 0, hover: { size: 5 } },
    dataLabels: { enabled: false },
    legend: { show: false },
    grid: { borderColor: palette.grid },
    xaxis: {
      type: 'datetime',
      min: getMinPointTimestamp(memPoints.value),
      max: getMaxPointTimestamp(memPoints.value),
      labels: { style: { colors: palette.tickText }, formatter: (v: string) => formatChartTime(Number(v)) },
      axisBorder: { show: false },
      axisTicks: { show: false },
      tooltip: { enabled: false },
    },
    yaxis: { min: 0, max: 100, labels: { style: { colors: palette.tickText }, formatter: (v: number) => `${v.toFixed(0)}%` } },
    tooltip: {
      custom: ({ dataPointIndex }) => {
        const p = memPoints.value[dataPointIndex]
        if (!p) return ''
        const pct = p.y.toFixed(1)
        const body = p.memory_used && p.memory_total
          ? `${pct}%  (${formatBytes(p.memory_used)} / ${formatBytes(p.memory_total)})`
          : `${pct}%`
        return tooltipHtml(palette, formatChartTime(p.x), body)
      },
    },
  }
})

function formatBytes(bytes: number | undefined): string {
  if (!bytes) return '0 B'
  const k = 1024
  const sizes = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function formatUptime(seconds: number | undefined): string {
  if (!seconds) return 'N/A'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  if (days > 0) return `${days}j ${hours}h`
  const mins = Math.floor((seconds % 3600) / 60)
  return `${hours}h ${mins}m`
}

function cpuColor(pct: number | undefined): string {
  if (!pct) return 'text-secondary'
  if (pct > 90) return 'text-danger'
  if (pct > 70) return 'text-warning'
  return 'text-success'
}

function memColor(pct: number | undefined): string {
  if (!pct) return 'text-secondary'
  if (pct > 90) return 'text-danger'
  if (pct > 75) return 'text-warning'
  return 'text-success'
}

function tempColor(temp: number | undefined): string {
  if (!temp) return 'text-secondary'
  if (temp >= 85) return 'text-danger'
  if (temp >= 70) return 'text-warning'
  return 'text-success'
}

function formatChartTime(timestamp: number | string | undefined): string {
  if (!timestamp) return ''
  const date = dayjs(timestamp)
  if (!date.isValid()) return ''
  if (chartHours.value <= 24) return date.format('HH:mm')
  if (chartHours.value <= 720) return date.format('DD/MM HH:mm')
  return date.format('DD/MM')
}

function toChartPoint(metric: HistoryPoint, field: keyof HistoryPoint): { x: number; y: number } | null {
  const timestamp = clampTimestamp(dayjs(metric.timestamp).valueOf())
  const value = metric[field] as number | undefined
  if (!Number.isFinite(timestamp) || value == null) return null
  return { x: timestamp, y: value }
}

let refreshTimer: ReturnType<typeof setTimeout> | null = null
watch(() => props.refreshTick, () => {
  if (refreshTimer) clearTimeout(refreshTimer)
  refreshTimer = setTimeout(() => {
    refreshTimer = null
    loadHistory(chartHours.value)
  }, 400)
})

async function loadHistory(hours: number): Promise<void> {
  chartHours.value = hours
  historyLoading.value = true
  try {
    const history = await fetchMetricsHistory(props.hostId, hours, props.metricsSource, props.proxmoxGuestId)
    metricsHistory.value = history
    if (!history.length) { cpuSeries.value = null; memSeries.value = null; return }
    buildCharts()
  } catch (e: unknown) {
    const err = e as { response?: { data?: unknown }; message?: string }
    console.error(`Failed to fetch metrics history (${hours}h):`, err?.response?.data || err?.message)
  } finally {
    historyLoading.value = false
  }
}

function buildCharts(): void {
  cpuPoints.value = metricsHistory.value
    .map(m => toChartPoint(m, 'cpu_usage_percent'))
    .filter((p): p is ChartPoint => p != null)

  memPoints.value = metricsHistory.value
    .map((m): MemChartPoint | null => {
      const base = toChartPoint(m, 'memory_percent')
      if (!base) return null
      return { ...base, memory_used: m.memory_used, memory_total: m.memory_total }
    })
    .filter((p): p is MemChartPoint => p != null)

  cpuSeries.value = [{ data: cpuPoints.value }]
  memSeries.value = [{ data: memPoints.value }]
}

// Reload chart when the metrics source changes (e.g. user switches agent ↔ proxmox).
watch(toRef(props, 'metricsSource'), () => loadHistory(chartHours.value))

onMounted(() => loadHistory(24))
</script>
