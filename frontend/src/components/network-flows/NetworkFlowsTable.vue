<template>
  <div>
    <div
      v-if="collectorAvailable === false"
      class="card"
    >
      <div class="card-body">
        <EmptyState
          :icon="IconWifiOff"
          title="Trafic réseau indisponible sur cet hôte"
          subtitle="Voir le bandeau de diagnostics de l'hôte pour la raison exacte (module conntrack absent ou comptage désactivé)."
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
            Top talkers
            <span
              v-if="totalFlows > talkers.length"
              class="text-muted small ms-1"
            >({{ talkers.length }} affichés sur {{ totalFlows }} flux actifs)</span>
          </h3>
          <div class="d-flex align-items-center gap-2">
            <select
              v-model="protocolFilter"
              class="form-select form-select-sm"
              style="width: auto;"
            >
              <option value="">
                Tous protocoles
              </option>
              <option value="tcp">
                TCP
              </option>
              <option value="udp">
                UDP
              </option>
            </select>
            <input
              v-model="search"
              type="text"
              class="form-control form-control-sm"
              style="width: auto;"
              placeholder="IP ou processus…"
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
            title="Aucun flux réseau observé"
            subtitle="Aucune connexion active n'a été trackée lors du dernier cycle de rapport."
          />
        </div>
        <div
          v-else
          class="table-responsive"
        >
          <table class="table table-vcenter card-table mb-0">
            <thead>
              <tr>
                <th>Processus</th>
                <th>IP distante</th>
                <th>Port</th>
                <th>Protocole</th>
                <th>Direction</th>
                <th class="text-end">
                  Rx
                </th>
                <th class="text-end">
                  Tx
                </th>
                <th class="text-end">
                  Connexions
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="t in filteredTalkers"
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
                    Autres ({{ t.connections }} connexion{{ t.connections > 1 ? 's' : '' }})
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
                    >inconnu</span>
                  </td>
                  <td class="fw-medium">
                    {{ t.remote_ip }}
                  </td>
                  <td>{{ t.remote_port }}</td>
                  <td>
                    <span class="badge bg-secondary-lt text-secondary text-uppercase">{{ t.protocol }}</span>
                  </td>
                  <td>
                    <span
                      v-if="t.direction === 'inbound'"
                      class="badge bg-azure-lt text-azure"
                    >Entrant</span>
                    <span
                      v-else
                      class="badge bg-teal-lt text-teal"
                    >Sortant</span>
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
              aria-label="Fermer"
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
import { IconNetwork, IconWifiOff } from '@tabler/icons-vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import EmptyState from '../EmptyState.vue'
import NetworkFlowsHistoryChart from './NetworkFlowsHistoryChart.vue'
import { useNetworkFlows } from '../../composables/useNetworkFlows'
import { useModalChrome } from '../../composables/useModalChrome'
import { formatBytes } from '../../utils/formatters'
import type { NetworkFlowMetric } from '../../types/networkFlows'

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

const { talkers, loading, load } = useNetworkFlows(props.hostId, props.initialData)

const protocolFilter = ref('')
const search = ref('')

const totalFlows = computed(() => talkers.value.length)

const filteredTalkers = computed(() => {
  const q = search.value.trim().toLowerCase()
  return talkers.value.filter((t) => {
    if (protocolFilter.value && t.protocol !== protocolFilter.value) return false
    if (!q) return true
    if (t.is_others) return false
    return (t.remote_ip ?? '').toLowerCase().includes(q) || (t.process_name ?? '').toLowerCase().includes(q)
  })
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
