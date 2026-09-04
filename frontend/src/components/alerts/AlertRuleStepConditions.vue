<template>
  <div>
    <!-- ── docker_container_state ───────────────────────────────────── -->
    <template v-if="form.metric === 'docker_container_state'">
      <div class="row">
        <div class="col-md-6 mb-3">
          <div class="form-label">
            {{ t('alerts.conditionsDockerStatesLabel') }} <span class="badge bg-warning-lt text-warning ms-1">warn</span>
          </div>
          <div class="border rounded p-2 d-flex flex-wrap gap-2">
            <label
              v-for="s in DOCKER_STATES"
              :key="'w-' + s"
              class="form-check form-check-inline mb-0"
            >
              <input
                type="checkbox"
                class="form-check-input"
                :checked="form.docker_scope.warn_states.includes(s)"
                @change="toggleState('warn', s, ($event.target as HTMLInputElement).checked)"
              >
              <span class="form-check-label"><code>{{ s }}</code></span>
            </label>
          </div>
        </div>
        <div class="col-md-6 mb-3">
          <div class="form-label">
            {{ t('alerts.conditionsDockerStatesLabel') }} <span class="badge bg-danger-lt text-danger ms-1">crit</span>
          </div>
          <div class="border rounded p-2 d-flex flex-wrap gap-2">
            <label
              v-for="s in DOCKER_STATES"
              :key="'c-' + s"
              class="form-check form-check-inline mb-0"
            >
              <input
                type="checkbox"
                class="form-check-input"
                :checked="form.docker_scope.crit_states.includes(s)"
                @change="toggleState('crit', s, ($event.target as HTMLInputElement).checked)"
              >
              <span class="form-check-label"><code>{{ s }}</code></span>
            </label>
          </div>
        </div>
      </div>
      <div
        v-if="dockerStateNoSelection"
        class="alert alert-warning py-2 small mb-3"
      >
        {{ t('alerts.conditionsDockerNoSelectionWarning') }}
      </div>
      <div class="mb-3">
        <label
          for="alert-cond-duration"
          class="form-label"
        >{{ t('alerts.conditionsDurationSecondsLabel') }}</label>
        <input
          id="alert-cond-duration"
          v-model.number="durationModel"
          type="number"
          min="0"
          class="form-control"
          placeholder="0"
          :aria-describedby="`duration-hint-${rule?.id || 'new'}`"
        >
        <small
          :id="`duration-hint-${rule?.id || 'new'}`"
          class="form-hint"
        >
          {{ t('alerts.conditionsDockerDurationHint') }}
        </small>
      </div>
    </template>

    <!-- ── docker_compose_degraded_services ────────────────────────── -->
    <template v-else-if="form.metric === 'docker_compose_degraded_services'">
      <div class="alert alert-light py-2 small mb-3 border">
        {{ t('alerts.conditionsComposeInfoLine1') }} <strong>{{ t('alerts.conditionsComposeInfoLine1Strong') }}</strong>.
        {{ t('alerts.conditionsComposeInfoLine2') }}
        {{ t('alerts.conditionsComposeInfoLine3') }}
      </div>
      <div class="row">
        <div class="col-md-6 mb-3">
          <label
            for="alert-cond-threshold-warn"
            class="form-label required"
          >{{ t('alerts.conditionsComposeWarnThresholdLabel') }}</label>
          <input
            id="alert-cond-threshold-warn"
            v-model.number="thresholdWarnModel"
            type="number"
            step="1"
            min="1"
            class="form-control"
            placeholder="1"
          >
          <small class="form-hint">
            {{ t('alerts.conditionsComposeWarnThresholdHint', { n: form.threshold_warn ?? 1 }, form.threshold_warn ?? 1) }}
          </small>
        </div>
        <div class="col-md-6 mb-3">
          <label
            for="alert-cond-threshold-crit"
            class="form-label required"
          >{{ t('alerts.conditionsComposeCritThresholdLabel') }}</label>
          <input
            id="alert-cond-threshold-crit"
            v-model.number="thresholdCritModel"
            type="number"
            step="1"
            min="1"
            class="form-control"
            placeholder="1"
          >
          <small class="form-hint">
            {{ t('alerts.conditionsComposeCritThresholdHint', { n: form.threshold_crit ?? 1 }, form.threshold_crit ?? 1) }}
          </small>
        </div>
      </div>
      <div class="mb-3">
        <label
          for="alert-cond-compose-duration"
          class="form-label"
        >{{ t('alerts.conditionsDurationSecondsLabel') }}</label>
        <input
          id="alert-cond-compose-duration"
          v-model.number="durationModel"
          type="number"
          min="0"
          class="form-control"
          placeholder="0"
          :aria-describedby="`duration-hint-${rule?.id || 'new'}`"
        >
        <small
          :id="`duration-hint-${rule?.id || 'new'}`"
          class="form-hint"
        >
          {{ t('alerts.conditionsComposeDurationHint') }}
        </small>
      </div>
    </template>

    <!-- ── bandwidth_vs_rolling_avg ─────────────────────────────────── -->
    <template v-else-if="form.metric === 'bandwidth_vs_rolling_avg'">
      <div class="alert alert-light py-2 small mb-3 border">
        {{ t('alerts.conditionsBandwidthInfoLine1') }} <strong>{{ t('alerts.conditionsBandwidthInfoLine1Strong') }}</strong>
        {{ t('alerts.conditionsBandwidthInfoLine2') }}
      </div>
      <div class="row">
        <div class="col-md-12 mb-3">
          <label
            for="alert-cond-baseline-window"
            class="form-label required"
          >{{ t('alerts.baselineWindowLabel') }}</label>
          <select
            id="alert-cond-baseline-window"
            v-model.number="baselineWindowModel"
            class="form-select"
          >
            <option :value="3600">
              {{ t('alerts.oneHourOption') }}
            </option>
            <option :value="21600">
              {{ t('alerts.sixHoursOption') }}
            </option>
            <option :value="86400">
              {{ t('alerts.twentyFourHoursOption') }}
            </option>
          </select>
          <small class="form-hint">
            {{ t('alerts.conditionsBandwidthWindowHint') }}
          </small>
        </div>
      </div>
      <div class="row">
        <div class="col-md-6 mb-3">
          <label
            for="alert-cond-bandwidth-threshold-warn"
            class="form-label required"
          >{{ t('alerts.conditionsBandwidthWarnThresholdLabel') }}</label>
          <input
            id="alert-cond-bandwidth-threshold-warn"
            v-model.number="thresholdWarnModel"
            type="number"
            step="1"
            min="0"
            class="form-control"
            placeholder="150"
          >
          <small class="form-hint">{{ t('alerts.conditionsBandwidthWarnThresholdHint') }}</small>
        </div>
        <div class="col-md-6 mb-3">
          <label
            for="alert-cond-bandwidth-threshold-crit"
            class="form-label required"
          >{{ t('alerts.conditionsBandwidthCritThresholdLabel') }}</label>
          <input
            id="alert-cond-bandwidth-threshold-crit"
            v-model.number="thresholdCritModel"
            type="number"
            step="1"
            min="0"
            class="form-control"
            placeholder="200"
          >
          <small class="form-hint">{{ t('alerts.conditionsBandwidthCritThresholdHint') }}</small>
        </div>
      </div>
    </template>

    <!-- ── heartbeat_timeout ────────────────────────────────────────── -->
    <template v-else-if="form.metric === 'heartbeat_timeout'">
      <div class="row">
        <div class="col-md-12 mb-3">
          <label
            for="alert-cond-heartbeat-timeout"
            class="form-label required"
          >{{ t('alerts.conditionsHeartbeatLabel') }}</label>
          <input
            id="alert-cond-heartbeat-timeout"
            v-model.number="thresholdCritModel"
            type="number"
            step="60"
            class="form-control"
            placeholder="300"
            :aria-describedby="`heartbeat-hint-${rule?.id || 'new'}`"
          >
          <small
            :id="`heartbeat-hint-${rule?.id || 'new'}`"
            class="form-hint"
          >
            {{ t('alerts.conditionsHeartbeatHint') }}
          </small>
        </div>
      </div>
    </template>

    <!-- ── Generic form (agent / proxmox / synthetic) ───────────────── -->
    <template v-else>
      <div class="row">
        <div class="col-md-6 mb-3">
          <label
            for="alert-cond-operator"
            class="form-label required"
          >{{ t('alerts.conditionsOperatorLabel') }}</label>
          <select
            id="alert-cond-operator"
            v-model="operatorModel"
            class="form-select"
          >
            <option value=">">
              {{ t('alerts.conditionsOperatorGreaterThan') }}
            </option>
            <option value=">=">
              {{ t('alerts.conditionsOperatorGreaterEqual') }}
            </option>
            <option value="<">
              {{ t('alerts.conditionsOperatorLessThan') }}
            </option>
            <option value="<=">
              {{ t('alerts.conditionsOperatorLessEqual') }}
            </option>
          </select>
        </div>
      </div>

      <div class="row">
        <div class="col-md-6 mb-3">
          <label
            for="alert-cond-generic-threshold-warn"
            class="form-label required"
          >{{ t('alerts.conditionsGenericWarnThresholdLabel') }}</label>
          <input
            id="alert-cond-generic-threshold-warn"
            v-model.number="thresholdWarnModel"
            type="number"
            step="0.1"
            class="form-control"
            placeholder="70"
          >
          <small class="form-hint">{{ t('alerts.conditionsGenericWarnThresholdHint') }}</small>
        </div>
        <div class="col-md-6 mb-3">
          <label
            for="alert-cond-generic-threshold-crit"
            class="form-label required"
          >{{ t('alerts.conditionsGenericCritThresholdLabel') }}</label>
          <input
            id="alert-cond-generic-threshold-crit"
            v-model.number="thresholdCritModel"
            type="number"
            step="0.1"
            class="form-control"
            placeholder="85"
          >
          <small class="form-hint">{{ t('alerts.conditionsGenericCritThresholdHint') }}</small>
        </div>
      </div>

      <div class="row">
        <div class="col-md-6 mb-3">
          <label
            for="alert-cond-threshold-clear-warn"
            class="form-label"
          >{{ t('alerts.conditionsClearWarnThresholdLabel') }}</label>
          <input
            id="alert-cond-threshold-clear-warn"
            v-model.number="thresholdClearWarnModel"
            type="number"
            step="0.1"
            class="form-control"
            :class="{ 'is-invalid': clearWarnIncoherent }"
            :placeholder="clearPlaceholder('warn')"
            :aria-describedby="`threshold-clear-warn-hint-${rule?.id || 'new'}`"
          >
          <small
            v-if="clearWarnIncoherent"
            class="invalid-feedback d-block"
          >
            {{ incoherenceMessage('warn') }}
          </small>
          <small
            :id="`threshold-clear-warn-hint-${rule?.id || 'new'}`"
            class="form-hint"
          >
            {{ t('alerts.conditionsClearThresholdHintBefore') }} <strong>{{ t('alerts.conditionsClearThresholdHintExact') }}</strong> {{ t('alerts.conditionsClearThresholdHintAfter', { sev: 'warn', example: clearExample('warn') }) }}
          </small>
        </div>
        <div class="col-md-6 mb-3">
          <label
            for="alert-cond-threshold-clear-crit"
            class="form-label"
          >{{ t('alerts.conditionsClearCritThresholdLabel') }}</label>
          <input
            id="alert-cond-threshold-clear-crit"
            v-model.number="thresholdClearCritModel"
            type="number"
            step="0.1"
            class="form-control"
            :class="{ 'is-invalid': clearCritIncoherent }"
            :placeholder="clearPlaceholder('crit')"
            :aria-describedby="`threshold-clear-crit-hint-${rule?.id || 'new'}`"
          >
          <small
            v-if="clearCritIncoherent"
            class="invalid-feedback d-block"
          >
            {{ incoherenceMessage('crit') }}
          </small>
          <small
            :id="`threshold-clear-crit-hint-${rule?.id || 'new'}`"
            class="form-hint"
          >
            {{ t('alerts.conditionsClearThresholdHintBefore') }} <strong>{{ t('alerts.conditionsClearThresholdHintExact') }}</strong> {{ t('alerts.conditionsClearThresholdHintAfter', { sev: 'crit', example: clearExample('crit') }) }}
          </small>
        </div>
      </div>

      <div class="mb-3">
        <label
          for="alert-cond-generic-duration"
          class="form-label"
        >{{ t('alerts.conditionsDurationSecondsLabel') }}</label>
        <input
          id="alert-cond-generic-duration"
          v-model.number="durationModel"
          type="number"
          class="form-control"
          placeholder="300"
          :aria-describedby="`duration-hint-${rule?.id || 'new'}`"
        >
        <small
          :id="`duration-hint-${rule?.id || 'new'}`"
          class="form-hint"
        >{{ t('alerts.conditionsGenericDurationHint') }}</small>
        <small
          v-if="Number.isFinite(Number(form.duration)) && form.duration > 0 && form.duration < 60"
          :id="`duration-warn-${rule?.id || 'new'}`"
          class="form-hint text-warning d-block mt-1"
        >
          {{ t('alerts.conditionsGenericDurationLowWarning') }}
        </small>
      </div>
    </template>

    <!-- ── Test results (all metrics) ───────────────────────────────── -->
    <div
      v-if="testResults"
      class="mt-3"
    >
      <div
        v-if="hasNoDataResults"
        class="alert alert-warning py-2 small mb-2"
      >
        <strong>{{ t('alerts.conditionsNoDataResultsBold') }}</strong> {{ t('alerts.conditionsNoDataResultsSuffix') }}
      </div>
      <div class="d-flex align-items-center justify-content-between mb-2">
        <div class="fw-bold">
          {{ t('alerts.conditionsTestResultTitle') }}
          <span
            v-if="testResults.any_fires"
            class="badge bg-danger-lt text-danger ms-2"
          >{{ t('alerts.conditionsTestFiresBadge') }}</span>
          <span
            v-else
            class="badge bg-success-lt text-success ms-2"
          >{{ t('alerts.conditionsTestNoFireBadge') }}</span>
        </div>
        <div class="d-flex align-items-center gap-2">
          <button
            v-if="canDownloadTestLogs"
            type="button"
            class="btn btn-sm btn-outline-secondary"
            :disabled="downloadingLogs"
            @click="emit('download-logs')"
          >
            <span
              v-if="downloadingLogs"
              class="spinner-border spinner-border-sm me-1"
            />
            {{ t('alerts.conditionsDownloadLogsButton') }}
          </button>
          <span class="text-secondary small">{{ formatDate(testResults.evaluated_at) }}</span>
        </div>
      </div>
      <div class="table-responsive">
        <table class="table table-sm table-vcenter card-table">
          <thead>
            <tr>
              <th>{{ testResultColLabel }}</th>
              <th>{{ t('alerts.conditionsCurrentValueColumn') }}</th>
              <th>{{ t('alerts.conditionsResultColumn') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!testResults.results?.length">
              <td colspan="3">
                <EmptyState :title="t('alerts.conditionsNoHostsConcernedTitle')" />
              </td>
            </tr>
            <tr
              v-for="result in testResults.results"
              :key="result.host_id"
            >
              <td class="fw-medium">
                {{ result.host_name }}
              </td>
              <td>
                <span v-if="result.has_data">{{ formatTestValue(result.current_value) }}</span>
                <span
                  v-else
                  class="text-secondary"
                >{{ t('alerts.conditionsNoDataLabel') }}</span>
              </td>
              <td>
                <span
                  v-if="result.would_fire"
                  class="badge bg-danger-lt text-danger"
                >{{ t('alerts.conditionsAlertBadge') }}</span>
                <span
                  v-else
                  class="badge bg-success-lt text-success"
                >{{ t('alerts.valueOk') }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import EmptyState from '../EmptyState.vue'
import type { AlertRuleFormData } from '../../composables/useAlertRuleForm'

interface TestResultRow {
  host_id: string
  host_name: string
  has_data: boolean
  current_value: number
  would_fire: boolean
}
interface TestResults {
  any_fires?: boolean
  evaluated_at?: string
  results?: TestResultRow[]
}

const props = defineProps<{
  form: AlertRuleFormData
  rule?: { id?: number | string } | null
  testResults?: TestResults | null
  hasNoDataResults?: boolean
  canDownloadTestLogs?: boolean
  downloadingLogs?: boolean
  metricUnit?: string
}>()

const emit = defineEmits<{
  (e: 'download-logs'): void
  (e: 'update:form', value: AlertRuleFormData): void
}>()

const { t, locale } = useI18n()

type Severity = 'warn' | 'crit'

// ── Shared form-field emit helpers ───────────────────────────────────
// The `form` prop is owned by the parent (AlertRuleModal, via
// useAlertRuleForm). This component never mutates it in place — every
// field write emits a whole-object replacement for the parent to apply
// (bound as `v-model:form` there), which also keeps sibling reads (e.g.
// AlertRuleStepSource) consistent.

function updateForm<K extends keyof AlertRuleFormData>(key: K, value: AlertRuleFormData[K]): void {
  emit('update:form', { ...props.form, [key]: value })
}

function updateDockerScope<K extends keyof AlertRuleFormData['docker_scope']>(
  key: K,
  value: AlertRuleFormData['docker_scope'][K],
): void {
  emit('update:form', { ...props.form, docker_scope: { ...props.form.docker_scope, [key]: value } })
}

function fieldModel<K extends keyof AlertRuleFormData>(key: K) {
  return computed<AlertRuleFormData[K]>({
    get: () => props.form[key],
    set: (value) => updateForm(key, value),
  })
}

const durationModel = fieldModel('duration')
const thresholdWarnModel = fieldModel('threshold_warn')
const thresholdCritModel = fieldModel('threshold_crit')
const thresholdClearWarnModel = fieldModel('threshold_clear_warn')
const thresholdClearCritModel = fieldModel('threshold_clear_crit')
const baselineWindowModel = fieldModel('baseline_window_seconds')
const operatorModel = fieldModel('operator')

// ── Docker state helpers ─────────────────────────────────────────────

const DOCKER_STATES = ['created', 'paused', 'restarting', 'exited', 'dead'] as const

const dockerStateNoSelection = computed(
  () => props.form.metric === 'docker_container_state' &&
    props.form.docker_scope.warn_states.length === 0 &&
    props.form.docker_scope.crit_states.length === 0
)

function toggleState(level: 'warn' | 'crit', state: string, checked: boolean): void {
  const key = level === 'warn' ? 'warn_states' : 'crit_states'
  const current = props.form.docker_scope[key]
  const next = checked
    ? (current.includes(state) ? current : [...current, state])
    : current.filter((s) => s !== state)
  updateDockerScope(key, next)
}

// ── Generic hysteresis helpers (generic form only) ──────────────────

const isDescending = computed(() => props.form.operator === '<' || props.form.operator === '<=')

function triggerThreshold(sev: Severity): number | null {
  const v = sev === 'warn' ? props.form.threshold_warn : props.form.threshold_crit
  return Number.isFinite(Number(v)) ? Number(v) : null
}

function clearValue(sev: Severity): number | null {
  const v = sev === 'warn' ? props.form.threshold_clear_warn : props.form.threshold_clear_crit
  return Number.isFinite(Number(v)) ? Number(v) : null
}

function isIncoherent(sev: Severity): boolean {
  const trigger = triggerThreshold(sev)
  const clear = clearValue(sev)
  if (trigger === null || clear === null) return false
  return isDescending.value ? clear < trigger : clear > trigger
}

const clearWarnIncoherent = computed(() => isIncoherent('warn'))
const clearCritIncoherent = computed(() => isIncoherent('crit'))

const unitLabel = computed(() => props.metricUnit || '')

function clearExample(sev: Severity): string {
  const trigger = triggerThreshold(sev)
  if (trigger === null) return isDescending.value ? '72' : '68'
  const suggestion = isDescending.value ? trigger + 2 : trigger - 2
  return `${suggestion}${unitLabel.value}`
}

function clearPlaceholder(sev: Severity): string {
  return t('alerts.conditionsClearPlaceholder', { example: clearExample(sev) })
}

function incoherenceMessage(sev: Severity): string {
  const trigger = triggerThreshold(sev)
  const side = isDescending.value ? `≥ ${trigger}${unitLabel.value}` : `≤ ${trigger}${unitLabel.value}`
  return t('alerts.conditionsIncoherenceMessage', { side })
}

// ── Test results display ─────────────────────────────────────────────

const testResultColLabel = computed(() => {
  switch (props.form.metric) {
    case 'docker_container_state': return t('alerts.conditionsContainerColumnLabel')
    case 'docker_compose_degraded_services': return t('alerts.conditionsComposeProjectColumnLabel')
    case 'proxmox_storage_percent': return t('alerts.conditionsStorageColumnLabel')
    default: return props.form.source_type === 'proxmox' ? t('alerts.conditionsScopeColumnLabel') : t('alerts.hostLabel')
  }
})

function formatTestValue(value: number): string {
  switch (props.form.metric) {
    case 'docker_container_state':
      if (value >= 2) return t('alerts.conditionsDockerStateCrit')
      if (value >= 1) return t('alerts.conditionsDockerStateWarn')
      return t('alerts.conditionsDockerStateOk')
    case 'docker_compose_degraded_services':
      return t('alerts.degradedServicesCount', { n: Math.round(value) }, Math.round(value))
    default:
      return `${value.toFixed(1)}${unitLabel.value}`
  }
}

function formatDate(dateStr?: string): string {
  if (!dateStr) return ''
  return new Date(dateStr).toLocaleString(locale.value)
}
</script>
