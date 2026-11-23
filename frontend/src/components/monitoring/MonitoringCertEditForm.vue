<template>
  <form @submit.prevent="emit('save')">
    <div class="modal-body pt-0">
      <div
        v-if="error"
        class="alert alert-danger"
      >
        {{ error }}
      </div>
      <div class="mb-3">
        <label
          for="monitoring-edit-cert-name"
          class="form-label required"
        >{{ t('monitoring.nameFieldLabel') }}</label>
        <input
          id="monitoring-edit-cert-name"
          v-model="certForm.name"
          type="text"
          class="form-control"
          placeholder="Ex: api.example.com"
          required
        >
      </div>
      <div class="row g-3">
        <div class="col-md-8">
          <label
            for="monitoring-edit-cert-host"
            class="form-label required"
          >{{ t('monitoring.hostFieldLabel') }}</label>
          <input
            id="monitoring-edit-cert-host"
            v-model="certForm.host"
            type="text"
            class="form-control"
            placeholder="api.example.com"
            required
          >
        </div>
        <div class="col-md-4">
          <label
            for="monitoring-edit-cert-port"
            class="form-label required"
          >{{ t('monitoring.portFieldLabel') }}</label>
          <input
            id="monitoring-edit-cert-port"
            v-model.number="certForm.port"
            type="number"
            min="1"
            max="65535"
            class="form-control"
          >
        </div>
        <div class="col-12">
          <label
            for="monitoring-edit-cert-sni"
            class="form-label"
          >{{ t('monitoring.sniFieldLabel') }}</label>
          <input
            id="monitoring-edit-cert-sni"
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
            <span class="form-check-label">{{ t('monitoring.certEnabledLabel') }}</span>
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
import type { CertForm } from '../../composables/useSslCertificates'

defineProps<{
  saving: boolean
  error: string
}>()
const emit = defineEmits<{
  (e: 'save'): void
  (e: 'close'): void
}>()
const certForm = defineModel<CertForm>('certForm', { required: true })

const { t } = useI18n()
</script>
