<template>
  <div class="card">
    <div class="card-header">
      <h3 class="card-title mb-0">
        {{ title }}
      </h3>
    </div>
    <div class="card-body proxmox-chart-body">
      <Line
        v-if="chartData"
        :data="chartData"
        :options="options"
        class="h-100"
      />
      <div
        v-else
        class="h-100 d-flex align-items-center justify-content-center text-secondary small"
      >
        {{ emptyText }}
      </div>
    </div>
  </div>
</template>

<script setup>
import { defineAsyncComponent } from 'vue'

defineProps({
  title: { type: String, required: true },
  chartData: { type: Object, default: null },
  options: { type: Object, default: () => ({}) },
  emptyText: { type: String, default: 'Aucune donnée' },
})

const Line = defineAsyncComponent(async () => {
  const [{ Line }, { Chart: ChartJS, LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip }] = await Promise.all([
    import('vue-chartjs'),
    import('chart.js'),
  ])
  ChartJS.register(LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip)
  return Line
})
</script>
