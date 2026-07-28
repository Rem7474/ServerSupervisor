<template>
  <div>
    <div class="page-header mb-3">
      <div class="page-pretitle">
        <router-link
          to="/monitoring"
          class="text-decoration-none"
        >
          Monitoring
        </router-link>
        <span class="text-muted mx-1">/</span>
        <span>{{ cert?.name || 'Certificat' }}</span>
      </div>
      <h2 class="page-title">
        {{ cert?.name || '...' }}
      </h2>
      <div
        v-if="cert"
        class="text-secondary"
      >
        <code>{{ cert.host }}:{{ cert.port }}</code>
        <span
          v-if="cert.server_name"
          class="ms-2 text-muted small"
        >(SNI: {{ cert.server_name }})</span>
      </div>
    </div>

    <SslDetailSection
      :cert-id="certId"
      @loaded="onLoaded"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SslDetailSection from '../components/monitoring/SslDetailSection.vue'
import type { SSLCertificate } from '../types/ssl'

const route = useRoute()
const router = useRouter()
const certId = route.params.id as string
const cert = ref<SSLCertificate | null>(null)

// A cert created by an NPM proxy host's monitoring toggle has a combined
// uptime+SSL page at /monitoring/host/:id (see MonitoringHostDetailView) —
// send any deep link that lands here (notifications, command palette,
// bookmarks) there instead of showing the certificate alone.
function onLoaded(c: SSLCertificate | null): void {
  cert.value = c
  if (c?.npm_proxy_host_id) {
    router.replace(`/monitoring/host/${c.npm_proxy_host_id}`)
  }
}
</script>
