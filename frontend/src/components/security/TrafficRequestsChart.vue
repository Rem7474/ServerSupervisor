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
      v-if="chartOptions"
      ref="chartRef"
      type="bar"
      height="100%"
      :options="chartOptions"
      :series="series"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, ref, shallowRef, watch } from 'vue'
import type { ApexOptions } from 'apexcharts'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import { getApexChartPalette } from '../../utils/apexChartTheme'

interface Point {
  timestamp: string
  human?: number | string
  bot?: number | string
  [key: string]: unknown
}

// vue3-apexcharts' own exposed instance API (its .d.ts declares this but doesn't
// export it under a name that resolves cleanly through defineAsyncComponent's
// template-ref typing — restated locally for the one method actually needed).
interface ApexChartInstance {
  updateOptions(options: ApexOptions, redrawPaths?: boolean, animate?: boolean, updateSyncedCharts?: boolean): Promise<void>
}

const props = defineProps<{
  timeseries: Point[]
  period: string
  chartReady: boolean
  loading: boolean
}>()

const ApexChart = defineAsyncComponent(() => import('vue3-apexcharts').then((m) => m.default))
const chartRef = ref<ApexChartInstance | null>(null)

function bucketLabel(ts: string): string {
  const d = new Date(ts)
  if (Number.isNaN(d.getTime())) return ts
  return props.period === '1h'
    ? d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    : d.toLocaleString([], { day: '2-digit', month: '2-digit', hour: '2-digit' })
}

const categories = computed(() => props.timeseries.map((p) => bucketLabel(p.timestamp)))

const series = computed(() => [
  { name: 'Humain', data: props.timeseries.map((p) => Number(p.human) || 0) },
  { name: 'Bot/scan', data: props.timeseries.map((p) => Number(p.bot) || 0) },
])

// Built once (on first data arrival) rather than as a `computed` over
// `timeseries`: vue3-apexcharts clones the whole `:options` prop via
// JSON.parse(JSON.stringify(...)) on every *reactive* update after mount —
// which silently drops every function (yaxis.labels.formatter,
// tooltip.custom) since JSON can't represent them. Keeping this object's
// identity stable after the initial build means later data refreshes flow
// through the (function-free, always-safe) `:series` prop only; the
// category labels (the one other thing that legitimately changes per
// refresh, since this is a category- not datetime-axis chart) are pushed
// via the exposed updateOptions() method directly in the watcher below
// (bypasses the wrapper's buggy reactive watcher).
const chartOptions = shallowRef<ApexOptions | null>(null)

function buildChartOptions(): ApexOptions {
  const palette = getApexChartPalette()
  return {
    chart: { type: 'bar', stacked: true, toolbar: { show: false }, animations: { enabled: false } },
    theme: { mode: 'dark' },
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
      y: { formatter: (v: number) => `${v}` },
    },
  }
}

watch(categories, (cats) => {
  if (!chartOptions.value) {
    chartOptions.value = buildChartOptions()
  } else {
    chartRef.value?.updateOptions({ xaxis: { categories: cats } }, false, false)
  }
}, { immediate: true })
</script>

<style scoped>
.apex-chart-wrap {
  height: 100%;
}

:deep(.apexcharts-legend-series) {
  cursor: pointer;
}
</style>
