<template>
  <div>
    <div class="d-flex align-items-center justify-content-between mb-3">
      <div class="subheader mb-0">
        {{ t('proxmox.rrdHistoryTitle') }}
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
          {{ t(opt.labelKey) }}
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
          :title="t('proxmox.cpuChartTitle')"
          :series="cpuChart || undefined"
          :options="pctOptions"
          :empty-text="error || t('proxmox.noDataLabel')"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          :title="t('proxmox.ramChartTitle')"
          :series="ramChart || undefined"
          :options="pctOptions"
          :empty-text="error || t('proxmox.noDataLabel')"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          :title="t('proxmox.iowaitChartTitle')"
          :series="iowaitChart || undefined"
          :options="pctOptions"
          :empty-text="error || t('proxmox.noDataLabel')"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          :title="t('proxmox.networkChartTitle')"
          :series="netChart || undefined"
          :options="netOptions"
          :empty-text="error || t('proxmox.noDataLabel')"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          :title="t('proxmox.cpuTempChartTitle')"
          :series="tempChart || undefined"
          :options="tempOptions"
          :empty-text="tempEmptyText ?? t('proxmox.noTempDataText')"
        />
      </div>
      <div class="col-12 col-lg-4">
        <RRDChartCard
          :title="t('proxmox.fanRpmChartTitle')"
          :series="fanChart || undefined"
          :options="fanOptions"
          :empty-text="fanEmptyText ?? t('proxmox.noFanDataText')"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ApexOptions } from 'apexcharts'
import RRDChartCard, { type RRDChartSeries } from './RRDChartCard.vue'
import { getApexChartPalette } from '../../utils/apexChartTheme'
import dayjs from '../../utils/dayjs'

const { t } = useI18n()

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
})

defineEmits<{
  (e: 'timeframe-changed', value: string): void
}>()

const timeframeOptions = [
  { value: 'hour', labelKey: 'proxmox.timeRange1h' },
  { value: 'day', labelKey: 'proxmox.timeRange24h' },
  { value: 'week', labelKey: 'proxmox.timeRange7d' },
  { value: 'month', labelKey: 'proxmox.timeRange30d' },
  { value: 'year', labelKey: 'proxmox.timeRangeYear' },
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
    chart: { type: 'area', background: 'transparent', toolbar: { show: false }, zoom: { enabled: false }, animations: { enabled: false }, parentHeightOffset: 0 },
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

const pctOptions = computed((): ApexOptions => {
  const palette = getApexChartPalette()
  return {
    ...baseChartOptions(palette),
    yaxis: { min: 0, max: 100, labels: { style: { colors: palette.tickText }, formatter: (v: number) => `${v.toFixed(0)}%` } },
    tooltip: {
      x: { formatter: formatAxisTime },
      y: { formatter: (v: number) => (v != null ? `${Number(v).toFixed(1)}%` : '—') },
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
      x: { formatter: formatAxisTime },
      y: { formatter: (v: number) => formatBytesPerSec(v) },
    },
  }
})

const tempOptions = computed((): ApexOptions => {
  const palette = getApexChartPalette()
  return {
    ...baseChartOptions(palette),
    yaxis: { labels: { style: { colors: palette.tickText }, formatter: (v: number) => `${v.toFixed(0)}°C` } },
    tooltip: {
      x: { formatter: formatAxisTime },
      y: { formatter: (v: number) => (v != null ? `${Number(v).toFixed(1)}°C` : '—') },
    },
  }
})

const fanOptions = computed((): ApexOptions => {
  const palette = getApexChartPalette()
  return {
    ...baseChartOptions(palette),
    yaxis: { min: 0, labels: { style: { colors: palette.tickText }, formatter: (v: number) => `${Math.round(v)} RPM` } },
    tooltip: {
      x: { formatter: formatAxisTime },
      y: { formatter: (v: number) => (v != null ? `${Math.round(Number(v))} RPM` : '—') },
    },
  }
})
</script>
