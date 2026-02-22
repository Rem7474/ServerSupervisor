<template>
  <div>
    <div class="page-header mb-4">
      <h2 class="page-title">Network</h2>
      <div class="text-secondary">Ports exposes et trafic par hote</div>
    </div>

    <WsStatusBar :status="wsStatus" :error="wsError" :retry-count="retryCount" @reconnect="reconnect" />

    <div class="row row-cards mb-4">
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="subheader">Hotes</div>
            <div class="h1 mb-0">{{ hosts.length }}</div>
          </div>
        </div>
      </div>
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="subheader">Conteneurs</div>
            <div class="h1 mb-0">{{ containers.length }}</div>
          </div>
        </div>
      </div>
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="subheader">Ports visibles</div>
            <div class="h1 mb-0">{{ totalPorts }}</div>
          </div>
        </div>
      </div>
      <div class="col-sm-6 col-lg-3">
        <div class="card card-sm">
          <div class="card-body">
            <div class="subheader">Trafic total</div>
            <div class="h1 mb-0">{{ formatBytes(totalRx + totalTx) }}</div>
            <div class="text-secondary small">Rx {{ formatBytes(totalRx) }} / Tx {{ formatBytes(totalTx) }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- View Mode Toggle -->
    <div class="card mb-4">
      <div class="card-body">
        <div class="btn-group" role="group">
          <input type="radio" class="btn-check" id="viewCards" value="cards" v-model="viewMode" />
          <label class="btn btn-outline-primary" for="viewCards">
            <svg width="20" height="20" fill="currentColor" viewBox="0 0 16 16">
              <path d="M1 1a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H2a1 1 0 0 1-1-1V1zm10 0a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1h-2a1 1 0 0 1-1-1V1zM1 11a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H2a1 1 0 0 1-1-1v-4zm10 0a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1h-2a1 1 0 0 1-1-1v-4z"/>
            </svg>
            Cards
          </label>
          
          <input type="radio" class="btn-check" id="viewGraph" value="graph" v-model="viewMode" />
          <label class="btn btn-outline-primary" for="viewGraph">
            <svg width="20" height="20" fill="currentColor" viewBox="0 0 16 16">
              <path d="M12 1a1 1 0 0 1 1 1v12a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V2a1 1 0 0 1 1-1h8zM4 0a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2V2a2 2 0 0 0-2-2H4z"/>
              <path d="M5 6a1 1 0 1 1-2 0 1 1 0 0 1 2 0zm3 0a1 1 0 1 1-2 0 1 1 0 0 1 2 0zM5 9a1 1 0 1 1-2 0 1 1 0 0 1 2 0zm3 0a1 1 0 1 1-2 0 1 1 0 0 1 2 0z"/>
            </svg>
            Graph
          </label>
        </div>
      </div>
    </div>

    <!-- Graph View -->
    <div v-if="viewMode === 'graph'" class="card mb-4 network-topology-card">
      <div class="card-header d-flex align-items-center justify-content-between">
        <div>
          <h3 class="card-title mb-1">Network Topology</h3>
          <div class="text-secondary small">Glisser pour reordonner, scroll pour zoomer</div>
        </div>
        <div class="text-secondary small">
          {{ hosts.length }} hotes • {{ totalPorts }} ports publies
        </div>
      </div>
      <div class="card-body network-topology-body">
        <NetworkGraph :data="hosts" @host-click="handleHostClick" />
      </div>
    </div>

    <!-- Cards View (Original) -->
    <template v-if="viewMode === 'cards'">
      <div class="card mb-4">
        <div class="card-body">
          <div class="row g-3">
            <div class="col-md-6 col-lg-3">
              <input v-model="search" type="text" class="form-control" placeholder="Rechercher un port, conteneur, image..." />
            </div>
            <div class="col-md-6 col-lg-3">
              <select v-model="protocolFilter" class="form-select">
                <option value="">Tous les protocoles</option>
                <option value="tcp">TCP</option>
                <option value="udp">UDP</option>
              </select>
            </div>
            <div class="col-md-6 col-lg-3">
              <select v-model="hostFilter" class="form-select">
                <option value="">Tous les hotes</option>
                <option v-for="h in hosts" :key="h.id" :value="h.id">
                  {{ h.name || h.hostname || h.id }}
                </option>
              </select>
            </div>
            <div class="col-md-6 col-lg-3">
              <label class="form-check form-switch">
                <input v-model="onlyPublished" class="form-check-input" type="checkbox" />
                <span class="form-check-label">Ports publies seulement</span>
              </label>
            </div>
          </div>
        </div>
      </div>

      <div class="card mb-4">
        <div class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>Hote</th>
                <th>Conteneur</th>
                <th>Image</th>
                <th>Port hote</th>
                <th>Port conteneur</th>
                <th>Proto</th>
                <th>Bind</th>
                <th>Etat</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in portRows" :key="row.key">
                <td>
                  <router-link :to="`/hosts/${row.host_id}`" class="text-decoration-none">
                    {{ row.host_name || row.host_id }}
                  </router-link>
                </td>
                <td class="fw-semibold">{{ row.container_name }}</td>
                <td>
                  <div>{{ row.image }}</div>
                  <div class="text-secondary small"><code>{{ row.image_tag || '-' }}</code></div>
                </td>
                <td class="fw-semibold">{{ row.host_port || '-' }}</td>
                <td class="text-secondary">{{ row.container_port || '-' }}</td>
                <td class="text-secondary text-uppercase">{{ row.protocol || '-' }}</td>
                <td class="text-secondary small font-monospace">{{ row.host_ip || '-' }}</td>
                <td>
                  <span :class="row.state === 'running' ? 'badge bg-green-lt text-green' : 'badge bg-secondary-lt text-secondary'">
                    {{ row.state || 'unknown' }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="portRows.length === 0" class="text-center text-secondary py-4">
          Aucun port visible
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <h3 class="card-title">Trafic par hote</h3>
        </div>
        <div class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>Hote</th>
                <th>IP</th>
                <th>Rx</th>
                <th>Tx</th>
                <th>Statut</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="h in hosts" :key="h.id">
                <td>
                  <router-link :to="`/hosts/${h.id}`" class="fw-semibold text-decoration-none">
                    {{ h.name || h.hostname || h.id }}
                  </router-link>
                </td>
                <td class="text-secondary">{{ h.ip_address }}</td>
                <td>{{ formatBytes(h.network_rx_bytes || 0) }}</td>
                <td>{{ formatBytes(h.network_tx_bytes || 0) }}</td>
                <td>
                  <span :class="h.status === 'online' ? 'badge bg-green-lt text-green' : h.status === 'warning' ? 'badge bg-yellow-lt text-yellow' : 'badge bg-red-lt text-red'">
                    {{ h.status || 'unknown' }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="hosts.length === 0" class="text-center text-secondary py-4">
          Aucun hote trouve
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useWebSocket } from '../composables/useWebSocket'
import WsStatusBar from '../components/WsStatusBar.vue'
import NetworkGraph from '../components/NetworkGraph.vue'
import apiClient from '../api'

const router = useRouter()
const hosts = ref([])
const containers = ref([])
const search = ref('')
const protocolFilter = ref('')
const hostFilter = ref('')
const onlyPublished = ref(true)
const viewMode = ref(localStorage.getItem('networkViewMode') || 'cards')
const auth = useAuthStore()

// Save view mode to localStorage when it changes
watch(viewMode, (newMode) => {
  localStorage.setItem('networkViewMode', newMode)
})

const portRows = computed(() => {
  const rows = []
  for (const container of containers.value) {
    const mappings = container.port_mappings || []
    for (const mapping of mappings) {
      const hostPort = Number(mapping.host_port || 0)
      const isPublished = hostPort > 0
      if (onlyPublished.value && !isPublished) continue

      rows.push({
        key: `${container.id}-${mapping.raw}`,
        host_id: container.host_id,
        host_name: container.hostname,
        container_name: container.name,
        image: container.image,
        image_tag: container.image_tag,
        state: container.state,
        host_port: hostPort,
        container_port: mapping.container_port,
        protocol: mapping.protocol,
        host_ip: mapping.host_ip,
        raw: mapping.raw,
      })
    }
  }

  const query = search.value.trim().toLowerCase()
  return rows.filter((row) => {
    const matchHost = !hostFilter.value || row.host_id === hostFilter.value
    const matchProto = !protocolFilter.value || row.protocol === protocolFilter.value
    const matchSearch =
      !query ||
      row.container_name?.toLowerCase().includes(query) ||
      row.image?.toLowerCase().includes(query) ||
      row.image_tag?.toLowerCase().includes(query) ||
      row.host_name?.toLowerCase().includes(query) ||
      String(row.host_port || '').includes(query) ||
      String(row.container_port || '').includes(query) ||
      row.protocol?.toLowerCase().includes(query) ||
      row.host_ip?.toLowerCase().includes(query)

    return matchHost && matchProto && matchSearch
  })
})

const totalPorts = computed(() => portRows.value.length)
const totalRx = computed(() => hosts.value.reduce((sum, h) => sum + (h.network_rx_bytes || 0), 0))
const totalTx = computed(() => hosts.value.reduce((sum, h) => sum + (h.network_tx_bytes || 0), 0))

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

function handleHostClick(hostId) {
  router.push(`/hosts/${hostId}`)
}

async function fetchSnapshot() {
  try {
    const res = await apiClient.getNetworkSnapshot()
    hosts.value = res.data?.hosts || []
    containers.value = res.data?.containers || []
  } catch (e) {
    // ignore
  }
}

const { wsStatus, wsError, retryCount, reconnect } = useWebSocket('/api/v1/ws/network', (payload) => {
  if (payload.type !== 'network') return
  hosts.value = payload.hosts || []
  containers.value = payload.containers || []
})

onMounted(() => {
  fetchSnapshot()
})

onUnmounted(() => {
})
</script>

<style scoped>
.network-topology-card {
  overflow: hidden;
}

.network-topology-body {
  height: 68vh;
  min-height: 520px;
  padding: 0;
}

@media (max-width: 991px) {
  .network-topology-body {
    height: 60vh;
    min-height: 420px;
  }
}
</style>
