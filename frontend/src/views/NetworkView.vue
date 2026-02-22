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
      <div class="network-subnav">
        <button class="btn" :class="networkTab === 'topology' ? 'btn-primary' : 'btn-outline-primary'" @click="networkTab = 'topology'">
          Topology
        </button>
        <button class="btn" :class="networkTab === 'config' ? 'btn-primary' : 'btn-outline-primary'" @click="networkTab = 'config'">
          Configuration
        </button>
      </div>
      <div class="card-body network-topology-body">
        <div v-if="networkTab === 'config'" class="network-config">
          <div class="network-config-row">
            <div class="network-config-item">
              <label class="form-label">Reverse proxy</label>
              <input v-model="rootNodeName" type="text" class="form-control form-control-sm" placeholder="Ex: Nginx Proxy Manager" />
            </div>
            <div class="network-config-item">
              <label class="form-label">IP du proxy</label>
              <input v-model="rootNodeIp" type="text" class="form-control form-control-sm" placeholder="Ex: 192.168.1.10" />
            </div>
            <div class="network-config-item">
              <label class="form-label">Exclure ports (global)</label>
              <input v-model="excludedPortsText" type="text" class="form-control form-control-sm" placeholder="Ex: 22, 2375, 9000" />
              <div class="text-secondary small">Liste separee par virgules</div>
            </div>
          </div>
          <div class="network-config-item">
            <label class="form-label">Nom des services (port=nom)</label>
            <textarea v-model="servicePortMapText" rows="2" class="form-control form-control-sm" placeholder="80=Nginx Proxy Manager&#10;3000=Vaultwarden"></textarea>
          </div>
          <div class="network-config-item mt-3">
            <label class="form-label">Liens proxy</label>
            <label class="form-check form-switch">
              <input v-model="showProxyLinks" class="form-check-input" type="checkbox" />
              <span class="form-check-label">Afficher les liens explicites Proxy → Service</span>
            </label>
          </div>
          <div class="network-config-item mt-3">
            <div class="d-flex align-items-center justify-content-between mb-2">
              <label class="form-label mb-0">Services exposes via proxy</label>
              <button class="btn btn-outline-light btn-sm" @click="addServiceRow">
                Ajouter un service
              </button>
            </div>
            <div class="table-responsive network-config-table">
              <table class="table table-sm table-vcenter">
                <thead>
                  <tr>
                    <th>Nom</th>
                    <th>Domaine</th>
                    <th>Chemin</th>
                    <th>Port interne</th>
                    <th>Port externe</th>
                    <th>Host</th>
                    <th>Tags</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="service in networkServices" :key="service.id">
                    <td><input v-model="service.name" class="form-control form-control-sm" placeholder="Ex: Vaultwarden" /></td>
                    <td><input v-model="service.domain" class="form-control form-control-sm" placeholder="vault.example.com" /></td>
                    <td><input v-model="service.path" class="form-control form-control-sm" placeholder="/" /></td>
                    <td><input v-model.number="service.internalPort" type="number" class="form-control form-control-sm" placeholder="3000" /></td>
                    <td><input v-model.number="service.externalPort" type="number" class="form-control form-control-sm" placeholder="443" /></td>
                    <td>
                      <select v-model="service.hostId" class="form-select form-select-sm">
                        <option value="">Choisir...</option>
                        <option v-for="h in hosts" :key="h.id" :value="h.id">
                          {{ h.name || h.hostname || h.ip_address || h.id }}
                        </option>
                      </select>
                    </td>
                    <td><input v-model="service.tags" class="form-control form-control-sm" placeholder="auth, admin" /></td>
                    <td class="text-end">
                      <button class="btn btn-sm btn-outline-danger" @click="removeServiceRow(service.id)">Supprimer</button>
                    </td>
                  </tr>
                  <tr v-if="networkServices.length === 0">
                    <td colspan="8" class="text-secondary text-center py-3">Aucun service configure</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
          <div class="network-config-item mt-4">
            <div class="d-flex align-items-center justify-content-between mb-2">
              <label class="form-label mb-0">Ports decouverts par host</label>
              <div class="text-secondary small">Nommer, masquer, lier au proxy</div>
            </div>
            <div class="network-discovered">
              <div v-for="host in hosts" :key="host.id" class="network-host-block">
                <div class="network-host-header">
                  <div class="fw-semibold">{{ host.name || host.hostname || host.ip_address || host.id }}</div>
                  <div class="text-secondary small">{{ host.ip_address || 'IP inconnue' }}</div>
                </div>
                <div class="table-responsive network-config-table">
                  <table class="table table-sm table-vcenter">
                    <thead>
                      <tr>
                        <th>Port interne</th>
                        <th>Proto</th>
                        <th>Nom</th>
                        <th>Domaine</th>
                        <th>Chemin</th>
                        <th>Afficher</th>
                        <th>Lier proxy</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="port in discoveredPortsByHost[host.id] || []" :key="port.key">
                        <td class="fw-semibold">{{ port.port }}</td>
                        <td class="text-secondary text-uppercase">{{ port.protocol }}</td>
                        <td>
                          <input v-model="getPortSetting(host.id, port.port).name" class="form-control form-control-sm" placeholder="Ex: Vaultwarden" />
                        </td>
                        <td>
                          <input v-model="getPortSetting(host.id, port.port).domain" class="form-control form-control-sm" placeholder="vault.example.com" />
                        </td>
                        <td>
                          <input v-model="getPortSetting(host.id, port.port).path" class="form-control form-control-sm" placeholder="/" />
                        </td>
                        <td>
                          <label class="form-check">
                            <input v-model="getPortSetting(host.id, port.port).enabled" class="form-check-input" type="checkbox" />
                            <span class="form-check-label">Afficher</span>
                          </label>
                        </td>
                        <td>
                          <label class="form-check form-switch">
                            <input v-model="getPortSetting(host.id, port.port).linkToProxy" class="form-check-input" type="checkbox" />
                            <span class="form-check-label">Proxy</span>
                          </label>
                        </td>
                      </tr>
                      <tr v-if="(discoveredPortsByHost[host.id] || []).length === 0">
                        <td colspan="7" class="text-secondary text-center py-3">Aucun port detecte</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div v-else class="network-graph-surface">
          <NetworkGraph
            :data="graphHosts"
            :root-label="rootNodeName"
            :root-ip="rootNodeIp"
            :service-map="servicePortMap"
            :excluded-ports="excludedPorts"
            :services="combinedServices"
            :host-port-overrides="hostPortOverrides"
            :show-proxy-links="showProxyLinks"
            @host-click="handleHostClick"
          />
        </div>
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
const networkTab = ref('topology')
const rootNodeName = ref(localStorage.getItem('networkRootName') || 'pve')
const rootNodeIp = ref(localStorage.getItem('networkRootIp') || '')
const showProxyLinks = ref(localStorage.getItem('networkShowProxyLinks') !== 'false')
const servicePortMapText = ref(localStorage.getItem('networkServiceMap') || '')
const excludedPortsText = ref(localStorage.getItem('networkExcludedPorts') || '')
const storedServices = localStorage.getItem('networkServices')
const networkServices = ref(parseStoredServices(storedServices))
const storedHostPorts = localStorage.getItem('networkHostPorts')
const hostPortConfig = ref(parseStoredHostPorts(storedHostPorts))
const auth = useAuthStore()

