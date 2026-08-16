<template>
  <div>
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <h2 class="page-title">
            Monitoring
          </h2>
          <div class="text-muted">
            Sondes HTTP/TCP synthétiques et suivi des certificats SSL/TLS.
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
            Nouveau suivi
          </button>
        </div>
      </div>
    </div>

    <MonitoringOverviewPanel ref="panelRef" />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { IconPlus } from '@tabler/icons-vue'
import { useAuthStore } from '../stores/auth'
import MonitoringOverviewPanel from '../components/monitoring/MonitoringOverviewPanel.vue'

const auth = useAuthStore()

const panelRef = ref<InstanceType<typeof MonitoringOverviewPanel> | null>(null)
</script>
