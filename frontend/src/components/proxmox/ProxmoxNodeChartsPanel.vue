<template>
  <div class="row row-cards mb-4">
    <div class="col-12 col-lg-4">
      <div class="card">
        <div class="card-header d-flex align-items-center justify-content-between">
          <h3 class="card-title mb-0">
            CPU
          </h3>
          <div
            v-if="!loading"
            class="btn-group btn-group-sm"
          >
            <button
              v-for="opt in timeframeOptions"
              :key="opt.value"
              :class="timeframe === opt.value ? 'btn btn-primary' : 'btn btn-outline-secondary'"
              @click="changeTimeframe(opt.value)"
            >
              {{ opt.label }}
            </button>
          </div>
          <span
            v-else
            class="spinner-border spinner-border-sm text-muted"
          />
        </div>
        <div
          class="card-body"
          style="height:11rem"
        >
          <Line
            v-if="cpuChart"
            :data="cpuChart"
            :options="pctOptions"
            class="h-100"
          />
          <div
            v-else
            class="h-100 d-flex align-items-center justify-content-center text-secondary small"
          >
            {{ error || 'Aucune donnée' }}
          </div>
        </div>
      </div>
    </div>
    <div class="col-12 col-lg-4">
      <div class="card">
        <div class="card-header">
          <h3 class="card-title mb-0">
            RAM
          </h3>
        </div>
        <div
          class="card-body"
          style="height:11rem"
        >
          <Line
            v-if="ramChart"
            :data="ramChart"
            :options="ramOptions"
            class="h-100"
          />
          <div
            v-else
            class="h-100 d-flex align-items-center justify-content-center text-secondary small"
          >
            {{ error || 'Aucune donnée' }}
          </div>
        </div>
      </div>
    </div>
    <div class="col-12 col-lg-4">
      <div class="card">
        <div class="card-header">
          <h3 class="card-title mb-0">
            IO Wait
          </h3>
        </div>
        <div
          class="card-body"
          style="height:11rem"
        >
          <Line
            v-if="iowaitChart"
            :data="iowaitChart"
            :options="pctOptions"
            class="h-100"
          />
          <div
            v-else
            class="h-100 d-flex align-items-center justify-content-center text-secondary small"
          >
            {{ error || 'Aucune donnée' }}
          </div>
        </div>
      </div>
    </div>
    <div class="col-12 col-lg-4">
      <div class="card">
        <div class="card-header">
          <h3 class="card-title mb-0">
            Réseau
          </h3>
        </div>
        <div
          class="card-body"
          style="height:11rem"
        >
          <Line
            v-if="netChart"
            :data="netChart"
            :options="netOptions"
            class="h-100"
          />
          <div
            v-else
            class="h-100 d-flex align-items-center justify-content-center text-secondary small"
          >
            {{ error || 'Aucune donnée' }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { defineProps, defineEmits, computed } from 'vue'
import { Line } from 'vue-chartjs'

const props = defineProps({
  cpuChart: { type: Object, default: null },
  ramChart: { type: Object, default: null },
  iowaitChart: { type: Object, default: null },
  netChart: { type: Object, default: null },
  timeframe: { type: String, default: 'hour' },
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
})

const emit = defineEmits(['timeframe-changed'])

const timeframeOptions = [
  { label: '1h', value: 'hour' },
  { label: 'jour', value: 'day' },
  { label: 'mois', value: 'month' },
  { label: 'année', value: 'year' },
]

const pctOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: { legend: { display: false } },
  scales: { y: { min: 0, max: 100, ticks: { callback: v => v + '%' } } },
}

const ramOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: { legend: { display: false } },
}

const netOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: { legend: { display: true, position: 'top' } },
}

const timeframeLabel = computed(() => {
  const opt = timeframeOptions.find(o => o.value === props.timeframe)
  return opt?.label || '1h'
})

function changeTimeframe(val) {
  emit('timeframe-changed', val)
}
</script>
