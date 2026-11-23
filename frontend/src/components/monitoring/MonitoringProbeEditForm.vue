<template>
  <form @submit.prevent="emit('save')">
    <div class="modal-body pt-0">
      <div
        v-if="error"
        class="alert alert-danger"
      >
        {{ error }}
      </div>
      <div class="row g-3">
        <div class="col-md-7">
          <label
            for="monitoring-edit-probe-name"
            class="form-label required"
          >{{ t('monitoring.nameFieldLabel') }}</label>
          <input
            id="monitoring-edit-probe-name"
            v-model="probeForm.name"
            type="text"
            class="form-control"
            placeholder="Ex: API prod"
            required
          >
        </div>
        <div class="col-md-5">
          <label
            for="monitoring-edit-probe-type"
            class="form-label required"
          >{{ t('monitoring.typeFieldLabel') }}</label>
          <select
            id="monitoring-edit-probe-type"
            v-model="probeForm.type"
            class="form-select"
          >
            <option value="http">
              {{ t('monitoring.httpOption') }}
            </option>
            <option value="tcp">
              {{ t('monitoring.tcpOption') }}
            </option>
            <option value="icmp">
              {{ t('monitoring.icmpOption') }}
            </option>
          </select>
        </div>
        <div class="col-12">
          <label
            for="monitoring-edit-probe-target"
            class="form-label required"
          >{{ probeTargetLabel }}</label>
          <input
            id="monitoring-edit-probe-target"
            v-model="probeForm.target"
            type="text"
            class="form-control"
            :placeholder="probeTargetPlaceholder"
            required
          >
          <div
            v-if="probeForm.type === 'icmp'"
            class="form-hint"
          >
            {{ t('monitoring.icmpHint') }}
          </div>
        </div>
        <div class="col-md-4">
          <label
            for="monitoring-edit-probe-interval"
            class="form-label"
          >{{ t('monitoring.intervalSecLabel') }}</label>
          <input
            id="monitoring-edit-probe-interval"
            v-model.number="probeForm.interval_sec"
            type="number"
            min="10"
            class="form-control"
          >
        </div>
        <div class="col-md-4">
          <label
            for="monitoring-edit-probe-timeout"
            class="form-label"
          >{{ t('monitoring.timeoutSecLabel') }}</label>
          <input
            id="monitoring-edit-probe-timeout"
            v-model.number="probeForm.timeout_sec"
            type="number"
            min="1"
            max="60"
            class="form-control"
          >
        </div>
        <template v-if="probeForm.type === 'http'">
          <div class="col-md-4">
            <label
              for="monitoring-edit-probe-expected-status"
              class="form-label"
            >{{ t('monitoring.expectedHttpStatusLabel') }}</label>
            <input
              id="monitoring-edit-probe-expected-status"
              v-model.number="probeForm.expected_status"
              type="number"
              min="100"
              max="599"
              class="form-control"
            >
          </div>
          <div class="col-12">
            <label
              for="monitoring-edit-probe-expected-body-regex"
              class="form-label"
            >{{ t('monitoring.expectedBodyRegexLabel') }}</label>
            <input
              id="monitoring-edit-probe-expected-body-regex"
              v-model="probeForm.expected_body_regex"
              type="text"
              class="form-control"
              :placeholder="t('monitoring.expectedBodyRegexPlaceholder')"
            >
          </div>
          <div class="col-md-6">
            <label class="form-check">
              <input
                v-model="probeForm.follow_redirects"
                type="checkbox"
                class="form-check-input"
              >
              <span class="form-check-label">{{ t('monitoring.followRedirectsLabel') }}</span>
            </label>
          </div>
          <div class="col-md-6">
            <label class="form-check">
              <input
                v-model="probeForm.verify_tls"
                type="checkbox"
                class="form-check-input"
              >
              <span class="form-check-label">{{ t('monitoring.verifyTlsLabel') }}</span>
            </label>
          </div>
        </template>
        <div class="col-12">
          <label class="form-check">
            <input
              v-model="probeForm.enabled"
              type="checkbox"
              class="form-check-input"
            >
            <span class="form-check-label">{{ t('monitoring.probeEnabledLabel') }}</span>
          </label>
        </div>
      </div>
    </div>
    <div class="modal-footer">
      <button
        type="button"
        class="btn link-secondary"
        :disabled="saving"
        @click="emit('close')"
      >
        {{ t('monitoring.cancelButton') }}
      </button>
      <button
        type="submit"
        class="btn btn-primary"
        :disabled="saving"
      >
        {{ saving ? t('monitoring.savingLabel') : t('monitoring.saveButton') }}
      </button>
    </div>
  </form>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ProbeForm } from '../../composables/useUptimeProbes'
import { useProbeTargetHints } from '../../composables/useProbeTargetHints'

defineProps<{
  saving: boolean
  error: string
}>()
const emit = defineEmits<{
  (e: 'save'): void
  (e: 'close'): void
}>()
const probeForm = defineModel<ProbeForm>('probeForm', { required: true })

const { t } = useI18n()
const { probeTargetLabel, probeTargetPlaceholder } = useProbeTargetHints(probeForm)
</script>
