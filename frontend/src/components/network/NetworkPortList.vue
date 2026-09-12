<template>
  <div>
    <div class="card mb-4">
      <div class="card-body">
        <div class="row g-3">
          <div class="col-md-6 col-lg-3">
            <input
              v-model="search"
              type="text"
              class="form-control"
              :placeholder="t('network.portListSearchPlaceholder')"
            >
          </div>
          <div class="col-md-6 col-lg-3">
            <select
              v-model="protocolFilter"
              class="form-select"
            >
              <option value="">
                {{ t('network.portListAllProtocols') }}
              </option>
              <option value="tcp">
                TCP
              </option>
              <option value="udp">
                UDP
              </option>
            </select>
          </div>
          <div class="col-md-6 col-lg-3">
            <select
              v-model="hostFilter"
              class="form-select"
            >
              <option value="">
                {{ t('network.portListAllHosts') }}
              </option>
              <option
                v-for="h in hosts"
                :key="h.id"
                :value="h.id"
              >
                {{ h.name || h.hostname || h.id }}
              </option>
            </select>
          </div>
          <div class="col-md-6 col-lg-3">
            <label class="form-check form-switch">
              <input
                v-model="onlyPublished"
                class="form-check-input"
                type="checkbox"
              >
              <span class="form-check-label">{{ t('network.portListPublishedOnly') }}</span>
            </label>
          </div>
        </div>
      </div>
    </div>

    <div class="card mb-4">
      <div class="table-responsive scroll-table">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>{{ t('network.portListHostColumn') }}</th>
              <th>{{ t('network.portListContainerColumn') }}</th>
              <th>{{ t('network.portListImageColumn') }}</th>
              <th>{{ t('network.portListHostPortColumn') }}</th>
              <th>{{ t('network.portListContainerPortColumn') }}</th>
              <th>{{ t('network.portListProtoColumn') }}</th>
              <th>IPv4</th>
              <th>IPv6</th>
              <th>{{ t('network.portListStateColumn') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="row in portRows"
              :key="row.key"
            >
              <td>
                <router-link
                  :to="`/hosts/${row.host_id}`"
                  class="text-decoration-none"
                >
                  {{ row.host_name || row.host_id }}
                </router-link>
              </td>
              <td class="fw-semibold">
                {{ row.container_name }}
              </td>
              <td>
                <div>{{ row.image }}</div>
                <div class="text-secondary small">
                  <code>{{ row.image_tag || '-' }}</code>
                </div>
              </td>
              <td class="fw-semibold">
                {{ row.host_port || '-' }}
              </td>
              <td class="text-secondary">
                {{ row.container_port || '-' }}
              </td>
              <td class="text-secondary text-uppercase">
                {{ row.protocol || '-' }}
              </td>
              <td class="text-secondary small font-monospace">
                <span
                  v-if="row.ipv4"
                  class="badge bg-blue-lt text-blue"
                >{{ row.ipv4 }}</span>
                <span
                  v-else
                  class="text-muted"
                >-</span>
              </td>
              <td class="text-secondary small font-monospace">
                <span
                  v-if="row.ipv6"
                  class="badge bg-purple-lt text-purple"
                >{{ row.ipv6 }}</span>
                <span
                  v-else
                  class="text-muted"
                >-</span>
              </td>
              <td>
                <span :class="row.state === 'running' ? 'badge bg-success-lt text-success' : 'badge bg-secondary-lt text-secondary'">
                  {{ containerStateLabels[row.state || ''] || row.state || t('network.nodeUnknownStatus') }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyState
        v-if="portRows.length === 0"
        :title="t('network.portListNoPortsTitle')"
      />
    </div>

    <div class="card">
      <div class="card-header">
        <h3 class="card-title">
          {{ t('network.portListTrafficByHostTitle') }}
        </h3>
        <div class="card-options">
          <span class="badge bg-azure-lt text-azure ms-1">
            {{ t('network.portListHostCountBadge', { count: hosts.length }, hosts.length) }}
          </span>
        </div>
      </div>
      <div class="table-responsive scroll-table">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>{{ t('network.portListHostColumn') }}</th>
              <th>{{ t('network.portListIpColumn') }}</th>
              <th class="text-end">
                ↓ {{ t('network.portListRxColumn') }}
              </th>
              <th class="text-end">
                ↑ {{ t('network.portListTxColumn') }}
              </th>
              <th>{{ t('common.status') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="h in hosts"
              :key="h.id"
            >
              <td>
                <router-link
                  :to="`/hosts/${h.id}`"
                  class="fw-semibold text-decoration-none"
                >
                  {{ h.name || h.hostname || h.id }}
                </router-link>
              </td>
              <td class="text-secondary">
                {{ h.ip_address }}
              </td>
              <td class="text-end font-monospace small text-info">
                {{ formatBytes(h.network_rx_bytes || 0) }}
              </td>
              <td class="text-end font-monospace small text-warning">
                {{ formatBytes(h.network_tx_bytes || 0) }}
              </td>
              <td>
                <span :class="h.status === 'online' ? 'status status-success' : h.status === 'warning' ? 'status status-warning' : 'status status-danger'">
                  <span class="status-dot status-dot-animated" />
                  <span :data-translation-id="h.status === 'online' ? 'online' : h.status === 'offline' ? 'offline' : 'unknown'">{{ h.status || t('network.nodeUnknownStatus') }}</span>
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyState
        v-if="hosts.length === 0"
        :title="t('network.portListNoHostsTitle')"
      />
    </div>

    <div
      v-if="containersWithNetStats.length"
      class="card mt-4"
    >
      <div class="card-header">
        <h3 class="card-title">
          {{ t('network.portListTrafficByContainerTitle') }}
        </h3>
        <div class="card-options">
          <span class="badge bg-azure-lt text-azure ms-1">
            {{ t('network.portListContainerCountBadge', { count: containersWithNetStats.length }, containersWithNetStats.length) }}
          </span>
        </div>
      </div>
      <div class="table-responsive scroll-table">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>{{ t('network.portListContainerColumn') }}</th>
              <th>{{ t('network.portListHostColumn') }}</th>
              <th class="text-end">
                ↓ {{ t('network.portListRxColumn') }}
              </th>
              <th class="text-end">
                ↑ {{ t('network.portListTxColumn') }}
              </th>
              <th class="text-end">
                {{ t('network.portListTotalColumn') }}
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="c in containersWithNetStats"
              :key="c.id"
            >
              <td class="fw-semibold">
                {{ c.name }}
              </td>
              <td class="text-secondary">
                {{ c.hostname }}
              </td>
              <td class="text-end font-monospace small text-info">
                {{ formatBytes(c.net_rx_bytes) }}
              </td>
              <td class="text-end font-monospace small text-warning">
                {{ formatBytes(c.net_tx_bytes) }}
              </td>
              <td class="text-end font-monospace small fw-semibold">
                {{ formatBytes((c.net_rx_bytes ?? 0) + (c.net_tx_bytes ?? 0)) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div
      v-if="ipInventoryLoading || proxmoxGuests.length"
      class="card mt-4"
    >
      <div class="card-header">
        <h3 class="card-title">
          {{ t('network.portListProxmoxIpTitle') }}
        </h3>
        <div class="card-options">
          <span
            v-if="ipInventoryLoading"
            class="spinner-border spinner-border-sm text-secondary"
          />
          <span
            v-else
            class="badge bg-azure-lt text-azure ms-1"
          >
            {{ t('network.portListResourceCountBadge', { count: proxmoxGuests.length }, proxmoxGuests.length) }}
          </span>
        </div>
      </div>
      <div class="table-responsive scroll-table">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>{{ t('network.portListResourceColumn') }}</th>
              <th>{{ t('network.portListNodeColumn') }}</th>
              <th>
                <SortableHeader
                  :label="t('network.portListIpAddressesColumn')"
                  :active="true"
                  :direction="proxmoxIpSortDir"
                  @toggle="toggleProxmoxIpSort"
                />
              </th>
              <th>{{ t('network.portListCorrelatedHostColumn') }}</th>
              <th>{{ t('network.portListStateColumn') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="g in sortedProxmoxGuests"
              :key="g.guest_id"
            >
              <td>
                <span class="fw-semibold">{{ g.name }}</span>
                <span
                  class="badge ms-2"
                  :class="g.guest_type === 'lxc' ? 'bg-purple-lt text-purple' : 'bg-blue-lt text-blue'"
                >{{ g.guest_type === 'lxc' ? 'LXC' : 'VM' }}</span>
              </td>
              <td class="text-secondary">
                {{ g.node }}
              </td>
              <td class="text-secondary small font-monospace">
                <span
                  v-if="g.ip_addresses.length === 0"
                  class="text-muted"
                >-</span>
                <span
                  v-for="ip in g.ip_addresses"
                  :key="ip"
                  class="badge bg-blue-lt text-blue me-1"
                >{{ ip }}</span>
              </td>
              <td>
                <router-link
                  v-if="g.host_id"
                  :to="`/hosts/${g.host_id}?tab=exposition`"
                  class="text-decoration-none"
                >
                  {{ g.host_name || g.host_id }}
                </router-link>
                <span
                  v-else
                  class="text-muted"
                >{{ t('network.portListNotLinked') }}</span>
              </td>
              <td>
                <span :class="g.status === 'running' ? 'badge bg-success-lt text-success' : 'badge bg-secondary-lt text-secondary'">
                  {{ g.status === 'running' ? t('network.portListStateRunning') : g.status || t('network.nodeUnknownStatus') }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyState
        v-if="!ipInventoryLoading && proxmoxGuests.length === 0"
        :title="t('network.portListNoProxmoxIpTitle')"
      />
    </div>

    <div
      v-if="ipInventoryLoading || npmEntries.length"
      class="card mt-4"
    >
      <div class="card-header">
        <h3 class="card-title">
          {{ t('network.portListNpmDomainsTitle') }}
        </h3>
        <div class="card-options">
          <span
            v-if="ipInventoryLoading"
            class="spinner-border spinner-border-sm text-secondary"
          />
          <span
            v-else
            class="badge bg-azure-lt text-azure ms-1"
          >
            {{ t('network.portListDomainCountBadge', { count: npmEntries.length }, npmEntries.length) }}
          </span>
        </div>
      </div>
      <div class="table-responsive scroll-table">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>{{ t('network.portListDomainsColumn') }}</th>
              <th>
                <SortableHeader
                  :label="t('network.portListIpTargetHostColumn')"
                  :active="true"
                  :direction="npmIpSortDir"
                  @toggle="toggleNpmIpSort"
                />
              </th>
              <th>{{ t('network.flowsPortColumn') }}</th>
              <th>{{ t('network.portListCorrelatedResourceColumn') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="n in sortedNpmEntries"
              :key="n.proxy_host_id"
            >
              <td class="fw-semibold">
                {{ (n.domain_names || []).join(', ') || '-' }}
              </td>
              <td class="text-secondary small font-monospace">
                {{ n.forward_host }}
              </td>
              <td class="text-secondary">
                {{ n.forward_port }}
              </td>
              <td>
                <router-link
                  v-if="n.matched_type === 'host'"
                  :to="`/hosts/${n.matched_id}?tab=exposition`"
                  class="text-decoration-none"
                >
                  {{ n.matched_name }}
                </router-link>
                <router-link
                  v-else-if="n.matched_type === 'proxmox_guest'"
                  :to="`/proxmox/guests/${n.matched_id}`"
                  class="text-decoration-none"
                >
                  {{ n.matched_name }}
                </router-link>
                <span
                  v-else
                  class="badge bg-secondary-lt text-secondary"
                >{{ t('network.portListNotResolved') }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyState
        v-if="!ipInventoryLoading && npmEntries.length === 0"
        :title="t('network.portListNoNpmTitle')"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { NetworkProxmoxGuestIP, NetworkNPMEntry } from '../../types/network'
import SortableHeader from '../common/SortableHeader.vue'
import EmptyState from '../EmptyState.vue'

interface PortMapping {
  host_port?: number | string
  container_port?: number | string
  protocol?: string
  host_ip?: string
}

interface Container {
  id: string
  host_id: string
  hostname?: string
  name?: string
  image?: string
  image_tag?: string
  state?: string
  net_rx_bytes?: number
  net_tx_bytes?: number
  port_mappings?: PortMapping[]
}

interface Host {
  id: string
  name?: string
  hostname?: string
  ip_address?: string
  status?: string
  network_rx_bytes?: number
  network_tx_bytes?: number
}

interface PortRow {
  key: string
  host_id: string
  host_name?: string
  container_name?: string
  image?: string
  image_tag?: string
  state?: string
  host_port: number
  container_port?: number | string
  protocol?: string
  ipv4: string | null
  ipv6: string | null
}

const props = withDefaults(defineProps<{
  hosts?: Host[]
  containers?: Container[]
  proxmoxGuests?: NetworkProxmoxGuestIP[]
  npmEntries?: NetworkNPMEntry[]
  ipInventoryLoading?: boolean
}>(), {
  hosts: () => [],
  containers: () => [],
  proxmoxGuests: () => [],
  npmEntries: () => [],
  ipInventoryLoading: false,
})

const i18n = useI18n()
const { t } = i18n
const search = ref('')
const protocolFilter = ref('')
const hostFilter = ref('')
const onlyPublished = ref(true)

const containerStateLabels = computed<Record<string, string>>(() => ({
  running: t('network.portListStateRunning'),
  exited: t('network.portListStateExited'),
  paused: t('network.portListStatePaused'),
  created: t('network.portListStateCreated'),
  restarting: t('network.portListStateRestarting'),
  dead: t('network.portListStateDead'),
}))

const portRows = computed<PortRow[]>(() => {
  const grouped = new Map<string, PortRow>()

  for (const container of props.containers) {
    const mappings = container.port_mappings || []
    for (const mapping of mappings) {
      const hostPort = Number(mapping.host_port || 0)
      const isPublished = hostPort > 0
      if (onlyPublished.value && !isPublished) continue

      const groupKey = `${container.id}-${hostPort}-${mapping.container_port}-${mapping.protocol}`

      if (!grouped.has(groupKey)) {
        grouped.set(groupKey, {
          key: groupKey,
          host_id: container.host_id,
          host_name: container.hostname,
          container_name: container.name,
          image: container.image,
          image_tag: container.image_tag,
          state: container.state,
          host_port: hostPort,
          container_port: mapping.container_port,
          protocol: mapping.protocol,
          ipv4: null,
          ipv6: null,
        })
      }

      const row = grouped.get(groupKey)!
      const ip = mapping.host_ip || ''
      if (ip.includes(':')) {
        row.ipv6 = ip
      } else {
        row.ipv4 = ip || '0.0.0.0'
      }
    }
  }

  const rows = [...grouped.values()]
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
      (row.ipv4 || '').includes(query) ||
      (row.ipv6 || '').includes(query)

    return matchHost && matchProto && matchSearch
  })
})

const containersWithNetStats = computed(() =>
  [...props.containers]
    .filter((c) => c.state === 'running' && ((c.net_rx_bytes ?? 0) > 0 || (c.net_tx_bytes ?? 0) > 0))
    .sort((a, b) => ((b.net_rx_bytes ?? 0) + (b.net_tx_bytes ?? 0)) - ((a.net_rx_bytes ?? 0) + (a.net_tx_bytes ?? 0)))
)

// ─── IP sorting (Adresses IP Proxmox / Domaines NPM tables) ───────────────
// IPv4 addresses sort numerically (10.0.0.9 before 10.0.0.10), not
// lexicographically like a plain string compare would; anything that isn't
// a parseable IPv4 (a hostname in NPM's forward_host, or no IP at all) falls
// back to a locale string compare and always sorts after real IPs.
function ipToComparable(ip: string): number | null {
  if (!ip) return null
  const parts = ip.split('.')
  if (parts.length !== 4) return null
  let value = 0
  for (const part of parts) {
    const n = Number(part)
    if (!Number.isInteger(n) || n < 0 || n > 255) return null
    value = value * 256 + n
  }
  return value
}

function compareIPs(a: string, b: string, direction: 'asc' | 'desc'): number {
  const dir = direction === 'asc' ? 1 : -1
  const av = ipToComparable(a)
  const bv = ipToComparable(b)
  if (av === null && bv === null) return a.localeCompare(b, i18n.locale.value, { sensitivity: 'base' }) * dir
  if (av === null) return 1 * dir
  if (bv === null) return -1 * dir
  if (av < bv) return -1 * dir
  if (av > bv) return 1 * dir
  return 0
}

const proxmoxIpSortDir = ref<'asc' | 'desc'>('asc')
function toggleProxmoxIpSort(): void {
  proxmoxIpSortDir.value = proxmoxIpSortDir.value === 'asc' ? 'desc' : 'asc'
}
const sortedProxmoxGuests = computed(() =>
  [...props.proxmoxGuests].sort((a, b) =>
    compareIPs(a.ip_addresses[0] || '', b.ip_addresses[0] || '', proxmoxIpSortDir.value),
  ),
)

const npmIpSortDir = ref<'asc' | 'desc'>('asc')
function toggleNpmIpSort(): void {
  npmIpSortDir.value = npmIpSortDir.value === 'asc' ? 'desc' : 'asc'
}
const sortedNpmEntries = computed(() =>
  [...props.npmEntries].sort((a, b) => compareIPs(a.forward_host || '', b.forward_host || '', npmIpSortDir.value)),
)

function formatBytes(bytes: number | undefined): string {
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

