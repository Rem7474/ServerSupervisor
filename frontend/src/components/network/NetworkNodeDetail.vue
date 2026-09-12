<template>
  <div class="network-node-detail">
    <!-- Header bar -->
    <div class="detail-header">
      <div class="d-flex align-items-center gap-2">
        <span :class="typeTagClass">{{ nodeTypeLabel }}</span>
        <span class="fw-semibold text-light">{{ selectedNode?.label || '—' }}</span>
        <span
          v-if="selectedNode?.sublabel && selectedNode?.type !== 'service'"
          class="text-secondary small"
        >{{ selectedNode.sublabel }}</span>
      </div>
      <button
        type="button"
        class="btn-close"
        :aria-label="t('common.close')"
        @click="$emit('close')"
      />
    </div>

    <!-- Grid of sections -->
    <div class="detail-grid">
      <!-- Network role (host) -->
      <div
        v-if="selectedNode?.type === 'host'"
        class="detail-section"
      >
        <div class="detail-section-title">
          {{ t('network.nodeNetworkRole') }}
        </div>
        <div class="detail-kv">
          <span class="detail-key">{{ t('common.status') }}</span>
          <span :class="statusBadgeClass">
            <span class="status-dot status-dot-animated" />
            {{ hostData?.status || t('network.nodeUnknownStatus') }}
          </span>
        </div>
        <div class="detail-kv">
          <span class="detail-key">{{ t('network.nodeTrafficLabel') }}</span>
          <span class="detail-val small">↓ {{ formatBytes(hostData?.network_rx_bytes || 0) }} / ↑ {{ formatBytes(hostData?.network_tx_bytes || 0) }}</span>
        </div>
        <div class="detail-kv">
          <span class="detail-key">{{ t('network.nodeContainersLabel') }}</span>
          <span class="detail-val">{{ hostContainers.length }} <span class="text-secondary">({{ hostContainers.filter(c => c.state === 'running').length }} {{ t('network.nodeActiveSuffix') }})</span></span>
        </div>
      </div>

      <!-- Network role (Proxmox guest, no agent) -->
      <div
        v-if="selectedNode?.type === 'proxmox_guest'"
        class="detail-section"
      >
        <div class="detail-section-title">
          {{ t('network.nodeProxmoxGuestTitle') }}
        </div>
        <div class="detail-kv">
          <span class="detail-key">{{ t('common.status') }}</span>
          <span :class="statusBadgeClass">
            <span class="status-dot status-dot-animated" />
            {{ selectedNode.status || t('network.nodeUnknownStatus') }}
          </span>
        </div>
        <div class="detail-kv text-secondary small">
          {{ t('network.nodeNoAgentHint') }}
        </div>
      </div>

      <!-- Port node details -->
      <div
        v-if="selectedNode?.type === 'port'"
        class="detail-section"
      >
        <div class="detail-section-title">
          {{ t('network.nodePortTitle') }}
        </div>
        <div class="detail-kv">
          <span class="detail-key">{{ t('network.nodeNumberLabel') }}</span>
          <span class="detail-val fw-semibold">{{ selectedNode.portNumber }}/{{ (selectedNode.protocol || '').toUpperCase() }}</span>
        </div>
        <div
          v-if="selectedNode.containers?.length"
          class="detail-kv"
        >
          <span class="detail-key">{{ t('network.nodeContainersLabel') }}</span>
          <span class="detail-val small">{{ selectedNode.containers.join(', ') }}</span>
        </div>
      </div>

      <!-- Service node details -->
      <div
        v-if="selectedNode?.type === 'service'"
        class="detail-section"
      >
        <div class="detail-section-title">
          {{ t('network.nodeServiceTitle') }}
        </div>
        <div class="detail-kv">
          <span class="detail-key">{{ t('network.nodeInternalPortLabel') }}</span>
          <span class="detail-val fw-semibold">{{ selectedNode.internalPort || '-' }}</span>
        </div>
        <div
          v-if="selectedNode.externalPort"
          class="detail-kv"
        >
          <span class="detail-key">{{ t('network.nodeExternalPortLabel') }}</span>
          <span class="detail-val">{{ selectedNode.externalPort }}</span>
        </div>
        <div
          v-if="serviceUrl"
          class="detail-kv"
        >
          <span class="detail-key">{{ t('network.nodeUrlLabel') }}</span>
          <a
            :href="serviceUrl"
            target="_blank"
            rel="noopener"
            class="detail-val small text-blue"
            style="word-break:break-all"
          >{{ serviceUrl }}</a>
        </div>
      </div>

      <!-- Integration (service / port) -->
      <div
        v-if="['service', 'port'].includes(selectedNode?.type || '')"
        class="detail-section"
      >
        <div class="detail-section-title">
          {{ t('network.nodeIntegrationTitle') }}
        </div>
        <div class="detail-kv">
          <span class="detail-key">{{ t('network.nodeReverseProxyLabel') }}</span>
          <span
            v-if="selectedNode?.isProxyLinked"
            class="badge bg-blue-lt text-blue"
          >{{ t('network.nodeYes') }}</span>
          <span
            v-else
            class="text-secondary small"
          >{{ t('network.nodeNo') }}</span>
        </div>
        <div class="detail-kv">
          <span class="detail-key">Authelia</span>
          <span
            v-if="selectedNode?.isAutheliaLinked"
            class="badge bg-purple-lt text-purple"
          >{{ t('network.nodeYes') }}</span>
          <span
            v-else
            class="text-secondary small"
          >{{ t('network.nodeNo') }}</span>
        </div>
        <div class="detail-kv">
          <span class="detail-key">Internet</span>
          <span
            v-if="selectedNode?.isInternetExposed"
            class="badge bg-orange-lt text-orange"
          >
            {{ selectedNode?.externalPort ? t('network.nodeExposedWithPort', { port: selectedNode.externalPort }) : t('network.nodeExposedLabel') }}
          </span>
          <span
            v-else
            class="text-secondary small"
          >{{ t('network.nodeNotExposedLabel') }}</span>
        </div>
      </div>

      <!-- Ports & services (host) -->
      <div
        v-if="selectedNode?.type === 'host' && (hostServices.length > 0 || allHostPorts.length > 0)"
        class="detail-section detail-section-wide"
      >
        <div class="detail-section-title">
          {{ t('network.nodePortsServicesTitle') }}
        </div>
        <div class="port-chips">
          <span
            v-for="svc in hostServices"
            :key="svc.id"
            class="port-chip port-chip-service"
            :title="svc.domain ? `https://${svc.domain}${svc.path || '/'}` : t('network.nodePortTitleAttr', { port: svc.internalPort })"
          >
            <span class="fw-semibold">{{ svc.name }}</span>
            <span class="text-secondary ms-1">:{{ svc.internalPort }}</span>
            <span
              v-if="svc.linkToProxy"
              class="badge bg-blue-lt text-blue ms-1"
            >proxy</span>
            <span
              v-if="svc.linkToAuthelia"
              class="badge bg-purple-lt text-purple ms-1"
            >auth</span>
            <span
              v-if="svc.exposedToInternet"
              class="badge bg-orange-lt text-orange ms-1"
            >inet</span>
          </span>
          <span
            v-for="p in allHostPorts"
            :key="p.key"
            class="port-chip"
            :class="{ 'port-chip-disabled': !p.enabled }"
          >
            <span :class="{ 'text-secondary': !p.enabled }">{{ p.name || (p.port + '/' + (p.protocol || '').toUpperCase()) }}</span>
            <span
              v-if="p.name"
              class="text-secondary ms-1"
            >:{{ p.port }}</span>
            <span
              v-if="!p.enabled"
              class="badge bg-secondary-lt text-secondary ms-1"
            >{{ t('network.nodeOffBadge') }}</span>
            <template v-else>
              <span
                v-if="p.isProxyLinked"
                class="badge bg-blue-lt text-blue ms-1"
              >proxy</span>
              <span
                v-if="p.isAutheliaLinked"
                class="badge bg-purple-lt text-purple ms-1"
              >auth</span>
              <span
                v-if="p.isInternetExposed"
                class="badge bg-orange-lt text-orange ms-1"
              >inet</span>
            </template>
          </span>
        </div>
      </div>

      <!-- Actions -->
      <div class="detail-section detail-actions">
        <div class="detail-section-title">
          {{ t('network.nodeActionsTitle') }}
        </div>
        <router-link
          v-if="selectedNode?.type === 'host' && selectedNode.hostId"
          :to="`/hosts/${selectedNode.hostId}?tab=exposition`"
          class="btn btn-sm btn-outline-primary"
        >
          <IconHome
            :size="14"
            class="me-1"
          />
          {{ t('network.nodeOpenHost') }}
        </router-link>
        <router-link
          v-if="selectedNode?.type !== 'host' && selectedNode?.hostId"
          :to="`/hosts/${selectedNode.hostId}?tab=exposition`"
          class="btn btn-sm btn-outline-secondary"
        >
          {{ t('network.nodeViewAssociatedHost') }}
        </router-link>
        <router-link
          v-if="selectedNode?.type === 'proxmox_guest' && selectedNode.guestId"
          :to="`/proxmox/guests/${selectedNode.guestId}`"
          class="btn btn-sm btn-outline-primary"
        >
          <IconHome
            :size="14"
            class="me-1"
          />
          {{ t('network.nodeOpenVM') }}
        </router-link>
        <router-link
          v-if="selectedNode?.type === 'service' && selectedNode.guestId"
          :to="`/proxmox/guests/${selectedNode.guestId}`"
          class="btn btn-sm btn-outline-secondary"
        >
          {{ t('network.nodeViewAssociatedVM') }}
        </router-link>
        <template v-if="selectedNode?.type === 'service' && serviceUrl">
          <a
            :href="serviceUrl"
            target="_blank"
            rel="noopener"
            class="btn btn-sm btn-outline-primary"
          >{{ t('network.nodeOpenInBrowser') }}</a>
          <button
            type="button"
            class="btn btn-sm btn-outline-secondary"
            @click="copyUrl(serviceUrl)"
          >
            {{ t('network.nodeCopyUrl') }}
          </button>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconHome } from '@tabler/icons-vue'

