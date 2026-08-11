<template>
  <div class="card">
    <div class="card-header">
      <h3 class="card-title mb-0">
        {{ title }}
      </h3>
    </div>
    <div class="card-body proxmox-chart-body">
      <ApexChart
        v-if="series && series.length"
        type="area"
        height="100%"
        :options="options"
        :series="series"
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

<script setup lang="ts">
import { defineAsyncComponent } from 'vue'
import type { ApexOptions } from 'apexcharts'

export type RRDChartSeries = NonNullable<ApexOptions['series']>

withDefaults(defineProps<{
  title: string
  series?: RRDChartSeries | null
  options?: ApexOptions
  emptyText?: string
}>(), {
  series: null,
  options: () => ({}),
  emptyText: 'Aucune donnée',
})

const ApexChart = defineAsyncComponent(() => import('vue3-apexcharts').then((m) => m.default))
</script>
