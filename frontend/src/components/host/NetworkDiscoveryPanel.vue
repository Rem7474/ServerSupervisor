<template>
  <div>
    <div
      class="nav nav-pills nav-sm mb-3 discovery-tools"
      role="tablist"
      :aria-label="t('discovery.toolsLabel')"
    >
      <button
        v-for="item in tools"
        :id="`${uid}-tab-${item.id}`"
        :key="item.id"
        type="button"
        role="tab"
        class="nav-link"
        :class="{ active: tool === item.id }"
        :aria-selected="tool === item.id"
        :aria-controls="`${uid}-panel-${item.id === 'ping' ? 'ping' : 'agent'}`"
        :tabindex="tool === item.id ? 0 : -1"
        @click="tool = item.id"
        @keydown.right.prevent="move(1)"
        @keydown.left.prevent="move(-1)"
      >
        {{ item.label }}
      </button>
    </div>

    <div
      v-if="agentToolsOpened"
      v-show="tool !== 'ping'"
      :id="`${uid}-panel-agent`"
      role="tabpanel"
      :aria-labelledby="`${uid}-tab-${tool}`"
    >
      <AgentDiscoveryPanel :tool="tool === 'ping' ? 'arp' : tool" />
    </div>

    <div
      v-show="tool === 'ping'"
      :id="`${uid}-panel-ping`"
      role="tabpanel"
      :aria-labelledby="`${uid}-tab-ping`"
    >
      <template v-if="!bulkResults">
        <form
          class="mb-3"
          @submit.prevent="scan"
        >
          <label
            class="form-label"
            for="scan-cidr"
          >{{ t('host.discoverySubnetLabel') }}</label>
          <div class="input-group">
            <input
              id="scan-cidr"
              v-model="cidr"
              type="text"
              class="form-control"
              placeholder="192.168.1.0/24"
              required
            >
            <button
              type="submit"
              class="btn btn-primary"
              :disabled="scanning"
            >
              {{ scanning ? t('host.discoveryScanningLabel') : t('host.discoveryScanButton') }}
            </button>
          </div>
          <div class="form-hint">
            {{ t('host.discoveryHint') }}
          </div>
        </form>

        <div
          v-if="scanError"
          class="alert alert-danger"
          role="alert"
        >
          {{ scanError }}
        </div>

        <div
          v-if="scanning"
          class="text-secondary small py-3"
        >
          <span class="spinner-border spinner-border-sm me-2" />
          {{ t('host.discoveryPinging') }}
        </div>

        <template v-else-if="hasScanned">
          <EmptyState
            v-if="!results.length"
            :title="t('host.discoveryNoAddressesTitle')"
            :subtitle="t('host.discoveryNoAddressesSubtitle')"
          />
          <template v-else>
            <div class="d-flex align-items-center justify-content-between mb-2">
              <div class="text-secondary small">
                {{ t('host.discoveryScanSummary', { scanned: t('host.discoveryScannedCount', { count: results.length }, results.length), candidates: t('host.discoveryCandidatesCount', { count: candidates.length }, candidates.length) }) }}
              </div>
              <button
                v-if="candidates.length"
                type="button"
                class="btn btn-sm btn-ghost-secondary"
                @click="toggleSelectAll"
              >
                {{ allSelected ? t('host.discoveryDeselectAll') : t('host.discoverySelectAll') }}
              </button>
            </div>

            <div class="table-responsive mb-3">
              <table class="table table-vcenter card-table table-sm">
                <thead>
                  <tr>
                    <th style="width: 32px" />
                    <th>{{ t('host.discoveryIpColumn') }}</th>
                    <th>{{ t('host.statusColumn') }}</th>
                    <th>{{ t('host.discoveryHostnameColumn') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="r in results"
                    :key="r.ip_address"
                  >
                    <td>
                      <input
                        v-if="r.responded && !r.already_registered"
                        type="checkbox"
                        class="form-check-input"
                        :checked="selected.has(r.ip_address)"
                        :aria-label="t('host.discoverySelectAriaLabel', { ip: r.ip_address })"
                        @change="toggleSelected(r.ip_address)"
                      >
                    </td>
                    <td class="font-monospace">
                      {{ r.ip_address }}
                    </td>
                    <td>
                      <span
                        v-if="r.already_registered"
                        class="badge bg-azure-lt"
                      >
                        {{ r.existing_host_name ? t('host.discoveryAlreadyRegisteredWithName', { name: r.existing_host_name }) : t('host.discoveryAlreadyRegistered') }}
                      </span>
                      <span
                        v-else-if="r.responded"
                        class="badge bg-success-lt"
                      >
                        {{ r.latency_ms ? t('host.discoveryRespondsWithLatency', { ms: r.latency_ms }) : t('host.discoveryResponds') }}
                      </span>
                      <span
                        v-else
                        class="badge bg-secondary-lt"
                      >
                        {{ t('host.discoveryNoResponse') }}
                      </span>
                    </td>
                    <td>
                      <input
                        v-if="r.responded && !r.already_registered"
                        v-model="names[r.ip_address]"
                        type="text"
                        class="form-control form-control-sm"
                        :aria-label="t('host.discoveryHostnameAriaLabel', { ip: r.ip_address })"
                      >
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <div
              v-if="addError"
              class="alert alert-danger"
              role="alert"
            >
              {{ addError }}
            </div>

            <button
              v-if="candidates.length"
              type="button"
              class="btn btn-primary"
              :disabled="!selected.size || adding"
              @click="addSelected"
            >
              {{ adding ? t('host.discoveryAdding') : t('host.discoveryAddButton', { count: selected.size }, selected.size) }}
            </button>
          </template>
        </template>
      </template>

      <div
        v-else
        class="host-success"
        role="alert"
      >
        <div class="host-success-header">
          <div class="fw-semibold">
            {{ t('host.discoveryHostsAddedCount', { count: createdCount }, createdCount) }}
          </div>
          <button
            type="button"
            class="btn btn-success"
            @click="finish"
          >
            {{ t('host.discoveryDone') }}
          </button>
        </div>
        <div class="text-secondary small mb-3">
          {{ t('host.discoveryKeysWarning') }}
        </div>

        <div class="d-flex flex-column gap-2">
          <div
            v-for="item in bulkResults"
            :key="item.ip_address"
            class="host-success-card"
          >
            <template v-if="item.created">
              <div class="d-flex align-items-center justify-content-between mb-2">
                <div>
                  <span class="fw-medium">{{ item.name }}</span>
                  <span class="text-secondary small ms-2 font-monospace">{{ item.ip_address }}</span>
                </div>
              </div>
              <div class="host-success-key">
                <code>{{ item.api_key }}</code>
                <button
                  type="button"
                  class="btn btn-outline-secondary btn-sm"
                  @click="copyKey(item)"
                >
                  {{ copiedKeys[item.ip_address] ? t('host.discoveryCopied') : t('host.discoveryCopy') }}
                </button>
              </div>
            </template>
            <div
              v-else
              class="text-danger small"
            >
              <span class="font-monospace">{{ item.ip_address }}</span> — {{ item.error }}
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import EmptyState from '../EmptyState.vue'
import AgentDiscoveryPanel from './AgentDiscoveryPanel.vue'
import type { AgentDiscoveryKind } from '../../types/discovery'
import { useNetworkDiscovery } from '../../composables/useNetworkDiscovery'

const { t } = useI18n()
const emit = defineEmits<{ done: [] }>()

const {
  cidr,
  scanning,
  scanError,
  results,
  hasScanned,
  candidates,
  names,
  selected,
  allSelected,
  scan,
  toggleSelected,
  toggleSelectAll,
  adding,
  addError,
  bulkResults,
  addSelected,
} = useNetworkDiscovery()

// Discovery tools: the server-side ping sweep, then the agent-run probes.
// The agent panel mounts on first use and stays mounted, so a running scan
// survives switching tabs.
type Tool = 'ping' | AgentDiscoveryKind
const uid = useId()
const tool = ref<Tool>('ping')
const agentToolsOpened = ref(false)
watch(tool, (v) => {
  if (v !== 'ping') agentToolsOpened.value = true
})
const tools = computed<{ id: Tool; label: string }[]>(() => [
  { id: 'ping', label: t('discovery.toolPing') },
  { id: 'arp', label: t('discovery.toolArp') },
  { id: 'port_scan', label: t('discovery.toolPorts') },
  { id: 'snmp', label: t('discovery.toolSnmp') },
])

function move(step: number): void {
  const list = tools.value
  const i = list.findIndex((x) => x.id === tool.value)
  tool.value = list[(i + step + list.length) % list.length].id
  document.getElementById(`${uid}-tab-${tool.value}`)?.focus()
}

const createdCount = computed(() => bulkResults.value?.filter((r) => r.created).length ?? 0)

const copiedKeys = reactive<Record<string, boolean>>({})

async function copyKey(item: { ip_address: string; api_key?: string }): Promise<void> {
  if (!item.api_key) return
  await navigator.clipboard.writeText(item.api_key)
  copiedKeys[item.ip_address] = true
  setTimeout(() => {
    copiedKeys[item.ip_address] = false
  }, 1500)
}

function finish(): void {
  emit('done')
}
</script>

<style scoped>
/* Tighter than the shared .host-success-header (style.css): the panel sits in a list. */
.host-success-header {
  margin-bottom: 8px;
}
</style>
