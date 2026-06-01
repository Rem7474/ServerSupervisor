<template>
  <div class="network-config">
    <div class="network-config-row">
      <div class="network-config-item">
        <label class="form-label">Reverse proxy</label>
        <input
          v-model="rootNodeName"
          type="text"
          class="form-control form-control-sm"
          placeholder="Ex: Nginx Proxy Manager"
        >
      </div>
      <div class="network-config-item">
        <label class="form-label">IP du proxy</label>
        <input
          v-model="rootNodeIp"
          type="text"
          class="form-control form-control-sm"
          placeholder="Ex: 192.168.1.10"
        >
      </div>
    </div>
    <div class="network-config-item mt-2">
      <label class="form-label">
        Hôte correspondant
        <span class="text-secondary fw-normal">(optionnel — supprime le nœud fantôme)</span>
      </label>
      <select
        v-model="rootHostId"
        class="form-select form-select-sm"
      >
        <option value="">
          — Nœud abstrait (non lié à un hôte) —
        </option>
        <option
          v-for="h in hosts"
          :key="h.id"
          :value="h.id"
        >
          {{ h.name || h.hostname || h.ip_address || h.id }}
        </option>
      </select>
      <div class="text-secondary small mt-1">
        Quand lié, le nœud proxy dans le graphe devient cet hôte — les deux ne sont plus affichés séparément.
      </div>
      <div
        v-if="rootHostId && proxyHostPorts.length > 0"
        class="mt-2"
      >
        <label class="form-label">
          Port spécifique
          <span class="text-secondary fw-normal">(optionnel)</span>
        </label>
        <select
          v-model="rootPortId"
          class="form-select form-select-sm"
        >
          <option value="">
            — Nœud hôte (pas de port précis) —
          </option>
          <option
            v-for="p in proxyHostPorts"
            :key="p.key"
            :value="p.key"
          >
            {{ p.port }}/{{ p.protocol.toUpperCase() }}
            <template v-if="p.containers?.length">
              — {{ p.containers.join(', ') }}
            </template>
          </option>
        </select>
        <div class="text-secondary small mt-1">
          Le nœud proxy sera ce port précis dans le graphe.
        </div>
      </div>
    </div>

    <div class="network-config-item mt-3">
      <div class="d-flex align-items-center justify-content-between mb-2">
        <div>
          <label class="form-label mb-0">Services manuels via proxy</label>
          <div class="text-secondary small mt-1">
            Services definis manuellement, non detectes automatiquement.
            Pour les ports decouverts, utilisez la section "Ports decouverts" ci-dessous
            et cochez "Proxy".
          </div>
        </div>
        <button
          class="btn btn-outline-light btn-sm ms-2"
          @click="addServiceRow"
        >
          + Ajouter
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
              <th>Host</th>
              <th>Proxy</th>
              <th>Authelia</th>
              <th>Internet</th>
              <th>Port ext.</th>
              <th />
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="service in networkServices"
              :key="service.id"
            >
              <td>
                <input
                  v-model="service.name"
                  class="form-control form-control-sm"
                  placeholder="Ex: Vaultwarden"
                >
              </td>
              <td>
                <input
                  v-model="service.domain"
                  class="form-control form-control-sm"
                  placeholder="vault.example.com"
                >
              </td>
              <td>
                <input
                  v-model="service.path"
                  class="form-control form-control-sm"
                  placeholder="/"
                >
              </td>
              <td>
                <input
                  v-model.number="service.internalPort"
                  type="number"
                  class="form-control form-control-sm"
                  placeholder="3000"
                >
              </td>
              <td>
                <select
                  v-model="service.hostId"
                  class="form-select form-select-sm"
                >
                  <option value="">
                    Choisir...
                  </option>
                  <option
                    v-for="h in hosts"
                    :key="h.id"
                    :value="h.id"
                  >
                    {{ h.name || h.hostname || h.ip_address || h.id }}
                  </option>
                </select>
              </td>
              <td>
                <label class="form-check form-switch"><input
                  v-model="service.linkToProxy"
                  class="form-check-input"
                  type="checkbox"
                ></label>
              </td>
              <td>
                <label class="form-check form-switch"><input
                  v-model="service.linkToAuthelia"
                  class="form-check-input"
                  type="checkbox"
                ></label>
              </td>
              <td>
                <label class="form-check form-switch"><input
                  v-model="service.exposedToInternet"
                  class="form-check-input"
                  type="checkbox"
                ></label>
              </td>
              <td>
                <input
                  v-model.number="service.externalPort"
                  type="number"
                  class="form-control form-control-sm"
                  placeholder="443"
                  :disabled="!service.exposedToInternet"
                  style="width: 70px;"
                >
              </td>
              <td class="text-end">
                <button
                  class="btn btn-sm btn-outline-danger"
                  @click="removeServiceRow(service.id)"
                >
                  Supprimer
                </button>
              </td>
            </tr>
            <tr v-if="networkServices.length === 0">
              <td
                colspan="10"
                class="text-center py-4"
              >
                <div class="text-secondary small">
                  Aucun service configure
                </div>
                <div
                  class="text-muted"
                  style="font-size:.8rem"
                >
                  Ajoutez un service pour le faire apparaitre dans la topologie reseau
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="network-config-item mt-3">
      <label class="form-label">Nœud Authelia (optionnel)</label>
      <div class="network-config-row">
        <div>
          <input
            v-model="autheliaLabel"
            type="text"
            class="form-control form-control-sm"
            placeholder="Ex: Authelia"
          >
          <div class="text-secondary small mt-1">
            Label affiché dans le graphe
          </div>
        </div>
        <div>
          <input
            v-model="autheliaIp"
            type="text"
            class="form-control form-control-sm"
            placeholder="Ex: 192.168.1.11"
          >
          <div class="text-secondary small mt-1">
            IP / domaine Authelia
          </div>
        </div>
      </div>
      <div class="mt-2">
        <label class="form-label">
          Hôte correspondant
          <span class="text-secondary fw-normal">(optionnel)</span>
        </label>
        <select
          v-model="autheliaHostId"
          class="form-select form-select-sm"
        >
          <option value="">
            — Nœud abstrait (non lié à un hôte) —
          </option>
          <option
            v-for="h in hosts"
            :key="h.id"
            :value="h.id"
          >
            {{ h.name || h.hostname || h.ip_address || h.id }}
          </option>
        </select>
        <div
          v-if="autheliaHostId && autheliaHostPorts.length > 0"
          class="mt-2"
        >
          <label class="form-label">
            Port spécifique
            <span class="text-secondary fw-normal">(optionnel)</span>
          </label>
          <select
            v-model="autheliaPortId"
            class="form-select form-select-sm"
          >
            <option value="">
              — Nœud hôte (pas de port précis) —
            </option>
            <option
              v-for="p in autheliaHostPorts"
              :key="p.key"
              :value="p.key"
            >
              {{ p.port }}/{{ p.protocol.toUpperCase() }}
              <template v-if="p.containers?.length">
                — {{ p.containers.join(', ') }}
              </template>
            </option>
          </select>
          <div class="text-secondary small mt-1">
            Le nœud Authelia sera ce port précis dans le graphe.
          </div>
        </div>
      </div>
    </div>

    <div class="network-config-item mt-3">
      <label class="form-label">Nœud Internet / Routeur (optionnel)</label>
      <div class="network-config-row">
        <div>
          <input
            v-model="internetLabel"
            type="text"
            class="form-control form-control-sm"
            placeholder="Ex: Internet"
          >
          <div class="text-secondary small mt-1">
            Label affiché dans le graphe
          </div>
        </div>
        <div>
          <input
            v-model="internetIp"
            type="text"
            class="form-control form-control-sm"
            placeholder="Ex: 1.2.3.4"
          >
          <div class="text-secondary small mt-1">
            IP publique / domaine
          </div>
        </div>
      </div>
    </div>

    <div class="network-config-item mt-4">
      <div class="d-flex align-items-center justify-content-between mb-2">
        <label class="form-label mb-0">Ports decouverts par host</label>
        <div class="text-secondary small">
          Nommer, masquer, lier au proxy
        </div>
      </div>
      <div class="network-discovered">
        <div
          v-for="host in hosts"
          :key="host.id"
          class="network-host-block"
        >
          <div class="network-host-header">
            <div class="fw-semibold">
              {{ host.name || host.hostname || host.ip_address || host.id }}
            </div>
            <div class="text-secondary small">
              {{ host.ip_address || 'IP inconnue' }}
            </div>
            <div class="d-flex gap-2 mt-1">
              <span class="badge bg-blue-lt text-blue text-xs">{{ countEnabled(host.id) }} / {{ (discoveredPortsByHost[host.id] || []).length }} ports affiches</span>
              <span
                v-if="countProxyLinked(host.id) > 0"
                class="badge bg-cyan-lt text-cyan text-xs"
              >{{ countProxyLinked(host.id) }} proxy</span>
              <span
                v-if="countAutheliaLinked(host.id) > 0"
                class="badge bg-purple-lt text-purple text-xs"
              >{{ countAutheliaLinked(host.id) }} Authelia</span>
              <span
                v-if="countInternetExposed(host.id) > 0"
                class="badge bg-orange-lt text-orange text-xs"
              >{{ countInternetExposed(host.id) }} Internet</span>
            </div>
          </div>
          <div class="table-responsive network-config-table">
            <table class="table table-sm table-vcenter">
              <thead>
                <tr>
                  <th>Port</th>
                  <th>Proto</th>
                  <th>Nom</th>
                  <th>Domaine</th>
                  <th>Chemin</th>
                  <th>Afficher</th>
                  <th>Proxy</th>
                  <th>Authelia</th>
                  <th>Internet</th>
                  <th>Port ext.</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="port in discoveredPortsByHost[host.id] || []"
                  :key="port.key"
                  :class="portRowClass(host.id, port.port)"
                >
                  <td class="fw-semibold">
                    {{ port.port }}
                    <span
                      v-if="port.internal"
                      class="badge bg-secondary-lt text-secondary ms-1"
                      title="Port interne Docker uniquement, non exposé sur l'hôte"
                    >interne</span>
                    <div
                      v-if="port.containers?.length"
                      class="text-secondary fw-normal"
                      style="font-size:.75rem;line-height:1.3"
                    >
                      {{ port.containers.join(', ') }}
                    </div>
                  </td>
                  <td class="text-secondary text-uppercase">
                    {{ port.protocol }}
                  </td>
                  <td>
                    <input
                      v-model="getPortSetting(host.id, port.port).name"
                      class="form-control form-control-sm"
                      placeholder="Ex: Vaultwarden"
                    >
                  </td>
                  <td>
                    <input
                      v-model="getPortSetting(host.id, port.port).domain"
                      class="form-control form-control-sm"
                      placeholder="vault.example.com"
                    >
                  </td>
                  <td>
                    <input
                      v-model="getPortSetting(host.id, port.port).path"
                      class="form-control form-control-sm"
                      placeholder="/"
                    >
                  </td>
                  <td>
                    <label class="form-check">
                      <input
                        :id="`port-enabled-${host.id}-${port.port}`"
                        v-model="getPortSetting(host.id, port.port).enabled"
                        class="form-check-input"
                        type="checkbox"
                        @change="onEnabledChange(host.id, port.port, $event)"
                      >
                    </label>
                  </td>
                  <td>
                    <label
                      class="form-check form-switch"
                      :title="getPortProxyTooltip(host.id, port.port)"
                    >
                      <input
                        v-model="getPortSetting(host.id, port.port).linkToProxy"
                        class="form-check-input"
                        type="checkbox"
                        :disabled="!getPortSetting(host.id, port.port).enabled"
                      >
                    </label>
                  </td>
                  <td>
                    <label class="form-check form-switch">
                      <input
                        v-model="getPortSetting(host.id, port.port).linkToAuthelia"
                        class="form-check-input"
                        type="checkbox"
                        :disabled="!getPortSetting(host.id, port.port).enabled"
                      >
                    </label>
                  </td>
                  <td>
                    <label class="form-check form-switch">
                      <input
                        v-model="getPortSetting(host.id, port.port).exposedToInternet"
                        class="form-check-input"
                        type="checkbox"
                        :disabled="!getPortSetting(host.id, port.port).enabled"
                      >
                    </label>
                  </td>
                  <td>
                    <input
                      v-model.number="getPortSetting(host.id, port.port).externalPort"
                      type="number"
                      class="form-control form-control-sm"
                      placeholder="443"
                      :disabled="!getPortSetting(host.id, port.port).exposedToInternet"
                      style="width: 70px;"
                    >
                  </td>
                  <td class="text-end">
                    <button
                      v-if="isPortModified(host.id, port.port)"
                      class="btn btn-sm btn-ghost-secondary"
                      title="Reinitialiser ce port"
                      aria-label="Reinitialiser ce port"
                      @click="resetPortSetting(host.id, port.port)"
                    >
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        class="icon icon-sm"
                        width="16"
                        height="16"
                        viewBox="0 0 24 24"
                        stroke-width="2"
                        stroke="currentColor"
                        fill="none"
                      >
                        <path
                          stroke="none"
                          d="M0 0h24v24H0z"
                          fill="none"
                        />
                        <path d="M20 11a8.1 8.1 0 0 0 -15.5 -2m-.5 -4v4h4" />
                        <path d="M4 13a8.1 8.1 0 0 0 15.5 2m.5 4v-4h-4" />
                      </svg>
                    </button>
                  </td>
                </tr>
                <tr v-if="(discoveredPortsByHost[host.id] || []).length === 0">
                  <td
                    colspan="7"
                    class="text-center py-4"
                  >
                    <div class="text-secondary small">
                      Aucun port detecte
                    </div>
                    <div
                      class="text-muted"
                      style="font-size:.8rem"
                    >
                      L'agent doit être actif et avoir collecté les données réseau
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'

