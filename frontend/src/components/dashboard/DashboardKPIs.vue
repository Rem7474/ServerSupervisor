<template>
  <div class="row row-cards mb-4">
    <div class="col-6 col-lg-3">
      <div class="card card-sm h-100">
        <div class="card-body">
          <div class="subheader">Hôtes</div>
          <div class="h1 mb-0">{{ hosts.length }}</div>
          <div class="text-secondary small mt-1">
            <span class="text-green me-2">{{ onlineCount }} en ligne</span>
            <span v-if="offlineCount > 0" class="text-red">{{ offlineCount }} hors ligne</span>
          </div>
        </div>
      </div>
    </div>
    <div class="col-6 col-lg-3">
      <div class="card card-sm h-100">
        <div class="card-body">
          <div class="subheader">Mises à jour</div>
          <div class="h1 mb-0" :class="outdatedVersions > 0 ? 'text-yellow' : 'text-green'">{{ outdatedVersions }}</div>
          <div class="text-secondary small mt-1">
            <span v-if="aptPending > 0" class="me-2">{{ aptPending }} paquet{{ aptPending > 1 ? 's' : '' }} APT</span>
            <span v-if="outdatedDockerImages > 0">{{ outdatedDockerImages }} image{{ outdatedDockerImages > 1 ? 's' : '' }} Docker</span>
            <span v-if="outdatedVersions === 0">Tout est à jour</span>
          </div>
        </div>
      </div>
    </div>
    <!-- Proxmox KPIs (masqués si non configuré) -->
    <template v-if="hasProxmox">
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">Proxmox — Nœuds</div>
            <div class="h1 mb-0" :class="proxmoxSummary?.nodes_down > 0 ? 'text-red' : 'text-green'">
              {{ (proxmoxSummary?.node_count ?? 0) - (proxmoxSummary?.nodes_down ?? 0) }}
              <span class="text-secondary fs-4">/ {{ proxmoxSummary?.node_count ?? 0 }}</span>
            </div>
            <div class="text-secondary small mt-1">{{ proxmoxSummary?.vm_count ?? 0 }} VM · {{ proxmoxSummary?.lxc_count ?? 0 }} LXC</div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">Proxmox — Stockage</div>
            <div class="h1 mb-0" :class="proxmoxStoragePct > 80 ? 'text-red' : proxmoxStoragePct > 60 ? 'text-yellow' : 'text-green'">
              {{ proxmoxStoragePct.toFixed(0) }}%
            </div>
            <div class="text-secondary small mt-1">
              {{ formatBytes(proxmoxSummary?.storage_used ?? 0) }} / {{ formatBytes(proxmoxSummary?.storage_total ?? 0) }}
            </div>
          </div>
        </div>
      </div>
    </template>
    <template v-else>
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">En ligne</div>
            <div class="h1 mb-0 text-green">{{ onlineCount }}</div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">Hors ligne</div>
            <div class="h1 mb-0 text-red">{{ offlineCount }}</div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { defineProps, computed } from 'vue'
import { formatBytes } from '../utils/formatters'

const props = defineProps({
  hosts: { type: Array, default: () => [] },
  proxmoxSummary: { type: Object, default: null },
  aptPending: { type: Number, default: 0 },
  outdatedDockerImages: { type: Number, default: 0 },
  versionComparisons: { type: Array, default: () => [] },
})

const hasProxmox = computed(() => !!props.proxmoxSummary?.node_count)

const onlineCount = computed(() => 
  props.hosts.filter(h => h.status === 'online').length
)

const offlineCount = computed(() => 
  props.hosts.filter(h => h.status !== 'online').length
)

const outdatedVersions = computed(() => 
  props.versionComparisons.filter(v => v.newer_version).length + props.aptPending + props.outdatedDockerImages
)

const proxmoxStoragePct = computed(() => {
  const s = props.proxmoxSummary
  if (!s?.storage_total) return 0
  return (s.storage_used / s.storage_total) * 100
})
</script>