// Save view mode to localStorage when it changes
watch(viewMode, (newMode) => {
  localStorage.setItem('networkViewMode', newMode)
})

watch(rootNodeName, (value) => {
  localStorage.setItem('networkRootName', value)
})

watch(rootNodeIp, (value) => {
  localStorage.setItem('networkRootIp', value)
})

watch(showProxyLinks, (value) => {
  localStorage.setItem('networkShowProxyLinks', value ? 'true' : 'false')
})

watch(servicePortMapText, (value) => {
  localStorage.setItem('networkServiceMap', value)
})

watch(excludedPortsText, (value) => {
  localStorage.setItem('networkExcludedPorts', value)
})

watch(networkServices, (value) => {
  localStorage.setItem('networkServices', JSON.stringify(value))
}, { deep: true })

watch(hostPortConfig, (value) => {
  localStorage.setItem('networkHostPorts', JSON.stringify(value))
}, { deep: true })

const servicePortMap = computed(() => {
  const map = {}
  const lines = servicePortMapText.value.split(/\r?\n|,/).map(line => line.trim()).filter(Boolean)
  for (const line of lines) {
    const [portRaw, ...nameParts] = line.split(/[=:]/)
    const port = Number(portRaw?.trim())
    const name = nameParts.join(':').trim()
    if (!port || !name) continue
    map[port] = name
  }
  return map
})

