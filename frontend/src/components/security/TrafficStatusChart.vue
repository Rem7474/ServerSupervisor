<template>
  <Transition name="skeleton-fade">
    <LoadingSkeleton
      v-if="!chartReady || loading"
      variant="donut"
      class="position-absolute inset-0"
    />
  </Transition>
  <div
    class="apex-chart-wrap"
    :style="{ opacity: (chartReady && !loading) ? 1 : 0, transition: 'opacity 0.3s' }"
  >
    <ApexChart
      type="donut"
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

type StatusDistribution = Record<string, number>

const props = defineProps<{
  statusDistribution: StatusDistribution
  chartReady: boolean
  loading: boolean
}>()

const ApexChart = defineAsyncComponent(() => import('vue3-apexcharts').then((m) => m.default))

function tooltipHtml(title: string, body: string): string {
  return '<div style="padding:6px 10px;font-size:12px;">'
    + `<div style="font-weight:600;margin-bottom:2px;">${title}</div>`
    + `<div>${body}</div>`
    + '</div>'
}

const series = computed(() => {
  const d = props.statusDistribution || {}
  return [
    Number(d['2xx']) || 0,
    Number(d['3xx']) || 0,
    Number(d['4xx']) || 0,
    Number(d['5xx']) || 0,
  ]
})

const chartOptions = computed((): ApexOptions => {
  const palette = getApexChartPalette()
  return {
    chart: { type: 'donut', animations: { enabled: false } },
    theme: { mode: 'dark' },
    labels: ['2xx', '3xx', '4xx', '5xx'],
    colors: ['#639922', '#185FA5', '#BA7517', '#E24B4A'],
    stroke: { width: 0 },
    dataLabels: { enabled: false },
    plotOptions: { pie: { donut: { size: '70%' } } },
    // onItemHover.highlightDataSeries: false — ApexCharts' legend-hover
    // highlight path (highlightSeries → getSeriesByName) throws on this
    // donut chart's plain-number series ("Cannot read properties of
    // undefined (reading 'toString')" in escapeString), since donut/pie
    // series have no per-item .name the way category-chart series do
    // (labels live in options.labels instead). Disabling the hover
    // highlight avoids the crash; the legend itself still shows/toggles
    // series on click.
    legend: { show: true, position: 'bottom', labels: { colors: palette.legendText }, onItemHover: { highlightDataSeries: false } },
    tooltip: {
      custom: ({ series: s, seriesIndex, w }) => {
        const label = w.globals.labels[seriesIndex] ?? ''
        return tooltipHtml(String(label), String(s[seriesIndex] ?? 0))
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
