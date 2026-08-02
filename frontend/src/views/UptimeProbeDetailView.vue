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
        <span>{{ probe?.name || 'Sonde' }}</span>
      </div>
      <h2 class="page-title">
        {{ probe?.name || '...' }}
      </h2>
      <div
        v-if="probe"
        class="text-secondary"
      >
        <code>{{ probe.target }}</code>
      </div>
    </div>

    <UptimeDetailSection
      :probe-id="probeId"
      @loaded="onLoaded"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import UptimeDetailSection from '../components/monitoring/UptimeDetailSection.vue'
import type { UptimeProbe } from '../types/generated'

const route = useRoute()
const router = useRouter()
const probeId = route.params.id as string
const probe = ref<UptimeProbe | null>(null)

// A probe created by an NPM proxy host's monitoring toggle has a combined
// uptime+SSL page at /monitoring/host/:id (see MonitoringHostDetailView) —
// send any deep link that lands here (notifications, command palette,
// bookmarks) there instead of showing uptime alone.
function onLoaded(p: UptimeProbe | null): void {
  probe.value = p
  if (p?.npm_proxy_host_id) {
    router.replace(`/monitoring/host/${p.npm_proxy_host_id}`)
  }
}
</script>
