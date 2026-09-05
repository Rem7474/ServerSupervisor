<template>
  <template v-if="host">
    <div
      ref="modalRef"
      class="modal modal-blur fade show d-block"
      tabindex="-1"
      @click.self="$emit('close')"
    >
      <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <div>
              <h5 class="modal-title">
                {{ t('apt.scheduleModalTitle') }}
              </h5>
              <div class="text-muted small mt-1">
                {{ hostLabel }}
              </div>
            </div>
            <button
              type="button"
              class="btn-close"
              @click="$emit('close')"
            />
          </div>
          <div class="modal-body">
            <div class="mb-3">
              <label class="form-label">{{ t('apt.taskName') }}</label>
              <input
                v-model="form.name"
                type="text"
                class="form-control"
                :placeholder="t('apt.taskNamePlaceholder')"
              >
            </div>
            <div class="mb-3">
              <label class="form-label">{{ t('apt.command') }}</label>
              <select
                v-model="form.action"
                class="form-select"
              >
                <option value="update">
                  apt update
                </option>
                <option value="upgrade">
                  apt upgrade
                </option>
                <option value="dist-upgrade">
                  apt dist-upgrade
                </option>
              </select>
            </div>
            <div class="mb-3">
              <label class="form-check form-switch">
                <input
                  v-model="form.manualOnly"
                  type="checkbox"
                  class="form-check-input"
                >
                <span class="form-check-label">{{ t('apt.manualExecutionOnly') }}</span>
              </label>
            </div>
            <div
              v-if="!form.manualOnly"
              class="mb-3"
            >
              <CronBuilder v-model="form.cron_expression" />
            </div>
            <div
              v-if="!form.manualOnly"
              class="form-check form-switch mb-2"
            >
              <input
                id="schedEnabled"
                v-model="form.enabled"
                type="checkbox"
                class="form-check-input"
              >
              <label
                class="form-check-label"
                for="schedEnabled"
              >{{ t('apt.enabled') }}</label>
            </div>
            <div
              v-if="form.error"
              class="alert alert-danger py-2"
            >
              {{ form.error }}
            </div>
          </div>
          <div class="modal-footer">
            <button
              type="button"
              class="btn btn-secondary"
              @click="$emit('close')"
            >
              {{ t('common.cancel') }}
            </button>
            <button
              type="button"
              class="btn btn-primary"
              :disabled="form.saving"
              @click="saveSchedule"
            >
              <span
                v-if="form.saving"
                class="spinner-border spinner-border-sm me-1"
              />
              {{ t('apt.createTask') }}
            </button>
          </div>
        </div>
      </div>
    </div>
    <div class="modal-backdrop fade show" />
  </template>
</template>

<script setup lang="ts">
import { reactive, computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import apiClient from '../../api'
import CronBuilder from '../CronBuilder.vue'
import { MANUAL_SENTINEL } from '../../utils/cron'
import { getApiErrorMessage } from '../../api/client'
import { useModalChrome } from '../../composables/useModalChrome'

const { t } = useI18n()

const props = defineProps<{
  host: { id: string; name?: string; hostname?: string } | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'created'): void
}>()

const modalRef = ref<HTMLElement | null>(null)
useModalChrome(modalRef, () => !!props.host, { onClose: () => emit('close') })

const form = reactive({
  name: '',
  action: 'update',
  cron_expression: '0 3 * * 0',
  manualOnly: false,
  enabled: true,
  saving: false,
  error: '',
})

const hostLabel = computed(() => {
  const host = props.host
  if (!host) return ''
  return host.name && host.hostname && host.name !== host.hostname
    ? `${host.name} (${host.hostname})`
    : (host.name || host.hostname)
})

// Reset the form each time the modal is opened for a host.
watch(() => props.host, (host) => {
  if (!host) return
  form.name = ''
  form.action = 'update'
  form.cron_expression = '0 3 * * 0'
  form.manualOnly = false
  form.enabled = true
  form.saving = false
  form.error = ''
})

async function saveSchedule(): Promise<void> {
  if (!props.host) return
  form.error = ''
  form.saving = true
  const cronExpr = form.manualOnly ? MANUAL_SENTINEL : form.cron_expression
  try {
    await apiClient.createScheduledTask(props.host.id, {
      name: form.name || `apt ${form.action}`,
      module: 'apt',
      action: form.action,
      target: '',
      payload: '{}',
      cron_expression: cronExpr,
      enabled: form.manualOnly ? false : form.enabled,
    })
    emit('created')
    emit('close')
  } catch (e: unknown) {
    form.error = getApiErrorMessage(e, t('apt.createError'))
  } finally {
    form.saving = false
  }
}
</script>
