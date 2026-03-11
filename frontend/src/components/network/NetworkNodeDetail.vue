<template>
  <aside class="network-node-detail card">
    <div class="card-header">
      <h3 class="card-title">Detail du noeud</h3>
    </div>
    <div class="card-body">
      <div v-if="!selectedNode" class="text-secondary small">
        Selectionnez un noeud dans le graphe pour afficher ses details.
      </div>
      <template v-else>
        <div class="mb-3">
          <div class="text-secondary small text-uppercase">Type</div>
          <div class="fw-semibold">{{ nodeTypeLabel }}</div>
        </div>
        <div class="mb-3">
          <div class="text-secondary small text-uppercase">Nom</div>
          <div class="fw-semibold">{{ selectedNode.label || '-' }}</div>
          <div v-if="selectedNode.sublabel" class="text-secondary small mt-1">{{ selectedNode.sublabel }}</div>
        </div>

        <div v-if="selectedNode.type === 'host'" class="vstack gap-3">
          <div>
            <div class="text-secondary small text-uppercase">Statut</div>
            <span :class="statusBadgeClass">{{ hostData?.status || selectedNode.status || 'unknown' }}</span>
          </div>
          <div>
            <div class="text-secondary small text-uppercase">Trafic</div>
            <div class="small">↓ {{ formatBytes(hostData?.network_rx_bytes || 0) }} / ↑ {{ formatBytes(hostData?.network_tx_bytes || 0) }}</div>
          </div>
          <div>
            <div class="text-secondary small text-uppercase">Conteneurs</div>
            <div class="small">{{ hostContainers.length }}</div>
          </div>
          <router-link v-if="selectedNode.hostId" :to="`/hosts/${selectedNode.hostId}`" class="btn btn-outline-primary btn-sm">
            Ouvrir l'hote
          </router-link>
        </div>

        <div v-else-if="selectedNode.type === 'service'" class="vstack gap-3">
          <div>
            <div class="text-secondary small text-uppercase">Port interne</div>
            <div>{{ selectedNode.internalPort || '-' }}</div>
          </div>
          <div v-if="selectedNode.externalPort">
            <div class="text-secondary small text-uppercase">Port externe</div>
            <div>{{ selectedNode.externalPort }}</div>
          </div>
          <div>
            <div class="text-secondary small text-uppercase">Tags</div>
            <div>{{ selectedNode.tags || '-' }}</div>
          </div>
        </div>

        <div v-else-if="selectedNode.type === 'port'" class="vstack gap-3">
          <div>
            <div class="text-secondary small text-uppercase">Port</div>
            <div>{{ selectedNode.portNumber || '-' }}/{{ (selectedNode.protocol || '').toUpperCase() }}</div>
          </div>
          <div>
            <div class="text-secondary small text-uppercase">Conteneurs</div>
            <div class="small">{{ selectedNode.containers?.join(', ') || '-' }}</div>
          </div>
          <div v-if="selectedNode.externalPort">
            <div class="text-secondary small text-uppercase">Exposition Internet</div>
            <div>Port {{ selectedNode.externalPort }}</div>
          </div>
        </div>
      </template>
    </div>
  </aside>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  selectedNode: {
    type: Object,
    default: null,
  },
  hosts: {
    type: Array,
    default: () => [],
  },
  containers: {
    type: Array,
    default: () => [],
  },
})

const hostData = computed(() => props.hosts.find((host) => host.id === props.selectedNode?.hostId) || null)
const hostContainers = computed(() => props.containers.filter((container) => container.host_id === props.selectedNode?.hostId))

const nodeTypeLabel = computed(() => {
  const map = {
    root: 'Reverse proxy',
    host: 'Hote',
    service: 'Service',
    port: 'Port',
    authelia: 'Authelia',
    internet: 'Internet',
  }
  return map[props.selectedNode?.type] || 'Noeud'
})

const statusBadgeClass = computed(() => {
  const status = hostData.value?.status || props.selectedNode?.status
  if (status === 'online') return 'badge bg-green-lt text-green'
  if (status === 'warning') return 'badge bg-yellow-lt text-yellow'
  if (status === 'offline') return 'badge bg-red-lt text-red'
  return 'badge bg-secondary-lt text-secondary'
})

function formatBytes(bytes) {
  if (!bytes && bytes !== 0) return '-'
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let value = bytes / 1024
  let idx = 0
  while (value >= 1024 && idx < units.length - 1) {
    value /= 1024
    idx += 1
  }
  return `${value.toFixed(1)} ${units[idx]}`
}
</script>

<style scoped>
.network-node-detail {
  width: 320px;
  min-width: 280px;
  align-self: stretch;
}

@media (max-width: 991px) {
  .network-node-detail {
    width: 100%;
    min-width: 0;
  }
}
</style>
