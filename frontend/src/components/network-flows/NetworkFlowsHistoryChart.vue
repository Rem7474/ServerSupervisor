<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between flex-wrap gap-2">
      <h3 class="card-title mb-0">
        {{ mode === 'summary' ? 'Bande passante trackée' : 'Historique du talker' }}
      </h3>
      <TimeRangePicker
        v-model="timeRange"
        :presets="timeRangeOptions"
        :loading="loading"
        @change="loadHistory"
      />
    </div>
    <div
      class="card-body"
      :style="{ height: mode === 'summary' ? '13rem' : '10rem' }"
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
import { ref, shallowRef, watch, onMounted, defineAsyncComponent } from 'vue'
import type { ApexOptions } from 'apexcharts'
import { fetchNetworkFlowsSummary, fetchNetworkFlowsHistory } from '../../composables/useNetworkFlowsHistory'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import TimeRangePicker from '../common/TimeRangePicker.vue'
import type { TimeRangeModel, TimeRangePreset } from '../../types/timeRange'
import dayjs from '../../utils/dayjs'
import { formatBytes } from '../../utils/formatters'
import { getApiErrorMessage } from '../../api/client'
import { getApexChartPalette } from '../../utils/apexChartTheme'
import { clampTimestamp, getMinPointTimestamp, getMaxPointTimestamp } from '../../utils/chartTimeAxis'

interface ChartPoint {
  x: number
  y: number
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
  mode: 'summary' | 'talker'
  remoteIp?: string
  remotePort?: number
  protocol?: string
  refreshTick?: number
}>(), {
  remoteIp: '',
  remotePort: 0,
  protocol: '',
  refreshTick: 0,
})

const rxPoints = ref<ChartPoint[]>([])
const txPoints = ref<ChartPoint[]>([])
const series = shallowRef<{ name: string; data: ChartPoint[]; color: string }[] | null>(null)
const loading = ref(false)

const timeRangeOptions: TimeRangePreset[] = [
  { value: '1h', label: '1h' },
  { value: '6h', label: '6h' },
  { value: '24h', label: '24h' },
  { value: '168h', label: '7j' },
]

const timeRange = ref<TimeRangeModel>({ mode: 'preset', period: '24h', from: null, to: null })

// Approximate span in hours for the current selection — used only to decide
// the axis label granularity below, not sent to the API (the backend derives
// its own bucket size the same way, from the actual since/until).
function currentSpanHours(): number {
  if (timeRange.value.mode === 'custom' && timeRange.value.from && timeRange.value.to) {
    return (new Date(timeRange.value.to).getTime() - new Date(timeRange.value.from).getTime()) / 3600000
  }
  const match = /^(\d+)h$/.exec(timeRange.value.period)
  return match ? Number(match[1]) : 24
}

function formatChartTime(timestamp: number | string | undefined): string {
  if (!timestamp) return ''
  const d = dayjs(timestamp)
  if (!d.isValid()) return ''
  if (currentSpanHours() <= 24) return d.format('HH:mm')
  return d.format('DD/MM HH:mm')
}

function tooltipHtml(palette: ReturnType<typeof getApexChartPalette>, title: string, body: string): string {
  return `<div style="background:${palette.tooltipBackground};color:${palette.tooltipText};border:1px solid ${palette.tooltipBorder};border-radius:4px;padding:8px 10px;font-size:12px;">`
    + `<div style="font-weight:600;margin-bottom:2px;">${title}</div>`
    + body
    + '</div>'
}

const chartOptions = shallowRef<ApexOptions | null>(null)
const chartRef = ref<ApexChartInstance | null>(null)

