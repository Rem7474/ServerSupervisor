<template>
  <div>
    <div
      v-if="!hostsLoaded"
      class="text-secondary small py-3"
    >
      <span class="spinner-border spinner-border-sm me-2" />
      {{ t('discovery.loadingAgents') }}
    </div>

    <div
      v-else-if="loadError"
      class="alert alert-danger"
      role="alert"
    >
      {{ loadError }}
    </div>

    <EmptyState
      v-else-if="!agents.length"
      :title="t('discovery.noAgentTitle')"
      :subtitle="t('discovery.noAgentSubtitle')"
    />

    <template v-else>
      <div class="mb-3">
        <label
          class="form-label"
          :for="ids.agent"
        >{{ t('discovery.agentLabel') }}</label>
        <select
          :id="ids.agent"
          v-model="hostId"
          class="form-select"
          :disabled="isActive(tool)"
        >
          <option
            v-for="a in agents"
            :key="a.id"
            :value="a.id"
          >
            {{ a.name }} ({{ a.ip_address }}){{ a.collectors?.network_scan ? '' : ` — ${t('discovery.passiveOnly')}` }}
          </option>
        </select>
        <div class="form-hint">
          {{ t('discovery.agentHint') }}
        </div>
      </div>

      <div
        v-if="tool !== 'arp' && !scanAllowed"
        class="alert alert-info"
        role="status"
      >
        <div class="fw-semibold mb-1">
          {{ t('discovery.scanDisabledTitle') }}
        </div>
        <I18nT
          keypath="discovery.scanDisabledBody"
          tag="div"
        >
          <template #option>
            <code>allow_network_scan: true</code>
          </template>
        </I18nT>
      </div>

      <!-- ARP -->
      <div v-if="tool === 'arp'">
        <p class="text-secondary small">
          {{ t('discovery.arpIntro') }}
        </p>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="starting.arp || isActive('arp')"
          @click="startArp"
        >
          {{ t('discovery.arpButton') }}
        </button>
      </div>

      <!-- Port scan -->
      <form
        v-else-if="tool === 'port_scan'"
        novalidate
        @submit.prevent="confirmPortScan"
      >
        <p class="text-secondary small">
          {{ t('discovery.scanIntro') }}
        </p>
        <div class="row g-2">
          <div class="col-md-5">
            <label
              class="form-label"
              :for="ids.cidr"
            >{{ t('discovery.cidrLabel') }}</label>
            <input
              :id="ids.cidr"
              v-model="cidr"
              type="text"
              class="form-control font-monospace"
              placeholder="192.168.1.0/24"
              required
              :disabled="!scanAllowed || isActive('port_scan')"
              :aria-describedby="ids.scanHint"
            >
          </div>
          <div class="col-md-7">
            <label
              class="form-label"
              :for="ids.ports"
            >{{ t('discovery.portsLabel') }}</label>
            <input
              :id="ids.ports"
              v-model="ports"
              type="text"
              class="form-control font-monospace"
              :placeholder="t('discovery.portsPlaceholder', { count: limits?.default_ports.length ?? 0 })"
              :disabled="!scanAllowed || isActive('port_scan')"
              :aria-describedby="ids.scanHint"
            >
          </div>
        </div>
        <div
          :id="ids.scanHint"
          class="form-hint mb-2"
        >
          {{ t('discovery.scanHint', { min: limits?.min_prefix_len ?? 24, max: limits?.max_ports ?? 1024 }) }}
          <span v-if="limits?.allowed_subnets.length">
            {{ t('discovery.allowedSubnets', { subnets: limits.allowed_subnets.join(', ') }) }}
          </span>
        </div>
        <div
          v-if="portScanProblem"
          class="text-danger small mb-2"
          role="alert"
        >
          {{ portScanProblem }}
        </div>
        <button
          type="submit"
          class="btn btn-primary"
          :disabled="!scanAllowed || !cidr.trim() || !!portScanProblem || starting.port_scan || isActive('port_scan')"
        >
          {{ t('discovery.scanButton') }}
        </button>
      </form>

      <!-- SNMP -->
      <form
        v-else
        novalidate
        autocomplete="off"
        @submit.prevent="startSnmp"
      >
        <p class="text-secondary small">
          {{ t('discovery.snmpIntro') }}
        </p>
        <fieldset :disabled="!scanAllowed || isActive('snmp')">
          <div class="row g-2 mb-2">
            <div class="col-md-5">
              <label
                class="form-label"
                :for="ids.target"
              >{{ t('discovery.snmpTargetLabel') }}</label>
              <input
                :id="ids.target"
                v-model="snmp.target"
                type="text"
                class="form-control font-monospace"
                placeholder="192.168.1.10"
                required
              >
            </div>
            <div class="col-md-3">
              <label
                class="form-label"
                :for="ids.port"
              >{{ t('discovery.snmpPortLabel') }}</label>
              <input
                :id="ids.port"
                v-model.number="snmp.port"
                type="number"
                min="1"
                max="65535"
                class="form-control"
              >
            </div>
            <div class="col-md-4">
              <label
                class="form-label"
                :for="ids.version"
              >{{ t('discovery.snmpVersionLabel') }}</label>
              <select
                :id="ids.version"
                v-model="snmp.version"
                class="form-select"
              >
                <option value="2c">
                  v2c
                </option>
                <option value="3">
                  v3
                </option>
              </select>
            </div>
          </div>

          <div
            v-if="snmp.version === '2c'"
            class="mb-2"
          >
            <label
              class="form-label"
              :for="ids.community"
            >{{ t('discovery.snmpCommunityLabel') }}</label>
            <input
              :id="ids.community"
              v-model="snmp.community"
              type="password"
              class="form-control"
              autocomplete="off"
              required
              :aria-describedby="ids.secretHint"
            >
          </div>

          <div
            v-else
            class="row g-2 mb-2"
          >
            <div class="col-md-6">
              <label
                class="form-label"
                :for="ids.username"
              >{{ t('discovery.snmpUsernameLabel') }}</label>
              <input
                :id="ids.username"
                v-model="snmp.username"
                type="text"
                class="form-control"
                autocomplete="off"
                required
              >
            </div>
            <div class="col-md-6">
              <label
                class="form-label"
                :for="ids.level"
              >{{ t('discovery.snmpLevelLabel') }}</label>
              <select
                :id="ids.level"
                v-model="snmp.securityLevel"
                class="form-select"
              >
                <option value="authPriv">
                  authPriv
                </option>
                <option value="authNoPriv">
                  authNoPriv
                </option>
                <option value="noAuthNoPriv">
                  noAuthNoPriv
                </option>
              </select>
            </div>
            <template v-if="snmp.securityLevel !== 'noAuthNoPriv'">
              <div class="col-md-6">
                <label
                  class="form-label"
                  :for="ids.authProto"
                >{{ t('discovery.snmpAuthProtocolLabel') }}</label>
                <select
                  :id="ids.authProto"
                  v-model="snmp.authProtocol"
                  class="form-select"
                >
                  <option value="SHA256">
                    SHA-256
                  </option>
                  <option value="SHA">
                    SHA-1
                  </option>
                </select>
              </div>
              <div class="col-md-6">
                <label
                  class="form-label"
                  :for="ids.authPass"
                >{{ t('discovery.snmpAuthPassphraseLabel') }}</label>
                <input
                  :id="ids.authPass"
                  v-model="snmp.authPassphrase"
                  type="password"
                  class="form-control"
                  autocomplete="new-password"
                  minlength="8"
                  required
                  :aria-describedby="ids.secretHint"
                >
              </div>
            </template>
            <div
              v-if="snmp.securityLevel === 'authPriv'"
              class="col-md-6"
            >
              <label
                class="form-label"
                :for="ids.privProto"
              >{{ t('discovery.snmpPrivProtocolLabel') }}</label>
              <select
                :id="ids.privProto"
                class="form-select"
                disabled
              >
                <option>AES-128</option>
              </select>
            </div>
            <div
              v-if="snmp.securityLevel === 'authPriv'"
              class="col-md-6"
            >
              <label
                class="form-label"
                :for="ids.privPass"
              >{{ t('discovery.snmpPrivPassphraseLabel') }}</label>
              <input
                :id="ids.privPass"
                v-model="snmp.privPassphrase"
                type="password"
                class="form-control"
                autocomplete="new-password"
                minlength="8"
                required
                :aria-describedby="ids.secretHint"
              >
            </div>
          </div>

          <div
            :id="ids.secretHint"
            class="form-hint mb-2"
          >
            {{ t('discovery.snmpSecretHint') }}
          </div>

          <label class="form-check mb-3">
            <input
              v-model="snmp.includeInterfaces"
              type="checkbox"
              class="form-check-input"
            >
            <span class="form-check-label">{{ t('discovery.snmpInterfacesLabel') }}</span>
          </label>

          <button
            type="submit"
            class="btn btn-primary"
            :disabled="!snmp.target.trim() || starting.snmp"
          >
            {{ t('discovery.snmpButton') }}
          </button>
        </fieldset>
      </form>

      <div
        v-if="errors[tool]"
        class="alert alert-danger mt-3"
        role="alert"
      >
        {{ errors[tool] }}
      </div>

      <!-- Job state -->
      <div
        v-if="job"
        class="mt-3"
      >
        <div
          v-if="isActive(tool)"
          class="d-flex align-items-center gap-3 mb-2"
          role="status"
          aria-live="polite"
        >
          <div class="flex-grow-1">
            <div class="small text-secondary mb-1">
              <span class="spinner-border spinner-border-sm me-2" />
              {{ statusLabel }}
            </div>
            <div
              v-if="tool === 'port_scan' && progress && progress.total"
              class="progress progress-sm"
              role="progressbar"
              :aria-label="t('discovery.progressLabel')"
              :aria-valuenow="progress.done"
              aria-valuemin="0"
              :aria-valuemax="progress.total"
            >
              <div
                class="progress-bar"
                :style="{ width: `${Math.round((progress.done / progress.total) * 100)}%` }"
              />
            </div>
          </div>
          <button
            type="button"
            class="btn btn-outline-danger btn-sm"
            :disabled="cancelling[tool]"
            @click="cancel(tool)"
          >
            {{ t('discovery.cancelButton') }}
          </button>
        </div>
        <DiscoveryJobResult
          v-else
          :job="job"
        />
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, useId } from 'vue'
import { I18nT, useI18n } from 'vue-i18n'
import EmptyState from '../EmptyState.vue'
import DiscoveryJobResult from './DiscoveryJobResult.vue'
import { useAgentDiscovery } from '../../composables/useAgentDiscovery'
import { useConfirmDialog } from '../../composables/useConfirmDialog'
import type { AgentDiscoveryKind } from '../../types/discovery'