interface NetworkService {
  id: string
  name: string
  domain: string
  path: string
  internalPort: number | null
  externalPort: number | null
  hostId: string
  tags: string
  linkToProxy: boolean
  linkToAuthelia: boolean
  exposedToInternet: boolean
}

interface PortSetting {
  name: string
  domain: string
  path: string
  enabled: boolean
  linkToProxy: boolean
  linkToAuthelia: boolean
  exposedToInternet: boolean
  externalPort: number | null
}

interface HostPortEntry {
  hostId: string
  ports: Record<string, PortSetting>
}

interface DiscoveredPort {
  key: string
  port: number
  protocol: string
  internal: boolean
  containers: string[]
}

interface PortMapping {
  host_port?: number
  container_port?: number
  protocol?: string
}

interface Container {
  host_id: string
  name?: string
  port_mappings?: PortMapping[]
}

interface Host {
  id: string
  name?: string
  hostname?: string
  ip_address?: string
}

const rootNodeName    = defineModel<string>('rootNodeName',    { default: 'Infrastructure' })
const rootNodeIp      = defineModel<string>('rootNodeIp',      { default: '' })
const autheliaLabel   = defineModel<string>('autheliaLabel',   { default: 'Authelia' })
const autheliaIp      = defineModel<string>('autheliaIp',      { default: '' })
const internetLabel   = defineModel<string>('internetLabel',   { default: 'Internet' })
const internetIp      = defineModel<string>('internetIp',      { default: '' })
const networkServices = defineModel<NetworkService[]>('networkServices', { default: () => [] })
const hostPortConfig  = defineModel<HostPortEntry[]>('hostPortConfig',  { default: () => [] })
const rootHostId      = defineModel<string>('rootHostId',      { default: '' })
const autheliaHostId  = defineModel<string>('autheliaHostId',  { default: '' })
const rootPortId      = defineModel<string>('rootPortId',      { default: '' })
const autheliaPortId  = defineModel<string>('autheliaPortId',  { default: '' })