interface SelectedNode {
  type?: string
  hostId?: string
  guestId?: string
  status?: string
  sublabel?: string
  portNumber?: number | string
  protocol?: string
  containers?: string[]
  internalPort?: number | string
  externalPort?: number | string
  isProxyLinked?: boolean
  isAutheliaLinked?: boolean
  isInternetExposed?: boolean
  [key: string]: unknown
}

interface Host {
  id: string
  status?: string
  network_rx_bytes?: number
  network_tx_bytes?: number
  [key: string]: unknown
}

interface Container {
  host_id: string
  state?: string
  [key: string]: unknown
}

interface ServicePort {
  id?: string | number
  hostId?: string
  internalPort?: number | string
  name?: string
  domain?: string
  path?: string
  linkToProxy?: boolean
  linkToAuthelia?: boolean
  exposedToInternet?: boolean
}

interface DiscoveredPort {
  key: string
  port: number
  protocol?: string
  internal?: boolean
  containers?: unknown[]
}

interface HostPortOverride {
  excludedPorts?: number[]
  portMap?: Record<number, string>
  proxyPorts?: Set<number>
  autheliaPortNumbers?: Set<number>
  internetExposedPorts?: Record<number, number | null>
}

const props = withDefaults(defineProps<{
  selectedNode?: SelectedNode | null
  hosts?: Host[]
  containers?: Container[]
  hostPortOverrides?: Record<string, HostPortOverride>
  combinedServices?: ServicePort[]
  discoveredPortsByHost?: Record<string, DiscoveredPort[]>
}>(), {
  selectedNode: null,
  hosts: () => [],
  containers: () => [],
  hostPortOverrides: () => ({}),
  combinedServices: () => [],
  discoveredPortsByHost: () => ({}),
})

defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()
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
  const override: HostPortOverride = props.hostPortOverrides?.[hostId] || {}
  const excludedPorts = new Set((override.excludedPorts || []).map(Number))
  const portMap: Record<number, string> = override.portMap || {}
  const proxyPorts: Set<number> = override.proxyPorts || new Set<number>()
  const autheliaPortNumbers: Set<number> = override.autheliaPortNumbers || new Set<number>()
  const internetExposedPorts: Record<number, number | null> = override.internetExposedPorts || {}

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
  const map: Record<string, string> = {
    root: t('network.nodeReverseProxyLabel'),
    host: t('network.nodeTypeHost'),
    proxmox_guest: t('network.nodeProxmoxGuestTitle'),
    service: t('network.nodeServiceTitle'),
    port: t('network.nodePortTitle'),
    authelia: 'Authelia',
    internet: 'Internet',
  }
  return map[props.selectedNode?.type || ''] || t('network.nodeTypeDefault')
})

const typeTagClass = computed(() => {
  const type = props.selectedNode?.type
  if (type === 'host') return 'badge bg-secondary-lt text-secondary'
  if (type === 'proxmox_guest') return 'badge bg-yellow-lt text-yellow'
  if (type === 'service') return 'badge bg-cyan-lt text-cyan'
  if (type === 'port') return 'badge bg-blue-lt text-blue'
  if (type === 'authelia') return 'badge bg-purple-lt text-purple'
  if (type === 'internet') return 'badge bg-orange-lt text-orange'
  if (type === 'root') return 'badge bg-teal-lt text-teal'
  return 'badge bg-secondary-lt text-secondary'
})

