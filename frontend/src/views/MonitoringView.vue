<template>
  <div>
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <h2 class="page-title">
            {{ t('monitoring.pageTitle') }}
          </h2>
          <div class="text-muted">
            {{ t('monitoring.pageSubtitle') }}
          </div>
        </div>
        <div
          v-if="auth.role === 'admin'"
          class="d-flex gap-2"
        >
          <!-- One entry point into the shared create modal (see
               MonitoringOverviewPanel.vue), which already lets the user pick
               uptime, SSL, or both inside — two separate buttons here used to
               just pre-select a type the modal's own toggle already covers. -->
          <button
            type="button"
            class="btn btn-primary btn-sm"
            @click="panelRef?.openCreateProbe()"
          >
            <IconPlus
              :size="14"
              class="icon me-1"
            />
            {{ t('monitoring.newTrackerButton') }}
          </button>
        </div>
      </div>
    </div>

    <MonitoringOverviewPanel ref="panelRef" />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconPlus } from '@tabler/icons-vue'
import { useAuthStore } from '../stores/auth'
import MonitoringOverviewPanel from '../components/monitoring/MonitoringOverviewPanel.vue'

const auth = useAuthStore()
const { t } = useI18n()

const panelRef = ref<InstanceType<typeof MonitoringOverviewPanel> | null>(null)
</script>
