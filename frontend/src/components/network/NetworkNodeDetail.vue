<template>
  <aside class="network-node-detail">
    <!-- Mobile header -->
    <div class="detail-mobile-header d-flex d-lg-none align-items-center gap-2 px-3 py-2 border-bottom">
      <svg width="14" height="14" fill="currentColor" viewBox="0 0 16 16">
        <path d="M8 15A7 7 0 1 1 8 1a7 7 0 0 1 0 14zm0 1A8 8 0 1 0 8 0a8 8 0 0 0 0 16z"/>
        <path d="m8.93 6.588-2.29.287-.082.38.45.083c.294.07.352.176.288.469l-.738 3.468c-.194.897.105 1.319.808 1.319.545 0 1.178-.252 1.465-.598l.088-.416c-.2.176-.492.246-.686.246-.275 0-.375-.193-.304-.533L8.93 6.588zM9 4.5a1 1 0 1 1-2 0 1 1 0 0 1 2 0z"/>
      </svg>
      <span class="text-secondary small fw-semibold">Détails du nœud sélectionné</span>
    </div>

    <div class="detail-scroll">
      <div v-if="!selectedNode" class="detail-empty">
        <svg width="36" height="36" fill="none" stroke="currentColor" stroke-width="1.2" viewBox="0 0 24 24" class="mb-3">
          <circle cx="12" cy="12" r="3"/><path d="M12 3v2m0 14v2M3 12h2m14 0h2m-3.22-6.78-1.42 1.42M6.64 17.36l-1.42 1.42m0-12.78 1.42 1.42m10.72 10.72 1.42 1.42"/>
        </svg>
        <div class="text-secondary small">Cliquez sur un nœud du graphe<br>pour afficher ses détails.</div>
      </div>

      <template v-else>

        <!-- ─── Identification ─────────────────────────────────── -->
        <section class="detail-section">
          <div class="detail-section-title">Identification</div>
          <div class="detail-kv">
            <span class="detail-key">Nom</span>
            <span class="detail-val fw-semibold">{{ selectedNode.label || '-' }}</span>
          </div>
          <div v-if="selectedNode.sublabel && selectedNode.type !== 'service'" class="detail-kv">
            <span class="detail-key">Adresse</span>
            <span class="detail-val text-secondary">{{ selectedNode.sublabel }}</span>
          </div>
          <div class="detail-kv">
            <span class="detail-key">Type</span>
            <span :class="typeTagClass">{{ nodeTypeLabel }}</span>
          </div>
        </section>

        <!-- ─── Rôle réseau (host) ─────────────────────────────── -->
        <section v-if="selectedNode.type === 'host'" class="detail-section">
          <div class="detail-section-title">Rôle réseau</div>
          <div class="detail-kv">
            <span class="detail-key">Statut</span>
            <span :class="statusBadgeClass">{{ hostData?.status || 'unknown' }}</span>
          </div>
          <div class="detail-kv">
            <span class="detail-key">Trafic cumul.</span>
            <span class="detail-val small">↓ {{ formatBytes(hostData?.network_rx_bytes || 0) }} / ↑ {{ formatBytes(hostData?.network_tx_bytes || 0) }}</span>
          </div>
          <div class="detail-kv">
            <span class="detail-key">Conteneurs</span>
            <span class="detail-val">{{ hostContainers.length }} <span class="text-secondary">({{ hostContainers.filter(c => c.state === 'running').length }} actifs)</span></span>
          </div>
        </section>

        <!-- ─── Ports & services exposés (host) ───────────────── -->
        <section v-if="selectedNode.type === 'host'" class="detail-section">
          <div class="detail-section-title">Ports & services exposés</div>

          <!-- Logical services -->
          <div v-for="svc in hostServices" :key="svc.id" class="port-entry port-entry-service">
            <div class="port-entry-head">
              <span class="fw-semibold">{{ svc.name }}</span>
              <div class="port-badges">
                <span class="badge bg-cyan-lt text-cyan">service</span>
                <span v-if="svc.linkToProxy" class="badge bg-blue-lt text-blue" title="Via reverse proxy">proxy</span>
                <span v-if="svc.linkToAuthelia" class="badge bg-purple-lt text-purple" title="Protégé par Authelia">auth</span>
                <span v-if="svc.exposedToInternet" class="badge bg-orange-lt text-orange" title="Exposé à Internet">inet</span>
              </div>
            </div>
            <div class="port-entry-sub text-secondary">
              Port {{ svc.internalPort }}
              <template v-if="svc.domain">
                •
                <a :href="`https://${svc.domain}${svc.path || '/'}`" target="_blank" rel="noopener" class="text-blue">{{ svc.domain }}{{ svc.path || '/' }}</a>
              </template>
            </div>
            <div v-if="svc.domain" class="port-entry-actions">
              <a :href="`https://${svc.domain}${svc.path || '/'}`" target="_blank" rel="noopener" class="btn btn-xs btn-outline-primary">Ouvrir</a>
              <button class="btn btn-xs btn-outline-secondary" @click="copyUrl(`https://${svc.domain}${svc.path || '/'}`)">Copier l'URL</button>
            </div>
          </div>

          <!-- Raw ports (excluding service-covered ones) -->
          <div
            v-for="p in allHostPorts"
            :key="p.key"
            class="port-entry"
            :class="{ 'port-entry-disabled': !p.enabled }"
          >
            <div class="port-entry-head">
              <span class="fw-medium" :class="{ 'text-secondary': !p.enabled }">
                {{ p.name || p.port + '/' + p.protocol.toUpperCase() }}
              </span>
              <div class="port-badges">
                <span v-if="!p.enabled" class="badge bg-secondary-lt text-secondary">désactivé</span>
                <template v-else>
                  <span v-if="p.isProxyLinked" class="badge bg-blue-lt text-blue">proxy</span>
                  <span v-if="p.isAutheliaLinked" class="badge bg-purple-lt text-purple">auth</span>
                  <span v-if="p.isInternetExposed" class="badge bg-orange-lt text-orange">inet</span>
                </template>
              </div>
            </div>
            <div class="port-entry-sub text-secondary">
              {{ p.port }}/{{ p.protocol.toUpperCase() }}
              <template v-if="p.containers?.length"> • {{ p.containers.join(', ') }}</template>
            </div>
          </div>

          <div v-if="hostServices.length === 0 && allHostPorts.length === 0" class="text-secondary small py-2">
            Aucun port exposé détecté.
          </div>
        </section>

        <!-- ─── Service node details ───────────────────────────── -->
        <section v-if="selectedNode.type === 'service'" class="detail-section">
          <div class="detail-section-title">Ports & services</div>
          <div class="detail-kv">
            <span class="detail-key">Port interne</span>
            <span class="detail-val fw-semibold">{{ selectedNode.internalPort || '-' }}</span>
          </div>
          <div v-if="selectedNode.externalPort" class="detail-kv">
            <span class="detail-key">Port externe</span>
            <span class="detail-val">{{ selectedNode.externalPort }}</span>
          </div>
          <div v-if="serviceUrl" class="detail-kv">
            <span class="detail-key">URL</span>
            <a :href="serviceUrl" target="_blank" rel="noopener" class="detail-val small text-blue" style="word-break:break-all">{{ serviceUrl }}</a>
          </div>
        </section>

        <!-- ─── Port node details ──────────────────────────────── -->
        <section v-if="selectedNode.type === 'port'" class="detail-section">
          <div class="detail-section-title">Port</div>
          <div class="detail-kv">
            <span class="detail-key">Numéro</span>
            <span class="detail-val fw-semibold">{{ selectedNode.portNumber }}/{{ (selectedNode.protocol || '').toUpperCase() }}</span>
          </div>
          <div v-if="selectedNode.containers?.length" class="detail-kv">
            <span class="detail-key">Conteneurs</span>
            <span class="detail-val small">{{ selectedNode.containers.join(', ') }}</span>
          </div>
        </section>

        <!-- ─── Intégration ───────────────────────────────────── -->
        <section
          v-if="['service', 'port'].includes(selectedNode.type)"
          class="detail-section"
        >
          <div class="detail-section-title">Intégration</div>
          <div class="detail-kv">
            <span class="detail-key">Reverse proxy</span>
            <span v-if="selectedNode.isProxyLinked" class="badge bg-blue-lt text-blue">Oui</span>
            <span v-else class="text-secondary small">Non</span>
          </div>
          <div class="detail-kv">
            <span class="detail-key">Authelia</span>
            <span v-if="selectedNode.isAutheliaLinked" class="badge bg-purple-lt text-purple">Oui</span>
            <span v-else class="text-secondary small">Non</span>
          </div>
          <div class="detail-kv">
            <span class="detail-key">Internet</span>
            <span v-if="selectedNode.isInternetExposed" class="badge bg-orange-lt text-orange">
              Exposé{{ selectedNode.externalPort ? ' (port ' + selectedNode.externalPort + ')' : '' }}
            </span>
            <span v-else class="text-secondary small">Non exposé</span>
          </div>
        </section>

        <!-- ─── Actions ───────────────────────────────────────── -->
        <section class="detail-section detail-actions">
          <router-link
            v-if="selectedNode.type === 'host' && selectedNode.hostId"
            :to="`/hosts/${selectedNode.hostId}`"
            class="btn btn-sm btn-outline-primary w-100"
          >
            <svg width="14" height="14" fill="currentColor" viewBox="0 0 16 16" class="me-1">
              <path d="M6.5 14.5v-3.505c0-.245.25-.495.5-.495h2c.25 0 .5.25.5.5v3.5a.5.5 0 0 0 .5.5h4a.5.5 0 0 0 .5-.5v-7a.5.5 0 0 0-.146-.354L13 5.793V2.5a.5.5 0 0 0-.5-.5h-1a.5.5 0 0 0-.5.5v1.293L8.354 1.146a.5.5 0 0 0-.708 0l-6 6A.5.5 0 0 0 1.5 7.5v7a.5.5 0 0 0 .5.5h4a.5.5 0 0 0 .5-.5z"/>
            </svg>
            Ouvrir l'hôte
          </router-link>

          <router-link
            v-if="selectedNode.type !== 'host' && selectedNode.hostId"
            :to="`/hosts/${selectedNode.hostId}`"
            class="btn btn-sm btn-outline-secondary w-100"
          >
            Voir l'hôte associé
          </router-link>

          <template v-if="selectedNode.type === 'service' && serviceUrl">
            <a :href="serviceUrl" target="_blank" rel="noopener" class="btn btn-sm btn-outline-primary w-100">
              Ouvrir dans le navigateur
            </a>
            <button class="btn btn-sm btn-outline-secondary w-100" @click="copyUrl(serviceUrl)">
              Copier l'URL
            </button>
          </template>
        </section>

      </template>
    </div>
  </aside>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  selectedNode: { type: Object, default: null },
  hosts: { type: Array, default: () => [] },
  containers: { type: Array, default: () => [] },
  hostPortOverrides: { type: Object, default: () => ({}) },
  combinedServices: { type: Array, default: () => [] },
  discoveredPortsByHost: { type: Object, default: () => ({}) },
})

