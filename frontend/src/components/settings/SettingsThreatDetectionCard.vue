<template>
  <div class="card h-100">
    <div class="card-header">
      <h3 class="card-title">
        {{ t('settings.threatScoreTitle') }}
      </h3>
    </div>
    <div class="card-body">
      <p class="text-secondary small">
        {{ t('settings.threatScoreDesc') }}
      </p>

      <h4 class="mb-2">
        {{ t('settings.weightsByCategory') }}
      </h4>
      <div class="row g-2 mb-3">
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-wordpress"
            class="form-label"
          >WordPress</label>
          <input
            id="threat-weight-wordpress"
            v-model.number="weightWordpressModel"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-adminpanel"
            class="form-label"
          >{{ t('settings.adminPanel') }}</label>
          <input
            id="threat-weight-adminpanel"
            v-model.number="weightAdminpanelModel"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-pathtraversal"
            class="form-label"
          >{{ t('settings.pathTraversal') }}</label>
          <input
            id="threat-weight-pathtraversal"
            v-model.number="weightPathtraversalModel"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-knownscanner"
            class="form-label"
          >{{ t('settings.knownScanner') }}</label>
          <input
            id="threat-weight-knownscanner"
            v-model.number="weightKnownscannerModel"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-suspiciousmethod"
            class="form-label"
          >{{ t('settings.suspiciousMethod') }}</label>
          <input
            id="threat-weight-suspiciousmethod"
            v-model.number="weightSuspiciousmethodModel"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
          >
        </div>
      </div>

      <h4 class="mb-2">
        {{ t('settings.multipliersByStatus') }}
      </h4>
      <div class="row g-2 mb-3">
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-status-2xx"
            class="form-label"
          >{{ t('settings.status2xx') }}</label>
          <input
            id="threat-weight-status-2xx"
            v-model.number="weightStatus2xxModel"
            type="number"
            step="0.05"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-status-3xx"
            class="form-label"
          >{{ t('settings.status3xx') }}</label>
          <input
            id="threat-weight-status-3xx"
            v-model.number="weightStatus3xxModel"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-status-404"
            class="form-label"
          >404</label>
          <input
            id="threat-weight-status-404"
            v-model.number="weightStatus404Model"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-status-4xx"
            class="form-label"
          >{{ t('settings.status4xxOther') }}</label>
          <input
            id="threat-weight-status-4xx"
            v-model.number="weightStatus4xxModel"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-status-5xx"
            class="form-label"
          >5xx</label>
          <input
            id="threat-weight-status-5xx"
            v-model.number="weightStatus5xxModel"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
          >
        </div>
      </div>

      <h4 class="mb-2">
        {{ t('settings.structuralWeighting') }}
      </h4>
      <div class="row g-2 mb-3">
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-breadth"
            class="form-label"
          >{{ t('settings.breadthLabel') }}</label>
          <input
            id="threat-weight-breadth"
            v-model.number="weightBreadthModel"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
            aria-describedby="threat-breadth-hint"
          >
          <div
            id="threat-breadth-hint"
            class="form-hint"
          >
            {{ t('settings.breadthHint') }}
          </div>
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-weight-hits"
            class="form-label"
          >{{ t('settings.requestVolumeLabel') }}</label>
          <input
            id="threat-weight-hits"
            v-model.number="weightHitsModel"
            type="number"
            step="0.1"
            min="0"
            class="form-control"
            aria-describedby="threat-hits-hint"
          >
          <div
            id="threat-hits-hint"
            class="form-hint"
          >
            {{ t('settings.requestVolumeHint') }}
          </div>
        </div>
      </div>

      <h4 class="mb-2">
        {{ t('settings.levelThresholds') }}
      </h4>
      <div class="row g-2">
        <div class="col-6 col-md-4">
          <label
            for="threat-threshold-medium"
            class="form-label"
          >{{ t('settings.mediumFrom') }}</label>
          <input
            id="threat-threshold-medium"
            v-model.number="thresholdMediumModel"
            type="number"
            step="1"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-threshold-high"
            class="form-label"
          >{{ t('settings.highFrom') }}</label>
          <input
            id="threat-threshold-high"
            v-model.number="thresholdHighModel"
            type="number"
            step="1"
            min="0"
            class="form-control"
          >
        </div>
        <div class="col-6 col-md-4">
          <label
            for="threat-threshold-critical"
            class="form-label"
          >{{ t('settings.criticalFrom') }}</label>
          <input
            id="threat-threshold-critical"
            v-model.number="thresholdCriticalModel"
            type="number"
            step="1"
            min="0"
            class="form-control"
          >
        </div>
      </div>
      <div
        v-if="thresholdOrderWarning"
        class="alert alert-warning mt-3 mb-0 py-2"
      >
        {{ thresholdOrderWarning }}
      </div>
    </div>
    <div class="card-footer d-flex align-items-center gap-2">
      <button
        v-if="authIsAdmin"
        type="button"
        class="btn btn-primary"
        :disabled="savingThreatDetection"
        @click="$emit('save')"
      >
        {{ savingThreatDetection ? t('common.saving') : t('common.save') }}
      </button>
      <span
        v-if="threatDetectionSaveMsg"
        :class="['ms-auto small', threatDetectionSaveOk ? 'text-success' : 'text-danger']"
      >
        {{ threatDetectionSaveMsg }}
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

