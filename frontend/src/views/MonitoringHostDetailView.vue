<template>
  <div>
    <div class="page-header mb-3">
      <div class="page-pretitle">
        <router-link
          to="/npm"
          class="text-decoration-none"
        >
          NPM
        </router-link>
        <span class="text-muted mx-1">/</span>
        <span>{{ host?.domain_names?.[0] || 'Monitoring' }}</span>
      </div>
      <h2 class="page-title">
        {{ host?.domain_names?.[0] || '...' }}
      </h2>
      <div
        v-if="host"
        class="text-secondary"
      >
        <code>{{ host.forward_host }}:{{ host.forward_port }}</code>
        <router-link
          to="/npm"
          class="badge bg-azure-lt text-azure text-decoration-none ms-2"
        >
          Voir dans NPM
        </router-link>
      </div>
    </div>

    <div
      v-if="loading"
      class="row row-cards"
    >
      <div class="col-12 col-md-3">
        <LoadingSkeleton
          variant="kpi"
          :lines="4"
        />
      </div>
    </div>

    <div
      v-else-if="error"
      class="alert alert-danger"
    >
      {{ error }}
    </div>

    <template v-else-if="host">
      <div
        v-if="!host.uptime_probe_id && !host.ssl_certificate_id"
        class="alert alert-info"
      >
        Aucun suivi actif pour ce proxy host. Activez le suivi uptime et/ou SSL depuis
        <router-link to="/npm">
          NPM
        </router-link>.
      </div>

      <template v-if="host.uptime_probe_id">
        <h3 class="mb-2">
          Disponibilité
        </h3>
        <UptimeDetailSection
          :probe-id="host.uptime_probe_id"
          class="mb-4"
        />
      </template>

      <template v-if="host.ssl_certificate_id">
        <h3 class="mb-2">
          Certificat SSL
        </h3>
        <SslDetailSection :cert-id="host.ssl_certificate_id" />
      </template>
    </template>
  </div>
</template>

<script setup lang="ts">
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import UptimeDetailSection from '../components/monitoring/UptimeDetailSection.vue'
import SslDetailSection from '../components/monitoring/SslDetailSection.vue'
import { useMonitoringHostDetail } from '../composables/useMonitoringHostDetail'

const { host, loading, error } = useMonitoringHostDetail()
</script>
