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
          <button
            type="button"
            class="btn btn-outline-primary"
            @click="panelRef?.openCreateCert()"
          >
            + Certificat SSL
          </button>
          <button
            type="button"
            class="btn btn-primary"
            @click="panelRef?.openCreateProbe()"
          >
            + Nouvelle sonde
          </button>
        </div>
      </div>
    </div>

    <MonitoringOverviewPanel ref="panelRef" />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useAuthStore } from '../stores/auth'
import MonitoringOverviewPanel from '../components/monitoring/MonitoringOverviewPanel.vue'

const auth = useAuthStore()

const panelRef = ref<InstanceType<typeof MonitoringOverviewPanel> | null>(null)
</script>