const props = defineProps<{ tool: AgentDiscoveryKind }>()
const { t } = useI18n()
const { confirm } = useConfirmDialog()

const uid = useId()
const ids = {
  agent: `${uid}-agent`,
  cidr: `${uid}-cidr`,
  ports: `${uid}-ports`,
  scanHint: `${uid}-scan-hint`,
  target: `${uid}-target`,
  port: `${uid}-port`,
  version: `${uid}-version`,
  community: `${uid}-community`,
  username: `${uid}-username`,
  level: `${uid}-level`,
  authProto: `${uid}-auth-proto`,
  authPass: `${uid}-auth-pass`,
  privProto: `${uid}-priv-proto`,
  privPass: `${uid}-priv-pass`,
  secretHint: `${uid}-secret-hint`,
}

const {
  hostsLoaded,
  loadError,
  limits,
  hostId,
  agents,
  selectedHost,
  scanAllowed,
  jobs,
  errors,
  starting,
  cancelling,
  progress,
  cidr,
  ports,
  snmp,
  addressCount,
  effectivePortCount,
  portScanProblem,
  load,
  isActive,
  startArp,
  startPortScan,
  startSnmp,
  cancel,
  dispose,
} = useAgentDiscovery()

const job = computed(() => jobs[props.tool])

const statusLabel = computed(() => {
  if (job.value?.status === 'pending') return t('discovery.statusPending')
  if (props.tool === 'port_scan' && progress.value?.total) {
    return t('discovery.statusScanning', { done: progress.value.done, total: progress.value.total, open: progress.value.open })
  }
  return t('discovery.statusRunning')
})

async function confirmPortScan(): Promise<void> {
  if (portScanProblem.value || !cidr.value.trim()) return
  const ok = await confirm({
    title: t('discovery.confirmTitle'),
    message: t('discovery.confirmMessage', {
      addresses: addressCount.value ?? 0,
      ports: effectivePortCount.value,
      cidr: cidr.value.trim(),
      agent: selectedHost.value?.name ?? '',
    }),
    okLabel: t('discovery.confirmOk'),
    cancelLabel: t('common.cancel'),
  })
  if (ok) await startPortScan()
}

onMounted(load)
onUnmounted(dispose)
</script>