const hostData = computed(() => props.hosts.find(h => h.id === props.selectedNode?.hostId) || null)
const hostContainers = computed(() => props.containers.filter(c => c.host_id === props.selectedNode?.hostId))

const hostServices = computed(() => {
  if (props.selectedNode?.type !== 'host') return []
  const hostId = props.selectedNode.hostId
  return props.combinedServices.filter(s => s.hostId === hostId)
})

const allHostPorts = computed(() => {
  if (props.selectedNode?.type !== 'host') return []
  const hostId = props.selectedNode.hostId
  if (!hostId) return []

  const discovered = props.discoveredPortsByHost?.[hostId] || []
  const override = props.hostPortOverrides?.[hostId] || {}
  const excludedPorts = new Set((override.excludedPorts || []).map(Number))
  const portMap = override.portMap || {}
  const proxyPorts = override.proxyPorts || new Set()
  const autheliaPortNumbers = override.autheliaPortNumbers || new Set()
  const internetExposedPorts = override.internetExposedPorts || {}

  const servicePorts = new Set(
    props.combinedServices
      .filter(s => s.hostId === hostId)
      .map(s => Number(s.internalPort))
      .filter(Boolean)
  )

  return discovered
    .filter(p => !p.internal)
    .filter(p => !servicePorts.has(p.port))
    .map(p => ({
      key: p.key,
      port: p.port,
      protocol: p.protocol,
      name: portMap[p.port] || '',
      enabled: !excludedPorts.has(p.port),
      isProxyLinked: proxyPorts.has(p.port),
      isAutheliaLinked: autheliaPortNumbers.has(p.port),
      isInternetExposed: p.port in internetExposedPorts,
      externalPort: internetExposedPorts[p.port] || null,
      containers: p.containers || [],
    }))
    .sort((a, b) => a.port - b.port)
})

