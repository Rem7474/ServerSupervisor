<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between flex-wrap gap-2">
      <h3 class="card-title mb-0">
        {{ mode === 'summary' ? 'Bande passante trackée' : 'Historique du talker' }}
      </h3>
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
    <div
      class="card-body"
      :style="{ height: mode === 'summary' ? '13rem' : '10rem' }"
    >
      <Line
        v-if="chartData"
        :data="chartData"
        :options="chartOptions"
        class="h-100"
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
import type { ChartData, ChartOptions, TooltipItem } from 'chart.js'
import { fetchNetworkFlowsSummary, fetchNetworkFlowsHistory } from '../../composables/useNetworkFlowsHistory'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import dayjs from '../../utils/dayjs'
import { formatBytes } from '../../utils/formatters'
import { getApiErrorMessage } from '../../api/client'

// RX/TX are a fixed-identity categorical pair (not a magnitude ramp), so hue
// is assigned in a fixed order and validated for CVD safety — see the
// dataviz skill's color-formula check (ΔE 15.9 deutan / 17.8 normal-vision
// for this azure/teal pair against the dark surface, both well above the
// floor). Never cycle or reassign these per filter.
const RX_COLOR = 'var(--tblr-azure)' // #4299e1 — inbound
const TX_COLOR = 'var(--tblr-teal)' // #0ca678 — outbound

interface ChartPoint {
  x: number
  y: number
}

const Line = defineAsyncComponent(async () => {
  const [{ Line }, { Chart: ChartJS, LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip, Legend }] = await Promise.all([
    import('vue-chartjs'),
    import('chart.js'),
  ])
  ChartJS.register(LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip, Legend)
  return Line
})

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

const chartHours = ref(24)
const rxPoints = ref<ChartPoint[]>([])
const txPoints = ref<ChartPoint[]>([])
const chartData = shallowRef<ChartData<'line'> | null>(null)
const loading = ref(false)

const timeRangeOptions = [
  { hours: 1,   label: '1h' },
  { hours: 6,   label: '6h' },
  { hours: 24,  label: '24h' },
  { hours: 168, label: '7j' },
]

function formatChartTime(timestamp: number | string | undefined): string {
  if (!timestamp) return ''
  const d = dayjs(timestamp)
  if (!d.isValid()) return ''
  if (chartHours.value <= 24) return d.format('HH:mm')
  return d.format('DD/MM HH:mm')
}

function clampTimestamp(timestampMs: number): number {
  if (!Number.isFinite(timestampMs)) return NaN
  return Math.min(timestampMs, Date.now())
}

function getMaxPointTimestamp(list: ChartPoint[]): number | undefined {
  let max = -Infinity
  for (const p of list) if (Number.isFinite(p.x) && p.x > max) max = p.x
  return Number.isFinite(max) ? Math.min(Date.now(), max) : undefined
}

function getMinPointTimestamp(list: ChartPoint[]): number | undefined {
  let min = Infinity
  for (const p of list) if (Number.isFinite(p.x) && p.x < min) min = p.x
  return Number.isFinite(min) ? min : undefined
}

const chartOptions = ref<ChartOptions<'line'>>()

function buildChartOptions(): ChartOptions<'line'> {
  const allPoints = [...rxPoints.value, ...txPoints.value]
  return {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        display: true,
        position: 'top',
        align: 'end',
        labels: { color: '#9ca3af', boxWidth: 12, boxHeight: 12, usePointStyle: true, pointStyle: 'rectRounded' },
      },
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
          title: (items: TooltipItem<'line'>[]) => formatChartTime(items[0]?.parsed?.x ?? undefined),
          label: (ctx: TooltipItem<'line'>) => `${ctx.dataset.label}: ${formatBytes(ctx.parsed.y ?? 0)}`,
        },
      },
    },
    scales: {
      x: {
        type: 'linear',
        display: true,
        grid: { color: 'rgba(255,255,255,0.05)' },
        min: getMinPointTimestamp(allPoints),
        max: getMaxPointTimestamp(allPoints),
        ticks: { color: '#6b7280', maxTicksLimit: 8, callback: (v: number | string) => formatChartTime(Number(v)) },
      },
      y: {
        display: true,
        min: 0,
        grid: { color: 'rgba(255,255,255,0.05)' },
        ticks: { color: '#6b7280', callback: (v: number | string) => formatBytes(Number(v)) },
      },
    },
    elements: { point: { radius: 0, hitRadius: 10, hoverRadius: 4 }, line: { tension: 0.3 } },
    interaction: { mode: 'nearest', axis: 'x', intersect: false },
  }
}

function cssVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

async function loadHistory(hours: number): Promise<void> {
  chartHours.value = hours
  if (!chartData.value) loading.value = true
  try {
    const raw = props.mode === 'summary'
      ? await fetchNetworkFlowsSummary(props.hostId, hours)
      : await fetchNetworkFlowsHistory(props.hostId, props.remoteIp, props.remotePort, props.protocol, hours)

    rxPoints.value = raw
      .map((p) => ({ x: clampTimestamp(dayjs(p.timestamp).valueOf()), y: p.rx_bytes }))
      .filter((p) => Number.isFinite(p.x))
    txPoints.value = raw
      .map((p) => ({ x: clampTimestamp(dayjs(p.timestamp).valueOf()), y: p.tx_bytes }))
      .filter((p) => Number.isFinite(p.x))

    if (!rxPoints.value.length && !txPoints.value.length) {
      chartData.value = null
      return
    }

    chartData.value = {
      datasets: [
        {
          label: 'Entrant (Rx)',
          data: rxPoints.value,
          borderColor: RX_COLOR,
          backgroundColor: `rgba(${cssVar('--tblr-azure-rgb')},0.12)`,
          fill: true,
          tension: 0.3,
          spanGaps: false,
        },
        {
          label: 'Sortant (Tx)',
          data: txPoints.value,
          borderColor: TX_COLOR,
          backgroundColor: `rgba(${cssVar('--tblr-teal-rgb')},0.12)`,
          fill: true,
          tension: 0.3,
          spanGaps: false,
        },
      ],
    }
    chartOptions.value = buildChartOptions()
  } catch (e: unknown) {
    console.error('Failed to load network flows history:', getApiErrorMessage(e))
    chartData.value = null
  } finally {
    loading.value = false
  }
}

let refreshTimer: ReturnType<typeof setTimeout> | null = null
watch(() => props.refreshTick, () => {
  if (refreshTimer) clearTimeout(refreshTimer)
  refreshTimer = setTimeout(() => {
    refreshTimer = null
    loadHistory(chartHours.value)
  }, 400)
})

onMounted(() => {
  loadHistory(chartHours.value)
})
</script>
