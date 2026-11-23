<template>
  <div class="modal-body pb-0">
    <div class="btn-group w-100 mb-3">
      <button
        type="button"
        class="btn"
        :class="createIncludeProbe ? 'btn-primary' : 'btn-outline-secondary'"
        @click="toggleIncludeProbe"
      >
        {{ t('monitoring.probeTypeToggle') }}
      </button>
      <button
        type="button"
        class="btn"
        :class="createIncludeCert ? 'btn-primary' : 'btn-outline-secondary'"
        @click="toggleIncludeCert"
      >
        {{ t('monitoring.certTypeToggle') }}
      </button>
    </div>
  </div>
  <form @submit.prevent="submitCreateModal">
    <div class="modal-body pt-0">
      <div
        v-if="error"
        class="alert alert-danger"
      >
        {{ error }}
      </div>
      <div class="mb-3">
        <label
          for="monitoring-create-name"
          class="form-label required"
        >{{ t('monitoring.nameFieldLabel') }}</label>
        <input
          id="monitoring-create-name"
          v-model="sharedName"
          type="text"
          class="form-control"
          placeholder="Ex: API prod"
          required
        >
      </div>

      <template v-if="createIncludeProbe">
        <div class="row g-3">
          <div class="col-md-5">
            <label
              for="monitoring-create-probe-type"
              class="form-label required"
            >{{ t('monitoring.typeFieldLabel') }}</label>
            <select
              id="monitoring-create-probe-type"
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
          <div class="col-md-7">
            <label
              for="monitoring-create-probe-target"
              class="form-label required"
            >{{ probeTargetLabel }}</label>
            <input
              id="monitoring-create-probe-target"
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
              for="monitoring-create-probe-interval"
              class="form-label"
            >{{ t('monitoring.intervalSecLabel') }}</label>
            <input
              id="monitoring-create-probe-interval"
              v-model.number="probeForm.interval_sec"
              type="number"
              min="10"
              class="form-control"
            >
          </div>
          <div class="col-md-4">
            <label
              for="monitoring-create-probe-timeout"
              class="form-label"
            >{{ t('monitoring.timeoutSecLabel') }}</label>
            <input
              id="monitoring-create-probe-timeout"
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
                for="monitoring-create-probe-expected-status"
                class="form-label"
              >{{ t('monitoring.expectedHttpStatusLabel') }}</label>
              <input
                id="monitoring-create-probe-expected-status"
                v-model.number="probeForm.expected_status"
                type="number"
                min="100"
                max="599"
                class="form-control"
              >
            </div>
            <div class="col-12">
              <label
                for="monitoring-create-probe-expected-body-regex"
                class="form-label"
              >{{ t('monitoring.expectedBodyRegexLabel') }}</label>
              <input
                id="monitoring-create-probe-expected-body-regex"
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
              <span class="form-check-label">{{ t('monitoring.probeEnabledLabelExplicit') }}</span>
            </label>
          </div>
        </div>
      </template>

      <template v-if="createIncludeCert">
        <hr
          v-if="createIncludeProbe"
          class="my-3"
        >
        <div class="row g-3">
          <template v-if="!createIncludeProbe">
            <div class="col-md-8">
              <label
                for="monitoring-create-cert-host"
                class="form-label required"
              >{{ t('monitoring.hostFieldLabel') }}</label>
              <input
                id="monitoring-create-cert-host"
                v-model="certForm.host"
                type="text"
                class="form-control"
                placeholder="api.example.com"
                required
              >
            </div>
            <div class="col-md-4">
              <label
                for="monitoring-create-cert-port"
                class="form-label required"
              >{{ t('monitoring.portFieldLabel') }}</label>
              <input
                id="monitoring-create-cert-port"
                v-model.number="certForm.port"
                type="number"
                min="1"
                max="65535"
                class="form-control"
              >
            </div>
          </template>
          <div
            v-else
            class="col-12 text-secondary small"
          >
            {{ t('monitoring.derivedCertHintBefore') }} <code>{{ certForm.host || '—' }}:{{ certForm.port }}</code> {{ t('monitoring.derivedCertHintAfter') }}
          </div>
          <div class="col-12">
            <label
              for="monitoring-create-cert-sni"
              class="form-label"
            >{{ t('monitoring.sniFieldLabel') }}</label>
            <input
              id="monitoring-create-cert-sni"
              v-model="certForm.server_name"
              type="text"
              class="form-control"
              :placeholder="t('monitoring.sniPlaceholder')"
            >
          </div>
          <div class="col-12">
            <label class="form-check">
              <input
                v-model="certForm.enabled"
                type="checkbox"
                class="form-check-input"
              >
              <span class="form-check-label">{{ t('monitoring.certEnabledLabelExplicit') }}</span>
            </label>
          </div>
        </div>
      </template>
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
        {{ (saving) ? t('monitoring.savingLabel') : t('monitoring.saveButton') }}
      </button>
    </div>
  </form>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ProbeForm } from '../../composables/useUptimeProbes'
import type { CertForm } from '../../composables/useSslCertificates'
import { useMonitoringCreateForm } from '../../composables/useMonitoringCreateForm'
import { useProbeTargetHints } from '../../composables/useProbeTargetHints'

const props = defineProps<{
  saving: boolean
  error: string
  isProbeModalOpen: () => boolean
  isCertModalOpen: () => boolean
  saveProbe: () => Promise<void>
  saveCert: () => Promise<void>
}>()
const emit = defineEmits<{
  (e: 'close'): void
}>()
const probeForm = defineModel<ProbeForm>('probeForm', { required: true })
const certForm = defineModel<CertForm>('certForm', { required: true })

const { t } = useI18n()
const { probeTargetLabel, probeTargetPlaceholder } = useProbeTargetHints(probeForm)
const {
  createIncludeProbe,
  createIncludeCert,
  toggleIncludeProbe,
  toggleIncludeCert,
  sharedName,
  submitCreateModal,
} = useMonitoringCreateForm({
  probeForm,
  certForm,
  isProbeModalOpen: props.isProbeModalOpen,
  isCertModalOpen: props.isCertModalOpen,
  saveProbe: props.saveProbe,
  saveCert: props.saveCert,
})
</script>