const props = withDefaults(defineProps<{
  hosts?: Host[]
  containers?: Container[]
}>(), {
  hosts: () => [],
  containers: () => [],
})

const discoveredPortsByHost = computed<Record<string, DiscoveredPort[]>>(() => {
  const map: Record<string, DiscoveredPort[]> = {}
  for (const container of props.containers) {
    const mappings = container.port_mappings || []
    for (const mapping of mappings) {
      const hostId = container.host_id
      if (!hostId) continue

      const hostPort = mapping.host_port || 0
      const containerPort = mapping.container_port || 0
      const portNumber = hostPort || containerPort
      if (!portNumber) continue

      const protocol = (mapping.protocol || 'tcp').toLowerCase()
      if (!map[hostId]) map[hostId] = []
      const key = `${portNumber}-${protocol}`
      const existing = map[hostId].find((entry) => entry.key === key)
      if (existing) {
        if (container.name && !existing.containers.includes(container.name)) existing.containers.push(container.name)
        continue
      }

      map[hostId].push({ key, port: portNumber, protocol, internal: hostPort === 0, containers: container.name ? [container.name] : [] })
    }
  }

  for (const host of props.hosts) {
    if (!map[host.id]) map[host.id] = []
  }

  return map
})

// Ports available for the proxy-host and authelia-host dropdowns
const proxyHostPorts = computed(() => {
  if (!rootHostId.value) return []
  return (discoveredPortsByHost.value[rootHostId.value] || []).filter(p => !p.internal)
})

