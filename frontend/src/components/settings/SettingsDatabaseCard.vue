<template>
  <div class="card">
    <div class="card-header">
      <h3 class="card-title">
        {{ t('settings.databaseTitle') }}
      </h3>
    </div>
    <div class="card-body">
      <div class="mb-3 pb-3 border-bottom">
        <div class="text-secondary small">
          {{ t('settings.connection') }}
        </div>
        <span :class="dbStatus.connected ? 'badge bg-success-lt text-success' : 'badge bg-danger-lt text-danger'">
          {{ dbStatus.connected ? t('settings.connected') : t('settings.disconnected') }}
        </span>
      </div>
      <div class="mb-3 pb-3 border-bottom">
        <div class="text-secondary small">
          {{ t('settings.auditLogs') }}
        </div>
        <div>{{ t('settings.entriesCount', { count: formatNumber(dbStatus.auditLogCount) }) }}</div>
      </div>
      <div class="mb-3 pb-3 border-bottom">
        <div class="text-secondary small">
          {{ t('settings.storedMetrics') }}
        </div>
        <div>{{ t('settings.pointsCount', { count: formatNumber(dbStatus.metricsCount) }) }}</div>
      </div>
      <div>
        <div class="text-secondary small">
          {{ t('settings.registeredHosts') }}
        </div>
        <div>{{ dbStatus.hostsCount }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

interface DBStatus {
  connected: boolean
  auditLogCount: number
  metricsCount: number
  hostsCount: number
}

defineProps<{
  dbStatus: DBStatus
  formatNumber: (n: number) => string
}>()
</script>
