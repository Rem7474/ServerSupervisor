<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between flex-wrap gap-2">
      <h3 class="card-title mb-0">
        {{ mode === 'summary' ? t('network.flowsSummaryTitle') : t('network.flowsTalkerTitle') }}
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
        {{ t('common.noData') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, shallowRef, watch, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ApexOptions } from 'apexcharts'
import { fetchNetworkFlowsSummary, fetchNetworkFlowsHistory } from '../../composables/useNetworkFlowsHistory'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import TimeRangePicker from '../common/TimeRangePicker.vue'
import type { TimeRangeModel, TimeRangePreset } from '../../types/timeRange'
import dayjs from '../../utils/dayjs'
import { formatBytes } from '../../utils/formatters'
import { getApiErrorMessage } from '../../api/client'
import { getApexChartPalette, AsyncApexChart as ApexChart, type ApexChartInstance } from '../../utils/apexChartTheme'
import { clampTimestamp, getMinPointTimestamp, getMaxPointTimestamp } from '../../utils/chartTimeAxis'

interface ChartPoint {
  x: number
  y: number
}

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

const { t } = useI18n()
const rxPoints = ref<ChartPoint[]>([])
const txPoints = ref<ChartPoint[]>([])
const series = shallowRef<{ name: string; data: ChartPoint[]; color: string }[] | null>(null)
const loading = ref(false)

const timeRangeOptions = computed<TimeRangePreset[]>(() => [
  { value: '1h', label: '1h' },
  { value: '6h', label: '6h' },
  { value: '24h', label: '24h' },
  { value: '168h', label: t('network.flowsRangeSevenDays') },
])

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
    chart: { type: 'area', background: 'transparent', toolbar: { show: false }, zoom: { enabled: false }, animations: { enabled: false }, parentHeightOffset: 0 },
    theme: { mode: 'dark' },
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
      x: { formatter: (v: number) => formatChartTime(v) },
      y: { formatter: (v: number) => formatBytes(v) },
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
      { name: t('network.flowsRxSeries'), data: rxPoints.value, color: palette.networkRx },
      { name: t('network.flowsTxSeries'), data: txPoints.value, color: palette.networkTx },
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