const autheliaHostPorts = computed(() => {
  if (!autheliaHostId.value) return []
  return (discoveredPortsByHost.value[autheliaHostId.value] || []).filter(p => !p.internal)
})

// Reset port selection when the linked host changes
watch(rootHostId, () => { rootPortId.value = '' })
watch(autheliaHostId, () => { autheliaPortId.value = '' })

watch(
  [() => props.hosts, discoveredPortsByHost],
  () => {
    ensureHostPortConfig()
  },
  { deep: true, immediate: true }
)

function onEnabledChange(hostId: string, portNumber: number, event: Event): void {
  const setting = getPortSetting(hostId, portNumber)
  if (!(event.target as HTMLInputElement).checked) {
    setting.linkToProxy = false
    setting.linkToAuthelia = false
    setting.exposedToInternet = false
  }
}

function getPortProxyTooltip(hostId: string, portNumber: number): string {
  const setting = getPortSetting(hostId, portNumber)
  return !setting.enabled ? "Activez d'abord l'affichage du port" : ''
}

function portRowClass(hostId: string, portNumber: number): Record<string, boolean> {
  const s = getPortSetting(hostId, portNumber)
  return {
    'opacity-50': !s.enabled,
    'port-row-proxy': s.enabled && s.linkToProxy && !s.linkToAuthelia,
    'port-row-authelia': s.enabled && s.linkToAuthelia,
  }
}

