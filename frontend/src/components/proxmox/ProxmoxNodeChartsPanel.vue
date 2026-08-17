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
          type="button"
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
          :series="cpuChart || undefined"
          :options="pctOptions"
          :empty-text="error || 'Aucune donnée'"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          title="RAM"
          :series="ramChart || undefined"
          :options="pctOptions"
          :empty-text="error || 'Aucune donnée'"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          title="IO Wait"
          :series="iowaitChart || undefined"
          :options="pctOptions"
          :empty-text="error || 'Aucune donnée'"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          title="Réseau"
          :series="netChart || undefined"
          :options="netOptions"
          :empty-text="error || 'Aucune donnée'"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          title="Température CPU"
          :series="tempChart || undefined"
          :options="tempOptions"
          :empty-text="tempEmptyText"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          title="RPM Ventilateurs"
          :series="fanChart || undefined"
          :options="fanOptions"
          :empty-text="fanEmptyText"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { ApexOptions } from 'apexcharts'
import RRDChartCard, { type RRDChartSeries } from './RRDChartCard.vue'
import { getApexChartPalette } from '../../utils/apexChartTheme'
import dayjs from '../../utils/dayjs'

const props = withDefaults(defineProps<{
  cpuChart?: RRDChartSeries | null
  ramChart?: RRDChartSeries | null
  iowaitChart?: RRDChartSeries | null
  netChart?: RRDChartSeries | null
  tempChart?: RRDChartSeries | null
  fanChart?: RRDChartSeries | null
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

function formatAxisTime(value: string): string {
  const d = dayjs(Number(value))
  if (!d.isValid()) return ''
  if (props.timeframe === 'hour' || props.timeframe === 'day') return d.format('HH:mm')
  if (props.timeframe === 'week') return d.format('DD/MM HH:mm')
  return d.format('DD/MM')
}

function baseChartOptions(palette: ReturnType<typeof getApexChartPalette>): ApexOptions {
  return {
    chart: { type: 'area', toolbar: { show: false }, zoom: { enabled: false }, animations: { enabled: false }, parentHeightOffset: 0 },
    theme: { mode: 'dark' },
    fill: { type: 'solid', opacity: 0.1 },
    stroke: { curve: 'smooth', width: 2 },
    markers: { size: 0, hover: { size: 4 } },
    dataLabels: { enabled: false },
    legend: { show: false },
    grid: { borderColor: palette.grid },
    xaxis: {
      type: 'datetime',
      labels: { style: { colors: palette.tickText }, formatter: formatAxisTime },
      axisBorder: { show: false },
      axisTicks: { show: false },
      tooltip: { enabled: false },
    },
  }
}

function tooltipHtml(title: string, body: string): string {
  return '<div style="padding:6px 10px;font-size:12px;">'
    + `<div style="font-weight:600;margin-bottom:2px;">${title}</div>`
    + `<div>${body}</div>`
    + '</div>'
}

const pctOptions = computed((): ApexOptions => {
  const palette = getApexChartPalette()
  return {
    ...baseChartOptions(palette),
    yaxis: { min: 0, max: 100, labels: { style: { colors: palette.tickText }, formatter: (v: number) => `${v.toFixed(0)}%` } },
    tooltip: {
      custom: ({ series, seriesIndex, dataPointIndex, w }) => {
        const ts = w.globals.seriesX[seriesIndex]?.[dataPointIndex]
        const y = series[seriesIndex]?.[dataPointIndex]
        const body = y != null ? `${Number(y).toFixed(1)}%` : '—'
        return tooltipHtml(formatAxisTime(String(ts)), body)
      },
    },
  }
})

const netOptions = computed((): ApexOptions => {
  const palette = getApexChartPalette()
  return {
    ...baseChartOptions(palette),
    fill: { type: 'solid', opacity: [0.15, 0] },
    legend: { show: true, position: 'top', labels: { colors: palette.legendText }, markers: { size: 5 } },
    yaxis: { min: 0, labels: { style: { colors: palette.tickText }, formatter: (v: number) => formatBytesPerSec(v) } },
    tooltip: {
      shared: true,
      custom: ({ series, seriesIndex, dataPointIndex, w }) => {
        const ts = w.globals.seriesX[seriesIndex]?.[dataPointIndex]
        const rows = w.globals.seriesNames
          .map((name: string, idx: number) => `<div>${name}: ${formatBytesPerSec(series[idx]?.[dataPointIndex])}</div>`)
          .join('')
        return tooltipHtml(formatAxisTime(String(ts)), rows)
      },
    },
  }
})

const tempOptions = computed((): ApexOptions => {
  const palette = getApexChartPalette()
  return {
    ...baseChartOptions(palette),
    yaxis: { labels: { style: { colors: palette.tickText }, formatter: (v: number) => `${v.toFixed(0)}°C` } },
    tooltip: {
      custom: ({ series, seriesIndex, dataPointIndex, w }) => {
        const ts = w.globals.seriesX[seriesIndex]?.[dataPointIndex]
        const y = series[seriesIndex]?.[dataPointIndex]
        const body = y != null ? `${Number(y).toFixed(1)}°C` : '—'
        return tooltipHtml(formatAxisTime(String(ts)), body)
      },
    },
  }
})

const fanOptions = computed((): ApexOptions => {
  const palette = getApexChartPalette()
  return {
    ...baseChartOptions(palette),
    yaxis: { min: 0, labels: { style: { colors: palette.tickText }, formatter: (v: number) => `${Math.round(v)} RPM` } },
    tooltip: {
      custom: ({ series, seriesIndex, dataPointIndex, w }) => {
        const ts = w.globals.seriesX[seriesIndex]?.[dataPointIndex]
        const y = series[seriesIndex]?.[dataPointIndex]
        const body = y != null ? `${Math.round(Number(y))} RPM` : '—'
        return tooltipHtml(formatAxisTime(String(ts)), body)
      },
    },
  }
})
</script>
