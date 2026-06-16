<template>
  <Transition name="skeleton-fade">
    <LoadingSkeleton
      v-if="!chartReady || loading"
      variant="donut"
      class="position-absolute inset-0"
    />
  </Transition>
  <canvas
    ref="canvasEl"
    :style="{ opacity: (chartReady && !loading) ? 1 : 0, transition: 'opacity 0.3s' }"
  />
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { Chart, ChartEvent, LegendItem, LegendElement } from 'chart.js'
import LoadingSkeleton from '../LoadingSkeleton.vue'

type StatusDistribution = Record<string, number>

const props = defineProps<{
  statusDistribution: StatusDistribution
  chartReady: boolean
  loading: boolean
}>()

const canvasEl = ref<HTMLCanvasElement | null>(null)
let chart: Chart | null = null
let chartLib: typeof Chart | null = null

async function ensureChartLib() {
  if (chartLib) return chartLib
  const mod = await import('chart.js')
  mod.Chart.register(...mod.registerables)
  chartLib = mod.Chart
  return chartLib
}

async function render() {
  const Chart = await ensureChartLib()
  if (!canvasEl.value) return
  if (chart) {
    chart.destroy()
    chart = null
  }
  const d = props.statusDistribution || {}
  chart = new Chart(canvasEl.value, {
    type: 'doughnut',
    data: {
      labels: ['2xx', '3xx', '4xx', '5xx'],
      datasets: [{
        data: [
          Number(d['2xx']) || 0,
          Number(d['3xx']) || 0,
          Number(d['4xx']) || 0,
          Number(d['5xx']) || 0,
        ],
        backgroundColor: ['#639922', '#185FA5', '#BA7517', '#E24B4A'],
        borderWidth: 0,
      }],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      cutout: '70%',
      plugins: {
        legend: {
          position: 'bottom',
          onHover: (_e: ChartEvent, _item: LegendItem, legend: LegendElement<'doughnut'>) => {
            legend.chart.canvas.style.cursor = 'pointer'
          },
          onLeave: (_e: ChartEvent, _item: LegendItem, legend: LegendElement<'doughnut'>) => {
            legend.chart.canvas.style.cursor = 'default'
          },
        },
      },
    },
  })
}

onMounted(() => {
  void render()
})

watch(
  () => props.statusDistribution,
  () => {
    void render()
  },
  { deep: true }
)

onBeforeUnmount(() => {
  if (chart) chart.destroy()
})
</script>
