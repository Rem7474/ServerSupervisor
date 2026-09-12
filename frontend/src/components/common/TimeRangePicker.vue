<template>
  <div class="d-flex align-items-end gap-2 flex-wrap">
    <div class="btn-group btn-group-sm">
      <button
        v-for="p in presets"
        :key="p.value"
        type="button"
        :class="isActivePreset(p.value) ? 'btn btn-primary' : 'btn btn-outline-secondary'"
        :disabled="loading"
        @click="selectPreset(p.value)"
      >
        {{ p.label }}
      </button>
      <button
        type="button"
        :class="modelValue.mode === 'custom' ? 'btn btn-primary' : 'btn btn-outline-secondary'"
        :disabled="loading"
        @click="showCustomFields = !showCustomFields"
      >
        {{ t('common.custom') }}
      </button>
    </div>

    <div
      v-if="showCustomFields"
      class="d-flex align-items-end gap-2 flex-wrap"
    >
      <div>
        <label class="form-label mb-0 small">{{ t('common.from') }}</label>
        <input
          v-model="fromLocal"
          type="datetime-local"
          class="form-control form-control-sm"
        >
      </div>
      <div>
        <label class="form-label mb-0 small">{{ t('common.to') }}</label>
        <input
          v-model="toLocal"
          type="datetime-local"
          class="form-control form-control-sm"
        >
      </div>
      <button
        type="button"
        class="btn btn-sm btn-primary"
        :disabled="loading"
        @click="applyCustom"
      >
        {{ t('common.apply') }}
      </button>
      <div
        v-if="validationError"
        class="text-danger small w-100"
      >
        {{ validationError }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import dayjs from '../../utils/dayjs'
import type { TimeRangeModel, TimeRangePreset } from '../../types/timeRange'

const { t } = useI18n()

export type { TimeRangeModel, TimeRangePreset }

withDefaults(defineProps<{
  presets: TimeRangePreset[]
  loading?: boolean
}>(), {
  loading: false,
})

const modelValue = defineModel<TimeRangeModel>({ required: true })
const emit = defineEmits<{ change: [] }>()

const showCustomFields = ref(false)
const fromLocal = ref('')
const toLocal = ref('')
const validationError = ref('')

// Single source of truth for the datetime-local fields: whenever the model
// is (re)set to a custom range — including externally, e.g. a shared URL
// link restoring from/to before this component ever mounted — reflect it
// here. Always through a UTC→local conversion (dayjs formats in the
// browser's local timezone by default), never by re-injecting the raw ISO
// string: datetime-local doesn't accept it, and a link opened in a
// different timezone would otherwise show the wrong wall-clock time.
watch(
  () => [modelValue.value.mode, modelValue.value.from, modelValue.value.to] as const,
  ([mode, from, to]) => {
    if (mode !== 'custom') return
    showCustomFields.value = true
    if (from) fromLocal.value = dayjs(from).format('YYYY-MM-DDTHH:mm')
    if (to) toLocal.value = dayjs(to).format('YYYY-MM-DDTHH:mm')
  },
  { immediate: true },
)

function isActivePreset(value: string): boolean {
  return modelValue.value.mode === 'preset' && modelValue.value.period === value
}

function selectPreset(value: string): void {
  showCustomFields.value = false
  validationError.value = ''
  modelValue.value = { mode: 'preset', period: value, from: null, to: null }
  emit('change')
}

function applyCustom(): void {
  validationError.value = ''
  if (!fromLocal.value || !toLocal.value) {
    validationError.value = t('common.fillBothDates')
    return
  }
  const from = new Date(fromLocal.value)
  const to = new Date(toLocal.value)
  if (Number.isNaN(from.getTime()) || Number.isNaN(to.getTime())) {
    validationError.value = t('common.invalidDate')
    return
  }
  if (to.getTime() <= from.getTime()) {
    validationError.value = t('common.endAfterStart')
    return
  }
  modelValue.value = {
    mode: 'custom',
    period: modelValue.value.period,
    from: from.toISOString(),
    to: to.toISOString(),
  }
  emit('change')
}
</script>
