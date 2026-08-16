<template>
  <Transition name="skeleton-fade">
    <LoadingSkeleton
      v-if="!chartReady || loading"
      variant="chart"
      class="position-absolute inset-0"
    />
  </Transition>
  <div
    class="apex-chart-wrap"
    :style="{ opacity: (chartReady && !loading) ? 1 : 0, transition: 'opacity 0.3s' }"
  >
    <ApexChart
      type="bar"
      height="100%"
      :options="chartOptions"
      :series="series"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue'
import type { ApexOptions } from 'apexcharts'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import { getApexChartPalette } from '../../utils/apexChartTheme'

interface Point {
  timestamp: string
  human?: number | string
  bot?: number | string
  [key: string]: unknown
}

const props = defineProps<{
  timeseries: Point[]
  period: string
  chartReady: boolean
  loading: boolean
}>()

const ApexChart = defineAsyncComponent(() => import('vue3-apexcharts').then((m) => m.default))

function bucketLabel(ts: string): string {
  const d = new Date(ts)
  if (Number.isNaN(d.getTime())) return ts
  return props.period === '1h'
    ? d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    : d.toLocaleString([], { day: '2-digit', month: '2-digit', hour: '2-digit' })
}

function tooltipHtml(palette: ReturnType<typeof getApexChartPalette>, title: string, body: string): string {
  return `<div style="background:${palette.tooltipBackground};color:${palette.tooltipText};border:1px solid ${palette.tooltipBorder};border-radius:4px;padding:8px 10px;font-size:12px;">`
    + `<div style="font-weight:600;margin-bottom:2px;">${title}</div>`
    + body
    + '</div>'
}

const categories = computed(() => props.timeseries.map((p) => bucketLabel(p.timestamp)))

const series = computed(() => [
  { name: 'Humain', data: props.timeseries.map((p) => Number(p.human) || 0) },
  { name: 'Bot/scan', data: props.timeseries.map((p) => Number(p.bot) || 0) },
])

const chartOptions = computed((): ApexOptions => {
  const palette = getApexChartPalette()
  return {
    chart: { type: 'bar', stacked: true, toolbar: { show: false }, animations: { enabled: false } },
    colors: ['#378ADD', '#E24B4A'],
    plotOptions: { bar: { columnWidth: '60%' } },
    dataLabels: { enabled: false },
    legend: { show: true, position: 'bottom', labels: { colors: palette.legendText } },
    grid: { borderColor: palette.grid, xaxis: { lines: { show: false } } },
    xaxis: {
      type: 'category',
      categories: categories.value,
      labels: { style: { colors: palette.tickText } },
      axisBorder: { show: false },
      axisTicks: { show: false },
    },
    yaxis: { labels: { style: { colors: palette.tickText }, formatter: (v: number) => `${Math.round(v)}` } },
    tooltip: {
      shared: true,
      intersect: false,
      custom: ({ series: s, dataPointIndex, w }) => {
        const title = categories.value[dataPointIndex] ?? ''
        const rows = w.globals.seriesNames
          .map((name: string, idx: number) => `<div>${name}: ${s[idx]?.[dataPointIndex] ?? 0}</div>`)
          .join('')
        return tooltipHtml(palette, String(title), rows)
      },
    },
  }
})
</script>

<style scoped>
.apex-chart-wrap {
  height: 100%;
}

:deep(.apexcharts-legend-series) {
  cursor: pointer;
}
</style>
