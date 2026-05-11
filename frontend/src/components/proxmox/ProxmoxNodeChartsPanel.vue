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
          :chart-data="cpuChart"
          :options="pctOptions"
          :empty-text="error || 'Aucune donnée'"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          title="RAM"
          :chart-data="ramChart"
          :options="ramOptions"
          :empty-text="error || 'Aucune donnée'"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          title="IO Wait"
          :chart-data="iowaitChart"
          :options="pctOptions"
          :empty-text="error || 'Aucune donnée'"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          title="Réseau"
          :chart-data="netChart"
          :options="netOptions"
          :empty-text="error || 'Aucune donnée'"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          title="Température CPU"
          :chart-data="tempChart"
          :options="tempOptions"
          :empty-text="tempEmptyText"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          title="RPM Ventilateurs"
          :chart-data="fanChart"
          :options="fanOptions"
          :empty-text="fanEmptyText"
        />
      </div>
    </div>
  </div>
</template>

<script setup>
import RRDChartCard from './RRDChartCard.vue'

defineProps({
  cpuChart: { type: Object, default: null },
  ramChart: { type: Object, default: null },
  iowaitChart: { type: Object, default: null },
  netChart: { type: Object, default: null },
  tempChart: { type: Object, default: null },
  fanChart: { type: Object, default: null },
  timeframe: { type: String, default: 'hour' },
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
  tempEmptyText: { type: String, default: 'Aucune donnée température disponible' },
  fanEmptyText: { type: String, default: 'Aucune donnée ventilateur disponible' },
})

defineEmits(['timeframe-changed'])

const timeframeOptions = [
  { value: 'hour', label: '1h' },
  { value: 'day', label: '24h' },
  { value: 'week', label: '7j' },
  { value: 'month', label: '30j' },
  { value: 'year', label: '1 an' },
]

function formatBytesPerSec(v) {
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
      enabled: true, mode: 'index', intersect: false,
      backgroundColor: 'rgba(0,0,0,0.8)', titleColor: '#fff', bodyColor: '#fff',
      borderColor: '#555', borderWidth: 1, padding: 8, displayColors: false,
      callbacks: { label: (ctx) => `${ctx.parsed.y != null ? ctx.parsed.y.toFixed(1) : '—'}%` },
    },
  },
  scales: {
    x: { display: true, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', maxTicksLimit: 8 } },
    y: { display: true, min: 0, max: 100, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', callback: (v) => `${v}%` } },
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
        label: (ctx) => {
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
    legend: { display: true, position: 'top', labels: { color: '#6b7280', boxWidth: 10, padding: 8 } },
    tooltip: {
      enabled: true, mode: 'index', intersect: false,
      backgroundColor: 'rgba(0,0,0,0.8)', titleColor: '#fff', bodyColor: '#fff',
      borderColor: '#555', borderWidth: 1, padding: 8,
      callbacks: { label: (ctx) => `${ctx.dataset.label}: ${formatBytesPerSec(ctx.parsed.y)}` },
    },
  },
  scales: {
    x: { display: true, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', maxTicksLimit: 8 } },
    y: { display: true, min: 0, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', callback: (v) => formatBytesPerSec(v) } },
  },
  elements: { point: { radius: 0, hitRadius: 10, hoverRadius: 4 }, line: { tension: 0.3 } },
  interaction: { mode: 'nearest', axis: 'x', intersect: false },
}

const tempOptions = {
  responsive: true, maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      enabled: true, mode: 'index', intersect: false,
      backgroundColor: 'rgba(0,0,0,0.8)', titleColor: '#fff', bodyColor: '#fff',
      borderColor: '#555', borderWidth: 1, padding: 8,
      callbacks: { label: (ctx) => `${ctx.parsed.y != null ? ctx.parsed.y.toFixed(1) : '—'}°C` },
    },
  },
  scales: {
    x: { display: true, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', maxTicksLimit: 8 } },
    y: { display: true, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', callback: (v) => `${v}°C` } },
  },
  elements: { point: { radius: 0, hitRadius: 10, hoverRadius: 4 }, line: { tension: 0.3 } },
  interaction: { mode: 'nearest', axis: 'x', intersect: false },
}

const fanOptions = {
  responsive: true, maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      enabled: true, mode: 'index', intersect: false,
      backgroundColor: 'rgba(0,0,0,0.8)', titleColor: '#fff', bodyColor: '#fff',
      borderColor: '#555', borderWidth: 1, padding: 8,
      callbacks: { label: (ctx) => `${ctx.parsed.y != null ? Math.round(ctx.parsed.y) : '—'} RPM` },
    },
  },
  scales: {
    x: { display: true, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', maxTicksLimit: 8 } },
    y: { display: true, min: 0, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#6b7280', callback: (v) => `${v} RPM` } },
  },
  elements: { point: { radius: 0, hitRadius: 10, hoverRadius: 4 }, line: { tension: 0.3 } },
  interaction: { mode: 'nearest', axis: 'x', intersect: false },
}
</script>