interface ThreatDetectionForm {
  threatWeightWordpress: number
  threatWeightAdminpanel: number
  threatWeightPathtraversal: number
  threatWeightKnownscanner: number
  threatWeightSuspiciousmethod: number
  threatWeightStatus2xx: number
  threatWeightStatus3xx: number
  threatWeightStatus404: number
  threatWeightStatus4xx: number
  threatWeightStatus5xx: number
  threatWeightBreadth: number
  threatWeightHits: number
  threatThresholdMedium: number
  threatThresholdHigh: number
  threatThresholdCritical: number
}

const props = withDefaults(defineProps<{
  form: ThreatDetectionForm
  authIsAdmin?: boolean
  savingThreatDetection?: boolean
  threatDetectionSaveMsg?: string
  threatDetectionSaveOk?: boolean
}>(), {
  authIsAdmin: false,
  savingThreatDetection: false,
  threatDetectionSaveMsg: '',
  threatDetectionSaveOk: false,
})

const emit = defineEmits<{
  (e: 'save'): void
  (e: 'update:form', value: ThreatDetectionForm): void
}>()

// The `form` prop is owned by the parent (SettingsView, via useSettings) and
// shared with the other Settings*Card siblings. This component never
// mutates it in place — every field write emits a whole-object replacement
// for the parent to apply (bound as `v-model:form` there).
function updateForm<K extends keyof ThreatDetectionForm>(key: K, value: ThreatDetectionForm[K]): void {
  emit('update:form', { ...props.form, [key]: value })
}

function fieldModel<K extends keyof ThreatDetectionForm>(key: K) {
  return computed<ThreatDetectionForm[K]>({
    get: () => props.form[key],
    set: (value) => updateForm(key, value),
  })
}

const weightWordpressModel = fieldModel('threatWeightWordpress')
const weightAdminpanelModel = fieldModel('threatWeightAdminpanel')
const weightPathtraversalModel = fieldModel('threatWeightPathtraversal')
const weightKnownscannerModel = fieldModel('threatWeightKnownscanner')
const weightSuspiciousmethodModel = fieldModel('threatWeightSuspiciousmethod')
const weightStatus2xxModel = fieldModel('threatWeightStatus2xx')
const weightStatus3xxModel = fieldModel('threatWeightStatus3xx')
const weightStatus404Model = fieldModel('threatWeightStatus404')
const weightStatus4xxModel = fieldModel('threatWeightStatus4xx')
const weightStatus5xxModel = fieldModel('threatWeightStatus5xx')
const weightBreadthModel = fieldModel('threatWeightBreadth')
const weightHitsModel = fieldModel('threatWeightHits')
const thresholdMediumModel = fieldModel('threatThresholdMedium')
const thresholdHighModel = fieldModel('threatThresholdHigh')
const thresholdCriticalModel = fieldModel('threatThresholdCritical')

const thresholdOrderWarning = computed(() => {
  const { threatThresholdMedium: m, threatThresholdHigh: h, threatThresholdCritical: c } = props.form
  if (m < h && h < c) return ''
  return t('settings.thresholdOrderWarning')
})
</script>