const serviceUrl = computed(() => {
  if (props.selectedNode?.type !== 'service') return null
  const sublabel = props.selectedNode.sublabel
  if (!sublabel || sublabel.startsWith('/')) return null
  return `https://${sublabel}`
})

const nodeTypeLabel = computed(() => {
  const map = {
    root: 'Reverse proxy',
    host: 'Hôte',
    service: 'Service logique',
    port: 'Port',
    authelia: 'Authelia',
    internet: 'Internet',
  }
  return map[props.selectedNode?.type] || 'Nœud'
})

const typeTagClass = computed(() => {
  const type = props.selectedNode?.type
  if (type === 'host') return 'badge bg-secondary-lt text-secondary'
  if (type === 'service') return 'badge bg-cyan-lt text-cyan'
  if (type === 'port') return 'badge bg-blue-lt text-blue'
  if (type === 'authelia') return 'badge bg-purple-lt text-purple'
  if (type === 'internet') return 'badge bg-orange-lt text-orange'
  if (type === 'root') return 'badge bg-teal-lt text-teal'
  return 'badge bg-secondary-lt text-secondary'
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
  while (value >= 1024 && idx < units.length - 1) { value /= 1024; idx++ }
  return `${value.toFixed(1)} ${units[idx]}`
}

function copyUrl(url) {
  navigator.clipboard.writeText(url).catch(() => {})
}
</script>

<style scoped>
.network-node-detail {
  width: 300px;
  min-width: 240px;
  display: flex;
  flex-direction: column;
  background: rgba(10, 15, 30, 0.35);
  border-left: 1px solid rgba(148, 163, 184, 0.15);
  align-self: stretch;
  overflow: hidden;
  flex-shrink: 0;
}

.detail-mobile-header {
  background: rgba(15, 23, 42, 0.5);
  font-size: 12px;
  flex-shrink: 0;
}

.detail-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 14px 12px;
}