function countEnabled(hostId: string): number {
  const entry = hostPortConfig.value.find((e) => e.hostId === hostId)
  if (!entry) return (discoveredPortsByHost.value[hostId] || []).length
  const ports = discoveredPortsByHost.value[hostId] || []
  return ports.filter((p) => {
    const s = entry.ports?.[String(p.port)]
    return s === undefined || s.enabled !== false
  }).length
}

function countProxyLinked(hostId: string): number {
  const entry = hostPortConfig.value.find((e) => e.hostId === hostId)
  if (!entry) return 0
  return Object.values(entry.ports || {}).filter((s) => s?.linkToProxy && s?.enabled).length
}

function countAutheliaLinked(hostId: string): number {
  const entry = hostPortConfig.value.find((e) => e.hostId === hostId)
  if (!entry) return 0
  return Object.values(entry.ports || {}).filter((s) => s?.linkToAuthelia && s?.enabled).length
}

function countInternetExposed(hostId: string): number {
  const entry = hostPortConfig.value.find((e) => e.hostId === hostId)
  if (!entry) return 0
  return Object.values(entry.ports || {}).filter((s) => s?.exposedToInternet && s?.enabled).length
}

function isPortModified(hostId: string, portNumber: number): boolean {
  const s = getPortSetting(hostId, portNumber)
  return s.name !== '' || !s.enabled || s.linkToProxy || s.linkToAuthelia || s.exposedToInternet || s.domain !== '' || (s.path !== '/' && s.path !== '')
}

