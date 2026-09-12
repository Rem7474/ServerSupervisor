<template>
  <div class="card">
    <div class="card-header">
      <h3 class="card-title">
        {{ t('settings.tabs.maintenance') }}
      </h3>
    </div>
    <div class="card-body">
      <div class="row g-3">
        <div class="col-md-6">
          <h4 class="text-sm mb-2">
            {{ t('settings.metricsCleanupTitle') }}
          </h4>
          <p class="text-secondary small mb-3">
            {{ t('settings.metricsCleanupDesc', { days: settings.metricsRetentionDays }) }}
          </p>
          <button
            type="button"
            class="btn btn-warning btn-sm"
            :disabled="cleaningMetrics"
            @click="confirmCleanMetrics"
          >
            {{ cleaningMetrics ? t('settings.cleanupInProgress') : t('settings.launchCleanup') }}
          </button>
          <div
            v-if="cleanMessage"
            :class="['alert alert-sm mt-2', cleanSuccess ? 'alert-success' : 'alert-danger']"
          >
            {{ cleanMessage }}
          </div>
        </div>

        <div class="col-md-6">
          <h4 class="text-sm mb-2">
            {{ t('settings.auditCleanupTitle') }}
          </h4>
          <p class="text-secondary small mb-3">
            {{ t('settings.auditCleanupDesc', { days: settings.auditRetentionDays }) }}
          </p>
          <button
            type="button"
            class="btn btn-warning btn-sm"
            :disabled="cleaningAuditLogs"
            @click="confirmCleanAudit"
          >
            {{ cleaningAuditLogs ? t('settings.cleanupInProgress') : t('settings.launchCleanup') }}
          </button>
          <div
            v-if="auditCleanMessage"
            :class="['alert alert-sm mt-2', auditCleanSuccess ? 'alert-success' : 'alert-danger']"
          >
            {{ auditCleanMessage }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useConfirmDialog } from '../../composables/useConfirmDialog'

const { t } = useI18n()

interface Settings {
  metricsRetentionDays: number
  auditRetentionDays: number
}

const props = withDefaults(defineProps<{
  settings: Settings
  cleaningMetrics?: boolean
  cleanMessage?: string
  cleanSuccess?: boolean
  cleaningAuditLogs?: boolean
  auditCleanMessage?: string
  auditCleanSuccess?: boolean
}>(), {
  cleaningMetrics: false,
  cleanMessage: '',
  cleanSuccess: false,
  cleaningAuditLogs: false,
  auditCleanMessage: '',
  auditCleanSuccess: false,
})

const emit = defineEmits<{
  (e: 'clean-metrics'): void
  (e: 'clean-audit'): void
}>()

const dialog = useConfirmDialog()

async function confirmCleanMetrics(): Promise<void> {
  const confirmed = await dialog.confirm({
    title: t('settings.confirmCleanupTitle'),
    message: t('settings.metricsCleanupConfirmMsg', { days: props.settings.metricsRetentionDays }),
    variant: 'warning',
    okLabel: t('settings.continueLabel'),
  })
  if (!confirmed) return
  emit('clean-metrics')
}

async function confirmCleanAudit(): Promise<void> {
  const confirmed = await dialog.confirm({
    title: t('settings.confirmCleanupTitle'),
    message: t('settings.auditCleanupConfirmMsg', { days: props.settings.auditRetentionDays }),
    variant: 'warning',
    okLabel: t('settings.continueLabel'),
  })
  if (!confirmed) return
  emit('clean-audit')
}
</script>