// Built once (on first data load), not reassigned on every loadHistory()
// call: vue3-apexcharts clones the whole `:options` prop via
// JSON.parse(JSON.stringify(...)) on every *reactive* update after mount —
// which silently drops every function (labels.formatter, tooltip.custom)
// since JSON can't represent them. Keeping this object's identity stable
// after the initial build means later data refreshes flow through the
// (function-free, always-safe) `:series` prop only; the time-range window
// (xaxis.min/max) is pushed via the exposed updateOptions() method directly
// in loadHistory() instead (bypasses the wrapper's buggy watcher).
function buildChartOptions(): ApexOptions {
  const palette = getApexChartPalette()
  const allPoints = [...rxPoints.value, ...txPoints.value]
  return {
    chart: { type: 'area', toolbar: { show: false }, zoom: { enabled: false }, animations: { enabled: false }, parentHeightOffset: 0 },
    fill: { type: 'solid', opacity: 0.12 },
    stroke: { curve: 'smooth', width: 2 },
    markers: { size: 0, hover: { size: 4 } },
    dataLabels: { enabled: false },
    legend: { show: true, position: 'top', horizontalAlign: 'right', labels: { colors: palette.legendText }, markers: { size: 5 } },
    grid: { borderColor: palette.grid },
    xaxis: {
      type: 'datetime',
      min: getMinPointTimestamp(allPoints),
      max: getMaxPointTimestamp(allPoints),
      labels: { style: { colors: palette.tickText }, formatter: (v: string) => formatChartTime(Number(v)) },
      axisBorder: { show: false },
      axisTicks: { show: false },
      tooltip: { enabled: false },
    },
    yaxis: { min: 0, labels: { style: { colors: palette.tickText }, formatter: (v: number) => formatBytes(v) } },
    tooltip: {
      shared: true,
      custom: ({ series: s, seriesIndex, dataPointIndex, w }) => {
        const ts = w.globals.seriesX[seriesIndex]?.[dataPointIndex]
        const rows = w.globals.seriesNames
          .map((name: string, idx: number) => `<div>${name}: ${formatBytes(Number(s[idx]?.[dataPointIndex] ?? 0))}</div>`)
          .join('')
        return tooltipHtml(palette, formatChartTime(Number(ts)), rows)
      },
    },
  }
}

async function loadHistory(): Promise<void> {
  if (!series.value) loading.value = true
  try {
    const period = timeRange.value.period || '24h'
    const range = timeRange.value.mode === 'custom' && timeRange.value.from && timeRange.value.to
      ? { from: timeRange.value.from, to: timeRange.value.to }
      : undefined
    const raw = props.mode === 'summary'
      ? await fetchNetworkFlowsSummary(props.hostId, period, range)
      : await fetchNetworkFlowsHistory(props.hostId, props.remoteIp, props.remotePort, props.protocol, period, range)

    rxPoints.value = raw
      .map((p) => ({ x: clampTimestamp(dayjs(p.timestamp).valueOf()), y: p.rx_bytes }))
      .filter((p) => Number.isFinite(p.x))
    txPoints.value = raw
      .map((p) => ({ x: clampTimestamp(dayjs(p.timestamp).valueOf()), y: p.tx_bytes }))
      .filter((p) => Number.isFinite(p.x))

    if (!rxPoints.value.length && !txPoints.value.length) {
      series.value = null
      return
    }

    const palette = getApexChartPalette()
    series.value = [
      { name: 'Entrant (Rx)', data: rxPoints.value, color: palette.networkRx },
      { name: 'Sortant (Tx)', data: txPoints.value, color: palette.networkTx },
    ]
    if (!chartOptions.value) {
      chartOptions.value = buildChartOptions()
    } else {
      const allPoints = [...rxPoints.value, ...txPoints.value]
      chartRef.value?.updateOptions(
        { xaxis: { min: getMinPointTimestamp(allPoints), max: getMaxPointTimestamp(allPoints) } },
        false, false,
      )
    }
  } catch (e: unknown) {
    console.error('Failed to load network flows history:', getApiErrorMessage(e))
    series.value = null
  } finally {
    loading.value = false
  }
}

// Fires on every new report cycle for this host (see HostDetailView's
// metricsUpdatedAt). Deliberately does not touch `timeRange` — a custom
// range the user picked stays active across refreshes, same as a preset.
let refreshTimer: ReturnType<typeof setTimeout> | null = null
watch(() => props.refreshTick, () => {
  if (refreshTimer) clearTimeout(refreshTimer)
  refreshTimer = setTimeout(() => {
    refreshTimer = null
    loadHistory()
  }, 400)
})

onMounted(() => {
  loadHistory()
})
</script>
