<template>
  <div class="border p-3 rounded">
    <div class="d-flex align-items-center justify-content-between mb-2">
      <label class="form-label mb-0">Expression cron</label>
      <div class="btn-group btn-group-sm">
        <button type="button" class="btn" :class="expertMode ? 'btn-outline-secondary' : 'btn-secondary'" @click="expertMode = false">
          Visuel
        </button>
        <button type="button" class="btn" :class="expertMode ? 'btn-secondary' : 'btn-outline-secondary'" @click="expertMode = true">
          Expert
        </button>
      </div>
    </div>

    <!-- Expert mode: raw input -->
    <div v-if="expertMode">
      <input
        :value="modelValue"
        type="text"
        class="form-control font-monospace"
        placeholder="0 3 * * 1"
        @input="$emit('update:modelValue', $event.target.value)"
      />
      <div class="form-hint">Format : minute heure jour-du-mois mois jour-de-la-semaine</div>
    </div>

    <!-- Visual builder -->
    <div v-else>
      <div class="mb-2">
        <label class="form-label text-secondary small mb-1">Fréquence</label>
        <div class="d-flex flex-wrap gap-2">
          <button
            v-for="f in frequencies"
            :key="f.key"
            type="button"
            class="btn btn-sm"
            :class="frequency === f.key ? 'btn-primary' : 'btn-outline-secondary'"
            @click="setFrequency(f.key)"
          >
            {{ f.label }}
          </button>
        </div>
      </div>

      <!-- Days of week (hebdomadaire / personnalisé) -->
      <div v-if="frequency === 'weekly' || frequency === 'custom'" class="mb-2">
        <label class="form-label text-secondary small mb-1">Jours</label>
        <div class="d-flex flex-wrap gap-2">
          <label v-for="d in daysOfWeek" :key="d.value" class="form-check form-check-inline mb-0">
            <input
              type="checkbox"
              class="form-check-input"
              :checked="selectedDays.includes(d.value)"
              @change="toggleDay(d.value)"
            />
            <span class="form-check-label">{{ d.label }}</span>
          </label>
        </div>
      </div>

      <!-- Day of month (mensuel) -->
      <div v-if="frequency === 'monthly'" class="mb-2">
        <label class="form-label text-secondary small mb-1">Jour du mois</label>
        <select v-model="dayOfMonth" class="form-select form-select-sm w-auto" @change="buildCron">
          <option v-for="d in 28" :key="d" :value="d">{{ d }}</option>
        </select>
      </div>

      <!-- Hour + Minute -->
      <div class="d-flex gap-3 mb-2">
        <div>
          <label class="form-label text-secondary small mb-1">Heure</label>
          <select v-model="hour" class="form-select form-select-sm" @change="buildCron">
            <option v-for="h in 24" :key="h - 1" :value="h - 1">{{ String(h - 1).padStart(2, '0') }}</option>
          </select>
        </div>
        <div>
          <label class="form-label text-secondary small mb-1">Minute</label>
          <select v-model="minute" class="form-select form-select-sm" @change="buildCron">
            <option v-for="m in minuteOptions" :key="m" :value="m">{{ String(m).padStart(2, '0') }}</option>
          </select>
        </div>
      </div>

      <!-- Preview -->
      <div v-if="preview" class="form-hint text-primary">
        <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="me-1">
          <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
        </svg>
        {{ preview }}
        <code class="ms-2 text-muted small">{{ modelValue }}</code>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'