const statusBadgeClass = computed(() => {
  const status = hostData.value?.status || props.selectedNode?.status
  if (status === 'online') return 'status status-success'
  if (status === 'warning') return 'status status-warning'
  if (status === 'offline') return 'status status-danger'
  return 'status status-secondary'
})

function formatBytes(bytes: number | undefined): string {
  if (!bytes && bytes !== 0) return '-'
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let value = bytes / 1024
  let idx = 0
  while (value >= 1024 && idx < units.length - 1) { value /= 1024; idx++ }
  return `${value.toFixed(1)} ${units[idx]}`
}

function copyUrl(url: string): void {
  navigator.clipboard.writeText(url).catch(() => {})
}
</script>

<style scoped>
.network-node-detail {
  border-top: 1px solid var(--ss-border-default);
  background: rgba(10, 15, 30, 0.5);
}

/* Header bar */
.detail-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  border-bottom: 1px solid var(--ss-border-soft);
  background: var(--ss-panel-medium);
  gap: 8px;
}

/* Grid of info sections */
.detail-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 0;
}

.detail-section {
  padding: 12px 16px;
  border-right: 1px solid var(--ss-border-soft);
  min-width: 180px;
  flex: 0 0 auto;
}

.detail-section-wide {
  flex: 1 1 260px;
}

.detail-actions {
  margin-left: auto;
  display: flex;
  flex-direction: column;
  gap: 5px;
  border-right: none;
  min-width: 160px;
}

.detail-section-title {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.7px;
  color: var(--ss-text-subtle-on-dark);
  margin-bottom: 8px;
}

.detail-kv {
  display: flex;
  align-items: baseline;
  gap: 6px;
  margin-bottom: 4px;
  font-size: 13px;
}

.detail-key {
  color: var(--ss-text-subtle-on-dark);
  font-size: 12px;
  flex-shrink: 0;
  min-width: 80px;
}

.detail-val {
  color: var(--ss-text-on-dark);
  word-break: break-word;
}

/* Port chips (compact display for host ports) */
.port-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}

.port-chip {
  display: inline-flex;
  align-items: center;
  padding: 3px 8px;
  border-radius: 6px;
  background: var(--ss-panel-medium);
  border: 1px solid var(--ss-border-soft);
  font-size: 12px;
  color: var(--ss-text-on-dark);
}

.port-chip-service {
  border-color: rgba(34, 211, 238, 0.25);
  background: rgba(34, 211, 238, 0.06);
}

.port-chip-disabled {
  opacity: 0.45;
}

@media (max-width: 767px) {
  .detail-grid {
    flex-direction: column;
  }

  .detail-section {
    border-right: none;
    border-bottom: 1px solid var(--ss-border-soft);
  }

  .detail-actions {
    margin-left: 0;
  }
}
</style>