function resetPortSetting(hostId: string, portNumber: number): void {
  const entry = getHostPortEntry(hostId)
  entry.ports[String(portNumber)] = createDefaultPortSetting()
}

function getPortSetting(hostId: string, portNumber: number): PortSetting {
  const entry = getHostPortEntry(hostId)
  const key = String(portNumber)
  if (!entry.ports[key]) {
    entry.ports[key] = createDefaultPortSetting()
  }
  return entry.ports[key]
}

function ensureHostPortConfig(): void {
  const known = new Set(hostPortConfig.value.map((item) => item.hostId))
  for (const host of props.hosts) {
    if (known.has(host.id)) continue
    hostPortConfig.value.push({ hostId: host.id, ports: {} })
  }
  for (const [hostId, ports] of Object.entries(discoveredPortsByHost.value)) {
    const entry = getHostPortEntry(hostId)
    for (const port of ports) {
      const portKey = String(port.port)
      if (!entry.ports[portKey]) {
        entry.ports[portKey] = createDefaultPortSetting()
      }
    }
  }
}

function getHostPortEntry(hostId: string): HostPortEntry {
  let entry = hostPortConfig.value.find((item) => item.hostId === hostId)
  if (!entry) {
    entry = { hostId, ports: {} }
    hostPortConfig.value.push(entry)
  }
  if (!entry.ports) entry.ports = {}
  return entry
}

function createDefaultPortSetting(): PortSetting {
  return { name: '', domain: '', path: '/', enabled: true, linkToProxy: false, linkToAuthelia: false, exposedToInternet: false, externalPort: null }
}

function addServiceRow(): void {
  networkServices.value.push({
    id: `svc-${Date.now()}-${Math.floor(Math.random() * 1000)}`,
    name: '',
    domain: '',
    path: '/',
    internalPort: null,
    externalPort: null,
    hostId: '',
    tags: '',
    linkToProxy: false,
    linkToAuthelia: false,
    exposedToInternet: false,
  })
}

function removeServiceRow(serviceId: string): void {
  networkServices.value = networkServices.value.filter((service) => service.id !== serviceId)
}
</script>

<style scoped>
.port-row-proxy {
  background-color: rgba(var(--tblr-cyan-rgb, 23, 162, 184), 0.07) !important;
}

.port-row-authelia {
  background-color: rgba(var(--tblr-purple-rgb, 132, 90, 223), 0.08) !important;
}

.network-config {
  padding: 16px 18px 24px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
  background: rgba(15, 23, 42, 0.45);
  overflow-y: auto;
  max-height: calc(100vh - 260px);
}

.network-config-row {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
  margin-bottom: 12px;
  align-items: start;
}

.network-config-item .form-label {
  font-size: 12px;
  color: #cbd5f5;
}

.network-config-item input:not([type="checkbox"]):not([type="radio"]) {
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

.network-host-header {
  margin-bottom: 0.75rem;
}

@media (max-width: 900px) {
  .network-config-row {
    grid-template-columns: 1fr;
  }
}
</style>

