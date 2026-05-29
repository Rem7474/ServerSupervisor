<template>
  <Transition name="skeleton-fade">
    <LoadingSkeleton
      v-if="!chartReady || loading"
      variant="chart"
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
import LoadingSkeleton from '../LoadingSkeleton.vue'

type Point = Record<string, any>

const props = defineProps<{
  timeseries: Point[]
  period: string
  chartReady: boolean
  loading: boolean
}>()

const canvasEl = ref<HTMLCanvasElement | null>(null)
let chart: any = null
let chartLib: any = null

async function ensureChartLib() {
  if (chartLib) return chartLib
  const mod = await import('chart.js')
  mod.Chart.register(...mod.registerables)
  chartLib = mod.Chart
  return chartLib
}

function bucketLabel(ts: string): string {
  const d = new Date(ts)
  if (Number.isNaN(d.getTime())) return ts
  return props.period === '1h'
    ? d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    : d.toLocaleString([], { day: '2-digit', month: '2-digit', hour: '2-digit' })
}

async function render() {
  const Chart = await ensureChartLib()
  if (!canvasEl.value) return
  if (chart) {
    chart.destroy()
    chart = null
  }
  const labels = props.timeseries.map((p) => bucketLabel(p.timestamp))
  const human = props.timeseries.map((p) => Number(p.human) || 0)
  const bot = props.timeseries.map((p) => Number(p.bot) || 0)
  chart = new Chart(canvasEl.value.getContext('2d'), {
    type: 'bar',
    data: {
      labels,
      datasets: [
        { label: 'Humain', data: human, backgroundColor: '#378ADD', stack: 'traffic' },
        { label: 'Bot/scan', data: bot, backgroundColor: '#E24B4A', stack: 'traffic' },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: {
          position: 'bottom',
          onHover: (_e: any, _item: any, legend: any) => {
            legend.chart.canvas.style.cursor = 'pointer'
          },
          onLeave: (_e: any, _item: any, legend: any) => {
            legend.chart.canvas.style.cursor = 'default'
          },
        },
      },
      scales: { x: { stacked: true, grid: { display: false } }, y: { stacked: true } },
    },
  })
}

onMounted(() => {
  void render()
})

watch(
  () => props.timeseries,
  () => {
    void render()
  },
  { deep: true }
)

onBeforeUnmount(() => {
  if (chart) chart.destroy()
})
</script>
