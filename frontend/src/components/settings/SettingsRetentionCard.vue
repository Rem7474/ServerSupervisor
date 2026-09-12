<template>
  <div class="card h-100">
    <div class="card-header">
      <h3 class="card-title">
        {{ t('settings.retentionTitle') }}
      </h3>
    </div>
    <div class="card-body">
      <div class="mb-3">
        <label class="form-label">{{ t('settings.metricsLabel') }}</label>
        <input
          v-model.number="form.metricsRetentionDays"
          type="number"
          class="form-control"
          min="1"
          max="365"
          aria-describedby="metrics-retention-hint"
        >
        <div
          id="metrics-retention-hint"
          class="form-hint"
        >
          {{ t('settings.metricsRetentionHint') }}
        </div>
      </div>
      <div class="mb-3">
        <label class="form-label">{{ t('settings.auditDefaultLabel') }}</label>
        <input
          v-model.number="form.auditRetentionDays"
          type="number"
          class="form-control"
          min="1"
          max="3650"
          aria-describedby="audit-retention-hint"
        >
        <div
          id="audit-retention-hint"
          class="form-hint"
        >
          {{ t('settings.auditRetentionHint') }}
        </div>
      </div>
      <div
        v-if="auditCategories.length > 0"
        class="mb-0"
      >
        <label class="form-label">{{ t('settings.auditByCategoryLabel') }}</label>
        <div class="row g-2">
          <div
            v-for="cat in auditCategories"
            :key="cat.key"
            class="col-6 col-md-3"
          >
            <div class="input-group input-group-flat">
              <span class="input-group-text">{{ cat.label }}</span>
              <input
                type="number"
                class="form-control"
                min="1"
                max="3650"
                :placeholder="String(form.auditRetentionDays)"
                :value="form.auditRetentionDaysByCategory[cat.key] ?? ''"
                @input="onCategoryDaysInput(cat.key, ($event.target as HTMLInputElement).value)"
              >
            </div>
          </div>
        </div>
        <div class="form-hint">
          {{ t('settings.categoryDaysHint') }}
        </div>
      </div>
    </div>
    <div class="card-footer d-flex align-items-center gap-2">
      <button
        v-if="authIsAdmin"
        type="button"
        class="btn btn-primary"
        :disabled="savingRetention"
        @click="$emit('save')"
      >
        {{ savingRetention ? t('common.saving') : t('common.save') }}
      </button>
      <span
        v-if="retentionSaveMsg"
        :class="['ms-auto small', retentionSaveOk ? 'text-success' : 'text-danger']"
      >
        {{ retentionSaveMsg }}
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

interface RetentionForm {
  metricsRetentionDays: number
  auditRetentionDays: number
  auditRetentionDaysByCategory: Record<string, number>
}

const props = withDefaults(defineProps<{
  form: RetentionForm
  auditCategories?: { key: string; label: string }[]
  authIsAdmin?: boolean
  savingRetention?: boolean
  retentionSaveMsg?: string
  retentionSaveOk?: boolean
}>(), {
  auditCategories: () => [],
  authIsAdmin: false,
  savingRetention: false,
  retentionSaveMsg: '',
  retentionSaveOk: false,
})

defineEmits<{
  (e: 'save'): void
}>()

// A blank input removes the category's override (falls back to the default
// above) rather than persisting 0/NaN — v-model.number can't express "no
// value" cleanly here since 0 is falsy but a real (invalid) retention value.
function onCategoryDaysInput(key: string, raw: string): void {
  if (raw === '') {
    delete props.form.auditRetentionDaysByCategory[key]
    return
  }
  const n = Number(raw)
  if (Number.isFinite(n) && n > 0) {
    props.form.auditRetentionDaysByCategory[key] = n
  }
}
</script>

