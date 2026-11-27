<template>
  <div>
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <h1 class="page-title">
            {{ t('monitoring.pageTitle') }}
          </h1>
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
            class="btn btn-outline-secondary btn-sm"
            @click="statusPageOpen = true"
          >
            <IconWorld
              :size="14"
              class="icon me-1"
            />
            {{ t('monitoring.statusPage.openButton') }}
          </button>
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

    <MonitoringSlaPanel class="mt-3" />

    <StatusPageSettingsModal
      v-if="statusPageOpen"
      @close="statusPageOpen = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconPlus, IconWorld } from '@tabler/icons-vue'
import { useAuthStore } from '../stores/auth'
import MonitoringOverviewPanel from '../components/monitoring/MonitoringOverviewPanel.vue'
import MonitoringSlaPanel from '../components/monitoring/MonitoringSlaPanel.vue'
import StatusPageSettingsModal from '../components/monitoring/StatusPageSettingsModal.vue'

const auth = useAuthStore()
const { t } = useI18n()

const panelRef = ref<InstanceType<typeof MonitoringOverviewPanel> | null>(null)
const statusPageOpen = ref(false)
</script>
