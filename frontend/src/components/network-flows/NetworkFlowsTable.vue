<template>
  <div>
    <div
      v-if="collectorAvailable === false"
      class="card"
    >
      <div class="card-body">
        <EmptyState
          :icon="IconWifiOff"
          :title="t('network.flowsUnavailableTitle')"
          :subtitle="t('network.flowsUnavailableSubtitle')"
        />
      </div>
    </div>

    <template v-else>
      <NetworkFlowsHistoryChart
        :host-id="hostId"
        mode="summary"
        :refresh-tick="refreshTick"
        class="mb-4"
      />

      <div class="card">
        <div class="card-header d-flex align-items-center justify-content-between flex-wrap gap-2">
          <h3 class="card-title mb-0">
            {{ t('network.flowsTopTalkersTitle') }}
            <span
              v-if="totalFlows > talkers.length"
              class="text-muted small ms-1"
            >{{ t('network.flowsDisplayedCount', { shown: talkers.length, total: totalFlows }) }}</span>
            <span
              class="text-muted small ms-1"
              :title="t('network.flowsSnapshotHint')"
            >{{ t('network.flowsSnapshotLabel') }}</span>
          </h3>
          <div class="d-flex align-items-center gap-2">
            <select
              v-model="protocolFilter"
              class="form-select form-select-sm"
              style="width: auto;"
            >
              <option value="">
                {{ t('network.flowsAllProtocols') }}
              </option>
              <optgroup :label="t('network.flowsTransportGroup')">
                <option value="tcp">
                  TCP
                </option>
                <option value="udp">
                  UDP
                </option>
              </optgroup>
              <optgroup
                v-if="serviceOptions.length > 0"
                :label="t('network.flowsServiceGroup')"
              >
                <option
                  v-for="svc in serviceOptions"
                  :key="svc"
                  :value="`svc:${svc}`"
                >
                  {{ svc }}
                </option>
              </optgroup>
            </select>
            <input
              v-model="search"
              type="text"
              class="form-control form-control-sm"
              style="width: auto;"
              :placeholder="t('network.flowsSearchPlaceholder')"
            >
          </div>
        </div>

        <div
          v-if="loading"
          class="card-body"
        >
          <LoadingSkeleton
            variant="card"
            :lines="4"
          />
        </div>
        <div
          v-else-if="filteredTalkers.length === 0"
          class="card-body"
        >
          <EmptyState
            :icon="IconNetwork"
            :title="t('network.flowsEmptyTitle')"
            :subtitle="t('network.flowsEmptySubtitle')"
          />
        </div>
        <div
          v-else
          class="table-responsive"
        >
          <table class="table table-vcenter card-table mb-0">
            <thead>
              <tr>
                <th>
                  <SortableHeader
                    :label="t('network.flowsProcessColumn')"
                    :active="sortKey === 'process_name'"
                    :direction="sortDir"
                    @toggle="toggleSort('process_name')"
                  />
                </th>
                <th>
                  <SortableHeader
                    :label="t('network.flowsRemoteIpColumn')"
                    :active="sortKey === 'remote_ip'"
                    :direction="sortDir"
                    @toggle="toggleSort('remote_ip')"
                  />
                </th>
                <th>
                  <SortableHeader
                    :label="t('network.flowsPortColumn')"
                    :active="sortKey === 'remote_port'"
                    :direction="sortDir"
                    @toggle="toggleSort('remote_port')"
                  />
                </th>
                <th>
                  <SortableHeader
                    :label="t('network.flowsProtocolColumn')"
                    :active="sortKey === 'protocol'"
                    :direction="sortDir"
                    @toggle="toggleSort('protocol')"
                  />
                </th>
                <th>
                  <SortableHeader
                    :label="t('network.flowsDirectionColumn')"
                    :active="sortKey === 'direction'"
                    :direction="sortDir"
                    @toggle="toggleSort('direction')"
                  />
                </th>
                <th class="text-end">
                  <SortableHeader
                    :label="t('network.flowsRxColumn')"
                    :active="sortKey === 'rx_bytes'"
                    :direction="sortDir"
                    @toggle="toggleSort('rx_bytes')"
                  />
                </th>
                <th class="text-end">
                  <SortableHeader
                    :label="t('network.flowsTxColumn')"
                    :active="sortKey === 'tx_bytes'"
                    :direction="sortDir"
                    @toggle="toggleSort('tx_bytes')"
                  />
                </th>
                <th class="text-end">
                  <SortableHeader
                    :label="t('network.flowsConnectionsColumn')"
                    :active="sortKey === 'connections'"
                    :direction="sortDir"
                    @toggle="toggleSort('connections')"
                  />
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="t in sortedTalkers"
                :key="`${t.is_others}-${t.remote_ip}-${t.remote_port}-${t.protocol}-${t.direction}`"
                :class="t.is_others ? 'text-muted fst-italic' : 'clickable-row'"
                :role="t.is_others ? undefined : 'button'"
                :tabindex="t.is_others ? undefined : 0"
                @click="!t.is_others && openDrilldown(t)"
                @keydown.enter="!t.is_others && openDrilldown(t)"
                @keydown.space.prevent="!t.is_others && openDrilldown(t)"
              >
                <template v-if="t.is_others">
                  <td colspan="4">
                    {{ i18n.t('network.flowsOthersLabel', { count: t.connections }, t.connections) }}
                  </td>
                </template>
                <template v-else>
                  <td>
                    <span v-if="t.process_name">{{ t.process_name }}<span
                      v-if="t.pid"
                      class="text-muted small"
                    > (PID {{ t.pid }})</span></span>
                    <span
                      v-else
                      class="text-muted small"
                    >{{ i18n.t('network.flowsUnknownProcess') }}</span>
                  </td>
                  <td class="fw-medium">
                    {{ t.remote_ip }}
                  </td>
                  <td>{{ t.remote_port }}</td>
                  <td>
                    <div class="d-flex align-items-center gap-1 flex-wrap">
                      <span class="badge bg-secondary-lt text-secondary text-uppercase">{{ t.protocol }}</span>
                      <!-- A real SNI hostname is presented plainly; a port-based
                           guess is muted + marked so it never reads as a fact. -->
                      <span
                        v-if="labelFor(t).source === 'sni'"
                        class="badge bg-purple-lt text-purple"
                        :title="i18n.t('network.flowsSniTitle')"
                      >{{ labelFor(t).text }}</span>
                      <span
                        v-else-if="labelFor(t).source === 'port'"
                        class="text-muted small heuristic-label"
                        :title="PORT_GUESS_HINT"
                      >≈ {{ labelFor(t).text }}</span>
                    </div>
                  </td>
                  <td>
                    <span
                      v-if="t.direction === 'inbound'"
                      class="badge bg-azure-lt text-azure"
                    >{{ i18n.t('network.flowsInboundBadge') }}</span>
                    <span
                      v-else
                      class="badge bg-teal-lt text-teal"
                    >{{ i18n.t('network.flowsOutboundBadge') }}</span>
                  </td>
                </template>
                <td class="text-end">
                  {{ formatBytes(t.rx_bytes) }}
                </td>
                <td class="text-end">
                  {{ formatBytes(t.tx_bytes) }}
                </td>
                <td class="text-end">
                  {{ t.connections }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>

    <div
      v-if="drilldownTalker"
      ref="drilldownModalRef"
      class="modal modal-blur fade show"
      style="display: block"
      tabindex="-1"
      role="dialog"
      aria-modal="true"
    >
      <div class="modal-dialog modal-dialog-centered modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">
              {{ drilldownTalker.remote_ip }}:{{ drilldownTalker.remote_port }} ({{ drilldownTalker.protocol }})
            </h5>
            <button
              type="button"
              class="btn-close"
              :aria-label="t('common.close')"
              @click="drilldownTalker = null"
            />
          </div>
          <div class="modal-body">
            <NetworkFlowsHistoryChart
              :host-id="hostId"
              mode="talker"
              :remote-ip="drilldownTalker.remote_ip ?? ''"
              :remote-port="drilldownTalker.remote_port ?? 0"
              :protocol="drilldownTalker.protocol ?? ''"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconNetwork, IconWifiOff } from '@tabler/icons-vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import EmptyState from '../EmptyState.vue'