const excludedPorts = computed(() => {
  const values = excludedPortsText.value.split(/\s*,\s*/).map(entry => Number(entry.trim())).filter(Boolean)
  return Array.from(new Set(values))
})

const discoveredPortsByHost = computed(() => {
  const map = {}
  for (const container of containers.value) {
    const mappings = container.port_mappings || []
    for (const mapping of mappings) {
      const hostId = container.host_id
      if (!hostId) continue

      const portNumber = mapping.container_port || mapping.host_port || 0
      if (!portNumber) continue

      const protocol = (mapping.protocol || 'tcp').toLowerCase()
      if (!map[hostId]) map[hostId] = []
      const key = `${portNumber}-${protocol}`
      if (map[hostId].some(entry => entry.key === key)) continue

      map[hostId].push({ key, port: portNumber, protocol })
    }
  }

  for (const host of hosts.value) {
    if (!map[host.id]) map[host.id] = []
  }

  return map
})

const hostPortOverrides = computed(() => {
  const overrides = {}
  for (const entry of hostPortConfig.value) {
    if (!entry.hostId) continue
    const excludedPortsList = []
    const portMap = {}
    for (const [port, settings] of Object.entries(entry.ports || {})) {
      const portNumber = Number(port)
      if (!settings?.enabled) excludedPortsList.push(portNumber)
      if (settings?.name) portMap[portNumber] = settings.name
    }
    overrides[entry.hostId] = { excludedPorts: excludedPortsList, portMap }
  }
  return overrides
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

const combinedServices = computed(() => {
  const linkedServices = []
  for (const entry of hostPortConfig.value) {
    if (!entry.hostId) continue
    for (const [port, settings] of Object.entries(entry.ports || {})) {
      if (!settings?.linkToProxy) continue
      const portNumber = Number(port)
      if (!portNumber) continue
      const name = settings.name || `Port ${portNumber}`
      const domain = settings.domain || ''
      const path = settings.path || '/'
      linkedServices.push({
        id: `linked-${entry.hostId}-${portNumber}`,
        name,
        domain,
        path,
        internalPort: portNumber,
        externalPort: null,
        hostId: entry.hostId,
        tags: 'proxy'
      })
    }
  }
  return [...networkServices.value, ...linkedServices]
})


const graphHosts = computed(() => {
  const portsByHost = new Map()
  for (const container of containers.value) {
    const mappings = container.port_mappings || []
    for (const mapping of mappings) {
      const hostId = container.host_id
      if (!hostId) continue

      const portNumber = mapping.host_port || mapping.container_port || 0
      if (!portNumber) continue

      const protocol = (mapping.protocol || 'tcp').toLowerCase()
      const key = `${portNumber}-${protocol}`

      if (!portsByHost.has(hostId)) {
        portsByHost.set(hostId, new Map())
      }

      const hostPorts = portsByHost.get(hostId)
      if (!hostPorts.has(key)) {
        hostPorts.set(key, {
          port: portNumber,
          protocol,
          containers: []
        })
      }

      const entry = hostPorts.get(key)
      entry.containers.push(container.name)
    }
  }

  return hosts.value.map((host) => {
    const hostPorts = portsByHost.get(host.id)
    return {
      ...host,
      ports: hostPorts ? Array.from(hostPorts.values()) : []
    }
  })
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

function hostLabel(hostId) {
  const host = hosts.value.find((item) => item.id === hostId)
  return host?.name || host?.hostname || host?.ip_address || hostId
}

function parseStoredHostPorts(raw) {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.map((entry) => {
      if (entry?.ports) return entry
      const ports = {}
      const excluded = parsePortList(entry?.excludedPortsText || '')
      const map = parsePortMap(entry?.portMapText || '')
      for (const port of Object.keys(map)) {
        const portNumber = Number(port)
        ports[portNumber] = {
          name: map[portNumber],
          domain: '',
          path: '/',
          enabled: !excluded.includes(portNumber),
          linkToProxy: false
        }
      }
      return { hostId: entry?.hostId, ports }
    })
  } catch (err) {
    return []
  }
}

function parsePortList(text) {
  if (!text) return []
  return Array.from(new Set(text.split(/\s*,\s*/).map(entry => Number(entry.trim())).filter(Boolean)))
}

function parsePortMap(text) {
  const map = {}
  if (!text) return map
  const lines = text.split(/\r?\n|,/).map(line => line.trim()).filter(Boolean)
  for (const line of lines) {
    const [portRaw, ...nameParts] = line.split(/[=:]/)
    const port = Number(portRaw?.trim())
    const name = nameParts.join(':').trim()
    if (!port || !name) continue
    map[port] = name
  }
  return map
}

function getPortSetting(hostId, portNumber) {
  const entry = getHostPortEntry(hostId)
  const key = String(portNumber)
  if (!entry.ports[key]) {
    entry.ports[key] = { name: '', domain: '', path: '/', enabled: true, linkToProxy: false }
  }
  return entry.ports[key]
}

function ensureHostPortConfig() {
  const known = new Set(hostPortConfig.value.map((item) => item.hostId))
  for (const host of hosts.value) {
    if (known.has(host.id)) continue
    hostPortConfig.value.push({ hostId: host.id, ports: {} })
  }
  for (const [hostId, ports] of Object.entries(discoveredPortsByHost.value)) {
    const entry = getHostPortEntry(hostId)
    for (const port of ports) {
      getPortSetting(hostId, port.port)
    }
  }
}

function getHostPortEntry(hostId) {
  let entry = hostPortConfig.value.find((item) => item.hostId === hostId)
  if (!entry) {
    entry = { hostId, ports: {} }
    hostPortConfig.value.push(entry)
  }
  if (!entry.ports) entry.ports = {}
  return entry
}

function parseStoredServices(raw) {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed : []
  } catch (err) {
    return []
  }
}

function addServiceRow() {
  networkServices.value.push({
    id: `svc-${Date.now()}-${Math.floor(Math.random() * 1000)}`,
    name: '',
    domain: '',
    path: '/',
    internalPort: null,
    externalPort: null,
    hostId: '',
    tags: ''
  })
}

function removeServiceRow(serviceId) {
  networkServices.value = networkServices.value.filter((service) => service.id !== serviceId)
}

function handleHostClick(hostId) {
  router.push(`/hosts/${hostId}`)
}

async function fetchSnapshot() {
  try {
    const res = await apiClient.getNetworkSnapshot()
    hosts.value = res.data?.hosts || []
    containers.value = res.data?.containers || []
    ensureHostPortConfig()
  } catch (e) {
    // ignore
  }
}

const { wsStatus, wsError, retryCount, reconnect } = useWebSocket('/api/v1/ws/network', (payload) => {
  if (payload.type !== 'network') return
  hosts.value = payload.hosts || []
  containers.value = payload.containers || []
  ensureHostPortConfig()
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

.network-subnav {
  display: flex;
  gap: 8px;
  padding: 14px 18px 0;
  background: rgba(15, 23, 42, 0.45);
}

.network-topology-body {
  height: 68vh;
  min-height: 520px;
  padding: 0;
  display: flex;
  flex-direction: column;
}

.network-config {
  padding: 16px 18px 12px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
  background: rgba(15, 23, 42, 0.45);
}

.network-config-row {
  display: grid;
  grid-template-columns: minmax(180px, 260px) minmax(220px, 1fr);
  gap: 16px;
  margin-bottom: 12px;
}

.network-config-item .form-label {
  font-size: 12px;
  color: #cbd5f5;
}

.network-config-item textarea,
.network-config-item input {
  background: rgba(15, 23, 42, 0.7);
  border: 1px solid rgba(148, 163, 184, 0.4);
  color: #e2e8f0;
}

.network-config-table {
  border: 1px solid rgba(148, 163, 184, 0.2);
  border-radius: 12px;
  overflow: hidden;
  background: rgba(15, 23, 42, 0.55);
}

.network-config-table table {
  margin: 0;
}

.network-config-table thead th {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.4px;
  color: #94a3b8;
  background: rgba(15, 23, 42, 0.7);
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
}

.network-config-table tbody td {
  border-top: 1px solid rgba(148, 163, 184, 0.1);
  vertical-align: middle;
}

.network-config-table .form-control,
.network-config-table .form-select {
  background: rgba(15, 23, 42, 0.6);
  border: 1px solid rgba(148, 163, 184, 0.3);
  color: #e2e8f0;
}

.network-config-item textarea::placeholder,
.network-config-item input::placeholder {
  color: rgba(226, 232, 240, 0.55);
}

.network-graph-surface {
  flex: 1;
  min-height: 0;
  padding: 16px 18px 18px;
}

@media (max-width: 991px) {
  .network-topology-body {
    height: 60vh;
    min-height: 420px;
  }

  .network-config-row {
    grid-template-columns: 1fr;
  }
}
</style>