const props = defineProps({
  modelValue: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['update:modelValue'])

const expertMode = ref(false)
const frequency = ref('daily')
const selectedDays = ref([1]) // monday by default
const hour = ref(3)
const minute = ref(0)
const dayOfMonth = ref(1)

const frequencies = [
  { key: 'daily', label: 'Quotidien' },
  { key: 'weekly', label: 'Hebdomadaire' },
  { key: 'monthly', label: 'Mensuel' },
  { key: 'custom', label: 'Personnalisé' }
]

const daysOfWeek = [
  { value: 1, label: 'Lun' },
  { value: 2, label: 'Mar' },
  { value: 3, label: 'Mer' },
  { value: 4, label: 'Jeu' },
  { value: 5, label: 'Ven' },
  { value: 6, label: 'Sam' },
  { value: 0, label: 'Dim' }
]

const minuteOptions = [0, 5, 10, 15, 20, 30, 45, 59]

const dayNames = ['dim', 'lun', 'mar', 'mer', 'jeu', 'ven', 'sam']

const preview = computed(() => {
  const expr = props.modelValue?.trim()
  if (!expr) return ''
  const presets = {
    '@daily': 'tous les jours à minuit',
    '@hourly': 'toutes les heures',
    '@weekly': 'hebdomadaire (dimanche minuit)',
    '@monthly': 'mensuel (1er du mois à minuit)',
    '@yearly': 'annuel (1er janvier à minuit)'
  }
  if (presets[expr]) return presets[expr]
  const parts = expr.split(' ')
  if (parts.length !== 5) return ''
  const [min, hr, dom, , dow] = parts

  if (dom === '*' && dow === '*' && hr !== '*' && min !== '*') {
    return `tous les jours à ${hr.padStart(2, '0')}h${min.padStart(2, '0')}`
  }
  if (dom !== '*' && dow === '*' && hr !== '*' && min !== '*') {
    return `le ${dom} de chaque mois à ${hr.padStart(2, '0')}h${min.padStart(2, '0')}`
  }
  if (dom === '*' && dow !== '*') {
    const days = dow.split(',').map(d => {
      const n = parseInt(d)
      return !isNaN(n) && n <= 6 ? dayNames[n] : d
    })
    const dayStr = days.join(', ')
    if (hr !== '*' && min !== '*') {
      return `chaque ${dayStr} à ${hr.padStart(2, '0')}h${min.padStart(2, '0')}`
    }
    return `chaque ${dayStr}`
  }
  return ''
})

function buildCron() {
  const m = String(minute.value)
  const h = String(hour.value)
  let cron = ''

  if (frequency.value === 'daily') {
    cron = `${m} ${h} * * *`
  } else if (frequency.value === 'weekly') {
    const days = selectedDays.value.length ? selectedDays.value.sort((a, b) => a - b).join(',') : '1'
    cron = `${m} ${h} * * ${days}`
  } else if (frequency.value === 'monthly') {
    cron = `${m} ${h} ${dayOfMonth.value} * *`
  } else if (frequency.value === 'custom') {
    const days = selectedDays.value.length ? selectedDays.value.sort((a, b) => a - b).join(',') : '*'
    cron = `${m} ${h} * * ${days}`
  }

  emit('update:modelValue', cron)
}

function setFrequency(f) {
  frequency.value = f
  if (f === 'weekly' && selectedDays.value.length === 0) {
    selectedDays.value = [1]
  }
  buildCron()
}

function toggleDay(d) {
  const idx = selectedDays.value.indexOf(d)
  if (idx === -1) {
    selectedDays.value = [...selectedDays.value, d]
  } else {
    selectedDays.value = selectedDays.value.filter(x => x !== d)
  }
  buildCron()
}

// Parse incoming cron expression to populate the visual builder
function parseCron(expr) {
  if (!expr) return
  const presets = ['@daily', '@hourly', '@weekly', '@monthly', '@yearly']
  if (presets.includes(expr)) {
    expertMode.value = true
    return
  }
  const parts = expr.split(' ')
  if (parts.length !== 5) {
    expertMode.value = true
    return
  }
  const [min, hr, dom, , dow] = parts

  // Try to parse numbers
  const minN = parseInt(min)
  const hrN = parseInt(hr)
  if (!isNaN(minN) && minuteOptions.includes(minN)) minute.value = minN
  else if (!isNaN(minN)) { minute.value = minN }
  if (!isNaN(hrN)) hour.value = hrN

  if (dom !== '*') {
    const domN = parseInt(dom)
    if (!isNaN(domN) && dow === '*') {
      frequency.value = 'monthly'
      dayOfMonth.value = domN
      return
    }
    expertMode.value = true
    return
  }

  if (dow === '*') {
    frequency.value = 'daily'
  } else {
    const days = dow.split(',').map(d => parseInt(d)).filter(d => !isNaN(d))
    selectedDays.value = days
    if (days.length === 1) {
      frequency.value = 'weekly'
    } else {
      frequency.value = 'custom'
    }
  }
}

onMounted(() => {
  parseCron(props.modelValue)
})

watch(() => props.modelValue, (val) => {
  // Only re-parse when switching to visual from expert or on init
  // Don't re-parse on every emit to avoid infinite loop
}, { immediate: false })
</script>