import SortableHeader from '../common/SortableHeader.vue'
import NetworkFlowsHistoryChart from './NetworkFlowsHistoryChart.vue'
import { useNetworkFlows } from '../../composables/useNetworkFlows'
import { useModalChrome } from '../../composables/useModalChrome'
import { formatBytes } from '../../utils/formatters'
import { compareValues } from '../../utils/sort'
import { protocolLabelFor, PORT_GUESS_HINT, type ProtocolLabel } from '../../utils/portServices'
import { SERVICE_FILTER_PREFIX, type NetworkFlowFilterValue, type NetworkFlowMetric } from '../../types/networkFlows'

const props = withDefaults(defineProps<{
  hostId: string
  initialData?: NetworkFlowMetric[] | null
  /** Whether this host's agent reports the network_flows collector as available
   * (Host.Collectors.network_flows). undefined/true = assume available until
   * the snapshot itself says otherwise; false = short-circuit to the
   * unavailable state without even attempting a fetch. */
  collectorAvailable?: boolean
  /** Whether this tab is the one currently visible — EntityTabShell keeps a
   * lazy tab mounted after first activation, so this gates refreshTick-driven
   * refetches to avoid polling REST endpoints for a tab the user isn't
   * looking at. */
  active?: boolean
  /** Bumped by the host detail page whenever a new report cycle lands for
   * this host (see useHostDetail's metricsUpdatedAt) — triggers a debounced
   * refetch, same pattern as DiskHistoryChart's refreshTick. */
  refreshTick?: number
}>(), {
  initialData: null,
  collectorAvailable: true,
  active: true,
  refreshTick: 0,
})

