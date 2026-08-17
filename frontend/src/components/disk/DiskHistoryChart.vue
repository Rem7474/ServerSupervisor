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
          :class="['badge', fillPrediction.days <= 30 ? 'bg-danger text-white' : 'bg-warning text-white']"
          :title="`Basé sur la tendance des ${chartHours}h`"
        >
          Plein dans ~{{ fillPrediction.days }}j
        </span>
        <span
          v-if="rangeLoading"
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
      style="height: 13rem;"
    >
      <ApexChart
        v-if="series && chartOptions"
        ref="chartRef"
        type="area"
        height="100%"
        :options="chartOptions"
        :series="series"
      />
      <LoadingSkeleton
        v-else-if="loading"
        variant="chart"
      />
      <div
        v-else
        class="h-100 d-flex align-items-center justify-content-center text-secondary"
      >
        Aucune donnée
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, shallowRef, watch, onMounted, computed, defineAsyncComponent } from 'vue'
import type { ApexOptions } from 'apexcharts'
import { fetchDiskMetricsHistory } from '../../composables/useDiskMetricsHistory'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import dayjs from '../../utils/dayjs'
import { getApiErrorMessage } from '../../api/client'
import { getApexChartPalette } from '../../utils/apexChartTheme'
import { clampTimestamp, getMinPointTimestamp, getMaxPointTimestamp } from '../../utils/chartTimeAxis'

interface ChartPoint {
  x: number
  y: number
  used_gb?: number
  size_gb?: number
}

// vue3-apexcharts' own exposed instance API (its .d.ts declares this but doesn't
// export it under a name that resolves cleanly through defineAsyncComponent's
// template-ref typing — restated locally for the one method actually needed).
interface ApexChartInstance {
  updateOptions(options: ApexOptions, redrawPaths?: boolean, animate?: boolean, updateSyncedCharts?: boolean): Promise<void>
}

const ApexChart = defineAsyncComponent(() => import('vue3-apexcharts').then((m) => m.default))

const props = withDefaults(defineProps<{
  hostId: string
  mounts?: string[]
  refreshTick?: number
}>(), {
  mounts: () => [],
  refreshTick: 0,
})

const chartHours = ref(24)
const selectedMount = ref<string>(props.mounts[0] ?? '')
const points = ref<ChartPoint[]>([])
const series = shallowRef<{ name: string; data: ChartPoint[] }[] | null>(null)
const chartOptions = shallowRef<ApexOptions | null>(null)
const chartRef = ref<ApexChartInstance | null>(null)
const loading = ref(false)
// Unlike `loading` (only true before the very first chart renders, so
// periodic WS-tick refreshes don't blank + re-animate an already-visible
// chart — see loadHistory), this is true for every in-flight fetch,
// including one triggered by a range button click, so that click always
// gets a visible spinner instead of silently doing nothing until the data
// pops in.
const rangeLoading = ref(false)

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

function formatChartTime(timestamp: number | string | undefined): string {
  if (!timestamp) return ''
  const d = dayjs(timestamp)
  if (!d.isValid()) return ''
  if (chartHours.value <= 24) return d.format('HH:mm')
  if (chartHours.value <= 720) return d.format('DD/MM HH:mm')
  return d.format('DD/MM')
}

// Built once (on first data load) rather than as a `computed` over `points`:
// vue3-apexcharts clones the whole `:options` prop via
// JSON.parse(JSON.stringify(...)) on every *reactive* update after mount —
// which silently drops every function (labels.formatter, tooltip.custom)
// since JSON can't represent them. Keeping this object's identity stable
// after the initial build means later data refreshes flow through the
// (function-free, always-safe) `:series` prop only; the time-range window
// (xaxis.min/max) is pushed via the exposed updateOptions() method directly
// in loadHistory() instead (bypasses the wrapper's buggy watcher).
function buildChartOptions(): ApexOptions {
  const palette = getApexChartPalette()
  return {
    chart: {
      type: 'area',
      background: 'transparent',
      toolbar: { show: false },
      zoom: { enabled: false },
      animations: { enabled: false },
      parentHeightOffset: 0,
    },
    theme: { mode: 'dark' },
    colors: [palette.disk],
    fill: { type: 'solid', opacity: 0.12 },
    stroke: { curve: 'smooth', width: 2 },
    markers: { size: 0, hover: { size: 4 } },
    dataLabels: { enabled: false },
    legend: { show: false },
    grid: { borderColor: palette.grid, strokeDashArray: 0 },
    xaxis: {
      type: 'datetime',
      min: getMinPointTimestamp(points.value),
      max: getMaxPointTimestamp(points.value),
      labels: {
        style: { colors: palette.tickText },
        formatter: (v: string) => formatChartTime(Number(v)),
      },
      axisBorder: { show: false },
      axisTicks: { show: false },
      tooltip: { enabled: false },
    },
    yaxis: {
      min: 0,
      max: 100,
      labels: {
        style: { colors: palette.tickText },
        formatter: (v: number) => `${v}%`,
      },
    },
    tooltip: {
      x: { formatter: (v: number) => formatChartTime(v) },
      y: {
        formatter: (v: number, opts?: { dataPointIndex: number }) => {
          const p = opts ? points.value[opts.dataPointIndex] : undefined
          return p?.used_gb != null && p?.size_gb
            ? `${v.toFixed(1)}%  (${p.used_gb.toFixed(1)} / ${p.size_gb.toFixed(1)} Go)`
            : `${v.toFixed(1)}%`
        },
      },
    },
  }
}

async function loadHistory(hours: number): Promise<void> {
  if (!selectedMount.value) return
  chartHours.value = hours
  // Only show the full-chart spinner on the very first load; on subsequent
  // refreshes (each WS tick) keep the existing chart visible and swap the
  // data in place, so the graph updates silently instead of blanking +
  // re-animating.
  if (!series.value) loading.value = true
  rangeLoading.value = true
  try {
    const raw = await fetchDiskMetricsHistory(props.hostId, selectedMount.value, hours)
    points.value = raw.map((p) => ({
      x: clampTimestamp(dayjs(p.timestamp).valueOf()),
      y: p.used_percent,
      used_gb: p.used_gb,
      size_gb: p.size_gb,
    })).filter((p: ChartPoint) => Number.isFinite(p.x) && p.y != null)

    if (!points.value.length) { series.value = null; return }

    series.value = [{ name: 'Utilisé', data: points.value }]
    if (!chartOptions.value) {
      chartOptions.value = buildChartOptions()
    } else {
      chartRef.value?.updateOptions(
        { xaxis: { min: getMinPointTimestamp(points.value), max: getMaxPointTimestamp(points.value) } },
        false, false,
      )
    }
  } catch (e: unknown) {
    console.error('Failed to load disk history:', getApiErrorMessage(e))
    series.value = null
  } finally {
    loading.value = false
    rangeLoading.value = false
  }
}

watch(() => props.mounts, (v) => {
  if (v.length && !selectedMount.value) {
    selectedMount.value = v[0]
    loadHistory(chartHours.value)
  }
}, { immediate: false })

let refreshTimer: ReturnType<typeof setTimeout> | null = null
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
