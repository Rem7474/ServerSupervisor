<template>
  <div>
    <div class="d-flex align-items-center justify-content-between mb-3">
      <div class="subheader mb-0">
        Historique RRD
      </div>
      <div
        v-if="!loading"
        class="btn-group btn-group-sm"
      >
        <button
          v-for="opt in timeframeOptions"
          :key="opt.value"
          :class="timeframe === opt.value ? 'btn btn-primary' : 'btn btn-outline-secondary'"
          @click="$emit('timeframe-changed', opt.value)"
        >
          {{ opt.label }}
        </button>
      </div>
      <span
        v-else
        class="spinner-border spinner-border-sm text-muted"
      />
    </div>

    <div class="row row-cards mb-4">
      <div class="col-12 col-lg-4">
        <RRDChartCard
          title="CPU"
          :chart-data="cpuChart || undefined"
          :options="pctOptions"
          :empty-text="error || 'Aucune donnée'"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          title="RAM"
          :chart-data="ramChart || undefined"
          :options="ramOptions"
          :empty-text="error || 'Aucune donnée'"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          title="IO Wait"
          :chart-data="iowaitChart || undefined"
          :options="pctOptions"
          :empty-text="error || 'Aucune donnée'"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          title="Réseau"
          :chart-data="netChart || undefined"
          :options="netOptions"
          :empty-text="error || 'Aucune donnée'"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          title="Température CPU"
          :chart-data="tempChart || undefined"
          :options="tempOptions"
          :empty-text="tempEmptyText"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          title="RPM Ventilateurs"
          :chart-data="fanChart || undefined"
          :options="fanOptions"
          :empty-text="fanEmptyText"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import RRDChartCard from './RRDChartCard.vue'
import { getChartPalette } from '../../utils/chartTheme'

const palette = getChartPalette()
const tooltipBase = {
  enabled: true, mode: 'index' as const, intersect: false,
  backgroundColor: palette.tooltipBackground,
  titleColor: palette.tooltipText,
  bodyColor: palette.tooltipText,
  borderColor: palette.tooltipBorder,
  borderWidth: 1,
  padding: 8,
}

withDefaults(defineProps<{
  cpuChart?: object | null
  ramChart?: object | null
  iowaitChart?: object | null
  netChart?: object | null
  tempChart?: object | null
  fanChart?: object | null
  timeframe?: string
  loading?: boolean
  error?: string
  tempEmptyText?: string
  fanEmptyText?: string
}>(), {
  cpuChart: null,
  ramChart: null,
  iowaitChart: null,
  netChart: null,
  tempChart: null,
  fanChart: null,
  timeframe: 'hour',
  loading: false,
  error: '',
  tempEmptyText: 'Aucune donnée température disponible',
  fanEmptyText: 'Aucune donnée ventilateur disponible',
})

defineEmits<{
  (e: 'timeframe-changed', value: string): void
}>()

const timeframeOptions = [
  { value: 'hour', label: '1h' },
  { value: 'day', label: '24h' },
  { value: 'week', label: '7j' },
  { value: 'month', label: '30j' },
  { value: 'year', label: '1 an' },
]

function formatBytesPerSec(v: number | null | undefined): string {
  if (v == null) return '—'
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)} MB/s`
  if (v >= 1_000) return `${(v / 1_000).toFixed(1)} KB/s`
  return `${v.toFixed(0)} B/s`
}

const pctOptions = {
  responsive: true, maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      ...tooltipBase,
      displayColors: false,
      callbacks: { label: (ctx: any) => `${ctx.parsed.y != null ? ctx.parsed.y.toFixed(1) : '—'}%` },
    },
  },
  scales: {
    x: { display: true, grid: { color: palette.grid }, ticks: { color: palette.tickText, maxTicksLimit: 8 } },
    y: { display: true, min: 0, max: 100, grid: { color: palette.grid }, ticks: { color: palette.tickText, callback: (v: number | string) => `${v}%` } },
  },
  elements: { point: { radius: 0, hitRadius: 10, hoverRadius: 4 }, line: { tension: 0.3 } },
  interaction: { mode: 'nearest', axis: 'x', intersect: false },
}

const ramOptions = {
  ...pctOptions,
  plugins: {
    ...pctOptions.plugins,
    tooltip: {
      ...pctOptions.plugins.tooltip,
      callbacks: {
        label: (ctx: any) => {
          const pct = ctx.parsed.y != null ? ctx.parsed.y.toFixed(1) : '—'
          return `${pct}%`
        },
      },
    },
  },
}

const netOptions = {
  responsive: true, maintainAspectRatio: false,
  plugins: {
    legend: { display: true, position: 'top', labels: { color: palette.legendText, boxWidth: 10, padding: 8 } },
    tooltip: {
      ...tooltipBase,
      callbacks: { label: (ctx: any) => `${ctx.dataset.label}: ${formatBytesPerSec(ctx.parsed.y)}` },
    },
  },
  scales: {
    x: { display: true, grid: { color: palette.grid }, ticks: { color: palette.tickText, maxTicksLimit: 8 } },
    y: { display: true, min: 0, grid: { color: palette.grid }, ticks: { color: palette.tickText, callback: (v: number | string) => formatBytesPerSec(typeof v === 'number' ? v : Number(v)) } },
  },
  elements: { point: { radius: 0, hitRadius: 10, hoverRadius: 4 }, line: { tension: 0.3 } },
  interaction: { mode: 'nearest', axis: 'x', intersect: false },
}

const tempOptions = {
  responsive: true, maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      ...tooltipBase,
      callbacks: { label: (ctx: any) => `${ctx.parsed.y != null ? ctx.parsed.y.toFixed(1) : '—'}°C` },
    },
  },
  scales: {
    x: { display: true, grid: { color: palette.grid }, ticks: { color: palette.tickText, maxTicksLimit: 8 } },
    y: { display: true, grid: { color: palette.grid }, ticks: { color: palette.tickText, callback: (v: number | string) => `${v}°C` } },
  },
  elements: { point: { radius: 0, hitRadius: 10, hoverRadius: 4 }, line: { tension: 0.3 } },
  interaction: { mode: 'nearest', axis: 'x', intersect: false },
}

const fanOptions = {
  responsive: true, maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      ...tooltipBase,
      callbacks: { label: (ctx: any) => `${ctx.parsed.y != null ? Math.round(ctx.parsed.y) : '—'} RPM` },
    },
  },
  scales: {
    x: { display: true, grid: { color: palette.grid }, ticks: { color: palette.tickText, maxTicksLimit: 8 } },
    y: { display: true, min: 0, grid: { color: palette.grid }, ticks: { color: palette.tickText, callback: (v: number | string) => `${v} RPM` } },
  },
  elements: { point: { radius: 0, hitRadius: 10, hoverRadius: 4 }, line: { tension: 0.3 } },
  interaction: { mode: 'nearest', axis: 'x', intersect: false },
}
</script>