.detail-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 40px 16px;
  color: #475569;
}

.detail-section {
  margin-bottom: 14px;
  padding-bottom: 14px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.1);
}

.detail-section:last-child {
  border-bottom: none;
  margin-bottom: 0;
  padding-bottom: 0;
}

.detail-section-title {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.7px;
  color: #475569;
  margin-bottom: 8px;
}

.detail-kv {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 5px;
  font-size: 13px;
}

.detail-key {
  color: #64748b;
  font-size: 12px;
  flex-shrink: 0;
  min-width: 88px;
}

.detail-val {
  color: #e2e8f0;
  word-break: break-word;
}

.port-entry {
  padding: 7px 9px;
  border-radius: 7px;
  background: rgba(15, 23, 42, 0.45);
  border: 1px solid rgba(148, 163, 184, 0.12);
  margin-bottom: 5px;
}

.port-entry-service {
  border-color: rgba(34, 211, 238, 0.2);
  background: rgba(34, 211, 238, 0.04);
}

.port-entry-disabled {
  opacity: 0.5;
}

.port-entry-head {
  display: flex;
  align-items: center;
  gap: 5px;
  flex-wrap: wrap;
  font-size: 12px;
}

.port-badges {
  display: flex;
  gap: 3px;
  flex-wrap: wrap;
  margin-left: auto;
}

.port-entry-sub {
  font-size: 11px;
  margin-top: 3px;
}

.port-entry-actions {
  display: flex;
  gap: 5px;
  margin-top: 6px;
}

.detail-actions {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.btn-xs {
  padding: 2px 8px;
  font-size: 11px;
  border-radius: 4px;
}

@media (max-width: 991px) {
  .network-node-detail {
    width: 100%;
    min-width: 0;
    border-left: none;
    border-top: 1px solid rgba(148, 163, 184, 0.15);
  }
}
</style>
