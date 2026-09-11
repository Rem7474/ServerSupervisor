<template>
  <div class="card">
    <div class="card-header">
      <h3 class="card-title">
        {{ t('settings.systemTitle') }}
      </h3>
    </div>
    <div class="card-body">
      <div class="mb-3 pb-3 border-bottom">
        <div class="text-secondary small">
          {{ t('settings.baseUrl') }}
        </div>
        <div class="font-monospace">
          {{ settings.baseUrl || t('settings.notConfigured') }}
        </div>
      </div>
      <div class="mb-3 pb-3 border-bottom">
        <div class="text-secondary small">
          {{ t('settings.databaseTitle') }}
        </div>
        <div class="font-monospace">
          {{ settings.dbHost }}:{{ settings.dbPort }}
        </div>
      </div>
      <div class="mb-3 pb-3 border-bottom">
        <div class="text-secondary small">
          {{ t('settings.httpsTlsMode') }}
        </div>
        <span :class="settings.tlsEnabled ? 'badge bg-success-lt text-success' : 'badge bg-warning-lt text-warning'">
          {{ settings.tlsEnabled ? t('settings.active') : t('settings.inactive') }}
        </span>
      </div>
      <div class="mb-3">
        <div class="text-secondary small">
          {{ t('settings.recommendedAgentVersion') }}
        </div>
        <div class="font-monospace">
          {{ settings.latestAgentVersion || '-' }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

interface SystemSettings {
  baseUrl?: string
  dbHost?: string
  dbPort?: number | string
  tlsEnabled?: boolean
  latestAgentVersion?: string
}

defineProps<{
  settings: SystemSettings
}>()
</script>