const i18n = useI18n()
const { t } = i18n
const { talkers, loading, load } = useNetworkFlows(props.hostId, props.initialData)

const protocolFilter = ref<NetworkFlowFilterValue>('')
const search = ref('')

const totalFlows = computed(() => talkers.value.length)

/**
 * Application-level label for a talker: the agent's observed TLS SNI hostname
 * when the optional capture provided one, otherwise a well-known-port guess.
 * Memoized per render pass — the template reads it up to three times per row.
 */
const labelCache = computed(() => {
  const map = new Map<NetworkFlowMetric, ProtocolLabel>()
  for (const t of talkers.value) {
    map.set(t, protocolLabelFor(t.server_name, t.remote_port, t.protocol))
  }
  return map
})

function labelFor(t: NetworkFlowMetric): ProtocolLabel {
  return labelCache.value.get(t) ?? { text: '', source: 'none' }
}

/** Distinct application labels present in the current data, for the filter. */
const serviceOptions = computed(() => {
  const seen = new Set<string>()
  for (const t of talkers.value) {
    if (t.is_others) continue
    const label = labelFor(t)
    if (label.source !== 'none') seen.add(label.text)
  }
  return [...seen].sort((a, b) => a.localeCompare(b, i18n.locale.value))
})

const filteredTalkers = computed(() => {
  const q = search.value.trim().toLowerCase()
  const filter = protocolFilter.value
  return talkers.value.filter((t) => {
    if (filter.startsWith(SERVICE_FILTER_PREFIX)) {
      // The "others" rollup has no remote port, so it can never match a
      // service filter — same reasoning as the search branch below.
      if (t.is_others) return false
      if (labelFor(t).text !== filter.slice(SERVICE_FILTER_PREFIX.length)) return false
    } else if (filter && t.protocol !== filter) {
      return false
    }
    if (!q) return true
    if (t.is_others) return false
    return (t.remote_ip ?? '').toLowerCase().includes(q)
      || (t.process_name ?? '').toLowerCase().includes(q)
      || labelFor(t).text.toLowerCase().includes(q)
  })
})

const sortKey = ref<keyof NetworkFlowMetric>('rx_bytes')
const sortDir = ref<'asc' | 'desc'>('desc')

function toggleSort(key: keyof NetworkFlowMetric): void {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
    return
  }
  sortKey.value = key
  sortDir.value = 'asc'
}

// The synthetic "Autres" aggregate row (is_others) always stays last — it
// summarizes everything past the top-N cutoff, not a real talker to rank
// alongside the rest.
const sortedTalkers = computed(() => {
  const real = filteredTalkers.value.filter((t) => !t.is_others)
  const others = filteredTalkers.value.filter((t) => t.is_others)
  real.sort((a, b) => compareValues(a[sortKey.value], b[sortKey.value], sortDir.value))
  return [...real, ...others]
})

const drilldownTalker = ref<NetworkFlowMetric | null>(null)
const drilldownModalRef = ref<HTMLElement | null>(null)
useModalChrome(drilldownModalRef, () => drilldownTalker.value !== null, { onClose: () => { drilldownTalker.value = null } })

function openDrilldown(t: NetworkFlowMetric): void {
  drilldownTalker.value = t
}

onMounted(async () => {
  if (props.collectorAvailable === false) return
  if (props.initialData) return
  await load()
})

let refreshTimer: ReturnType<typeof setTimeout> | null = null
watch(() => props.refreshTick, () => {
  if (!props.active || props.collectorAvailable === false) return
  if (refreshTimer) clearTimeout(refreshTimer)
  refreshTimer = setTimeout(() => {
    refreshTimer = null
    load()
  }, 400)
})

// Refresh immediately when the tab becomes visible again — the composable
// stays mounted (EntityTabShell keeps lazy tabs alive via v-show), so data
// can otherwise go stale while the user was looking at another tab.
watch(() => props.active, (isActive) => {
  if (isActive && props.collectorAvailable !== false) load()
})
</script>

<style scoped>
/* Signals "hover me for the caveat" on the port-derived guess. */
.heuristic-label {
  cursor: help;
}
</style>
