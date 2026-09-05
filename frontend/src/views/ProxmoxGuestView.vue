<template>
  <div>
    <PageRefreshBar
      v-model="autoRefresh"
      :label="t('proxmox.guestRefreshLabel')"
      :interval-sec="GUEST_REFRESH_SEC"
      :last-updated-at="lastUpdatedAt"
    />
    <div v-if="loading">
      <LoadingSkeleton
        variant="kpi"
        :lines="4"
      />
      <LoadingSkeleton
        variant="card"
        :lines="4"
        class="mt-4"
      />
    </div>
    <div
      v-else-if="error"
      class="alert alert-danger"
    >
      {{ error }}
    </div>
    <div v-else-if="guest">
      <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
        <div>
          <div class="page-pretitle">
            <router-link
              to="/"
              class="text-decoration-none"
            >
              {{ t('nav.sections.control.items.dashboard') }}
            </router-link>
            <span class="text-muted mx-1">/</span>
            <router-link
              to="/proxmox"
              class="text-decoration-none"
            >
              {{ t('proxmox.breadcrumbTitle') }}
            </router-link>
            <span class="text-muted mx-1">/</span>
            <router-link
              :to="`/proxmox/nodes/${guestNodeId}`"
              class="text-decoration-none"
            >
              {{ guest.node_name }}
            </router-link>
            <span class="text-muted mx-1">/</span>
            <span>{{ guest.guest_type.toUpperCase() }} {{ guest.vmid }}</span>
          </div>
          <div class="d-flex align-items-center gap-3 flex-wrap">
            <h2 class="page-title mb-0">
              {{ guest.name || `${guest.guest_type.toUpperCase()} ${guest.vmid}` }}
            </h2>
            <span :class="getEntityStateClass(guest.status)">{{ getEntityStateLabel(guest.status) }}</span>
            <span class="badge bg-azure-lt text-azure">{{ guest.guest_type.toUpperCase() }}</span>
            <GuestLinkCell
              v-if="guestLink && guestLink.status !== 'ignored'"
              :link="guestLink"
              @confirm="confirmLink"
              @ignore="ignoreLink"
              @go="goToHostLink"
            />
            <span
              v-if="linkMsg"
              :class="['small', linkMsgOk ? 'text-success' : 'text-danger']"
            >{{ linkMsg }}</span>
          </div>
          <div class="text-secondary">
            {{ t('proxmox.nodeUptimeSummary', { node: guest.node_name, vmid: guest.vmid, uptime: formatUptime(guest.uptime) }) }}
          </div>
        </div>
        <div
          v-if="auth.isAdmin"
          class="d-flex gap-2"
        >
          <span
            v-if="guest.guest_type === 'lxc' && consoleConfigured === false"
            class="d-inline-flex align-items-center"
            :title="t('proxmox.consoleUnconfiguredAlertTitle')"
          >
            <IconAlertTriangle
              :size="16"
              class="icon text-warning"
            />
          </span>
          <button
            type="button"
            class="btn btn-sm btn-outline-secondary"
            :disabled="guest.guest_type !== 'lxc'"
            :title="consoleButtonTitle"
            @click="openConsole"
          >
            <IconTerminal2
              :size="16"
              class="icon me-1"
            />
            {{ t('proxmox.consoleButton') }}
          </button>
          <button
            v-if="guest.status === 'stopped'"
            type="button"
            class="btn btn-sm btn-outline-success"
            :disabled="actionLoading !== null"
            @click="performGuestAction('start')"
          >
            <span
              v-if="actionLoading === 'start'"
              class="spinner-border spinner-border-sm me-1"
            />
            <IconPlayerPlay
              v-else
              :size="16"
              class="icon me-1"
            />
            {{ t('proxmox.startButton') }}
          </button>
          <template v-else>
            <button
              type="button"
              class="btn btn-sm btn-outline-warning"
              :disabled="actionLoading !== null"
              @click="performGuestAction('reboot')"
            >
              <span
                v-if="actionLoading === 'reboot'"
                class="spinner-border spinner-border-sm me-1"
              />
              <IconRefresh
                v-else
                :size="16"
                class="icon me-1"
              />
              {{ t('proxmox.restartButton') }}
            </button>
            <button
              type="button"
              class="btn btn-sm btn-outline-danger"
              :disabled="actionLoading !== null"
              @click="performGuestAction('shutdown')"
            >
              <span
                v-if="actionLoading === 'shutdown'"
                class="spinner-border spinner-border-sm me-1"
              />
              <IconPlayerStop
                v-else
                :size="16"
                class="icon me-1"
              />
              {{ t('proxmox.stopButton') }}
            </button>
          </template>
        </div>
      </div>

      <div class="side-layout">
        <div class="side-main">
          <div class="row row-cards mb-4">
            <div class="col-6 col-lg-3">
              <div class="card card-sm h-100">
                <div class="card-body">
                  <div class="subheader">
                    {{ t('proxmox.cpuColumn') }}
                  </div>
                  <div
                    class="h2 mt-2 mb-0"
                    :class="getMetricColorClass(guest.cpu_usage * 100)"
                  >
                    {{ (guest.cpu_usage * 100).toFixed(1) }}%
                  </div>
                  <div class="text-secondary small">
                    {{ t('proxmox.cpuAllocLabel', { n: guest.cpu_alloc }) }}
                  </div>
                </div>
              </div>
            </div>
            <div class="col-6 col-lg-3">
              <div class="card card-sm h-100">
                <div class="card-body">
                  <div class="subheader">
                    {{ t('proxmox.ramColumn') }}
                  </div>
                  <div
                    class="h2 mt-2 mb-0"
                    :class="ramPct != null ? getMetricColorClass(ramPct) : 'text-secondary'"
                  >
                    {{ ramPct != null ? ramPct.toFixed(1) + '%' : '—' }}
                  </div>
                  <div class="text-secondary small">
                    {{ formatBytes(guest.mem_usage) }} / {{ formatBytes(guest.mem_alloc) }}
                  </div>
                </div>
              </div>
            </div>
            <div class="col-6 col-lg-3">
              <div class="card card-sm h-100">
                <div class="card-body">
                  <div class="subheader">
                    {{ t('proxmox.diskColumn') }}
                  </div>
                  <div
                    class="h2 mt-2 mb-0"
                    :class="diskPct != null ? getMetricColorClass(diskPct) : 'text-secondary'"
                  >
                    {{ diskPct != null ? diskPct.toFixed(1) + '%' : '—' }}
                  </div>
                  <div class="text-secondary small">
                    {{ formatBytes(guest.disk_usage) }} / {{ formatBytes(guest.disk_alloc) }}
                  </div>
                </div>
              </div>
            </div>
            <div class="col-6 col-lg-3">
              <div class="card card-sm h-100">
                <div class="card-body">
                  <div class="subheader">
                    {{ t('proxmox.ipColumn') }}
                  </div>
                  <div class="h2 mt-2 mb-0">
                    {{ guestPrimaryIp || '—' }}
                  </div>
                  <a
                    href="#"
                    class="text-decoration-none small"
                    @click.prevent="showNetworkDetail = !showNetworkDetail"
                  >{{ showNetworkDetail ? t('proxmox.hideNetworkDetailLink') : t('proxmox.showNetworkDetailLink') }}</a>
                </div>
              </div>
            </div>
          </div>

          <div
            v-if="showNetworkDetail"
            class="card mb-4"
          >
            <div class="card-header">
              <h3 class="card-title mb-0">
                {{ t('proxmox.networkInterfacesTitle') }}
              </h3>
            </div>
            <div class="card-body">
              <span
                v-if="guestNetworksLoading"
                class="text-muted small"
              >{{ t('proxmox.loadingLabel') }}</span>
              <EmptyState
                v-else-if="guestNetworks.length === 0"
                :title="t('proxmox.noNetworkInterfacesTitle')"
              />
              <div
                v-else
                class="d-flex flex-column gap-2"
              >
                <div
                  v-for="iface in guestNetworks"
                  :key="iface.name"
                  class="d-flex align-items-center gap-2"
                >
                  <span class="badge bg-secondary-lt text-secondary">{{ iface.name }}</span>
                  <span
                    v-for="ip in iface.ips"
                    :key="ip"
                    class="small font-monospace"
                  >{{ ip }}</span>
                </div>
              </div>
            </div>
          </div>

          <div
            v-if="guestLink?.status === 'confirmed' && guestLink?.host_id"
            class="card mb-4"
          >
            <div class="card-body d-flex align-items-center justify-content-between gap-2 flex-wrap">
              <div>
                <div class="fw-medium">
                  {{ t('proxmox.domainsExposureTitle') }}
                </div>
                <div class="text-secondary small">
                  {{ t('proxmox.guestExposureLinkedHint', { host: guestLink.host_hostname || guestLink.host_name }) }}
                </div>
              </div>
              <router-link
                :to="`/hosts/${guestLink.host_id}?tab=exposition`"
                class="btn btn-sm btn-outline-primary"
              >
                {{ t('proxmox.viewDomainIpCorrelationButton') }}
              </router-link>
            </div>
          </div>
          <GuestExposureCard
            v-else
            :guest-id="guest.id"
            class="mb-4"
          />

          <div class="card">
            <div class="card-header d-flex align-items-center justify-content-between gap-2 flex-wrap">
              <h3 class="card-title mb-0">
                {{ t('proxmox.cpuRamHistoryTitle') }}
              </h3>
              <div
                class="btn-group btn-group-sm guest-range-group"
                role="group"
                :aria-label="t('proxmox.timeRangeAriaLabel')"
              >
                <button
                  v-for="h in [1, 6, 24, 168, 720]"
                  :key="h"
                  type="button"
                  :class="hours === h ? 'btn btn-primary' : 'btn btn-outline-secondary'"
                  @click="changeRange(h)"
                >
                  {{ h >= 24 ? (h / 24) + t('proxmox.daySuffix') : h + 'h' }}
                </button>
              </div>
            </div>
            <div
              class="card-body"
              style="height: 14rem;"
            >
              <LoadingSkeleton
                v-if="summaryLoading"
                variant="chart"
              />
              <ApexChart
                v-else-if="series && chartOptions"
                type="area"
                height="100%"
                :options="chartOptions"
                :series="series"
              />
              <div
                v-else
                class="h-100 d-flex align-items-center justify-content-center text-secondary"
              >
                {{ t('proxmox.noChartDataText') }}
              </div>
            </div>
          </div>
        </div> <!-- /side-main -->
        <ProxmoxConsoleLazy
          v-if="hasOpenedConsole"
          :guest-id="guest.id"
          :guest-name="guest.name || `${guest.guest_type.toUpperCase()} ${guest.vmid}`"
          :show="showConsole"
          @close="showConsole = false"
          @open="showConsole = true"
        />
      </div> <!-- /side-layout -->
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { IconAlertTriangle, IconPlayerPlay, IconPlayerStop, IconRefresh, IconTerminal2 } from '@tabler/icons-vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import PageRefreshBar from '../components/PageRefreshBar.vue'
import EmptyState from '../components/EmptyState.vue'
import GuestExposureCard from '../components/proxmox/GuestExposureCard.vue'
import GuestLinkCell from '../components/proxmox/GuestLinkCell.vue'
import { useAuthStore } from '../stores/auth'
import { useProxmoxGuest } from '../composables/useProxmoxGuest'
import { getEntityStateClass, getEntityStateLabel } from '../utils/statusClasses'
import { getMetricColorClass } from '../utils/metricColor'
import { AsyncApexChart as ApexChart } from '../utils/apexChartTheme'

const router = useRouter()
const auth = useAuthStore()
const { t } = useI18n()

const {
  guest,
  guestLink,
  loading,
  summaryLoading,
  error,
  hours,
  series,
  chartOptions,
  autoRefresh,
  lastUpdatedAt,
  GUEST_REFRESH_SEC,
  changeRange,
  actionLoading,
  performGuestAction,
  guestNetworks,
  guestNetworksLoading,
  nodeId: guestNodeId,
  linkMsg,
  linkMsgOk,
  confirmLink,
  ignoreLink,
  consoleConfigured,
  consoleButtonTitle,
} = useProxmoxGuest()

const showNetworkDetail = ref(false)
const showConsole = ref(false)
// Mounted once the user first opens a console (see openConsole), then stays
// mounted so ProxmoxConsole.vue's own v-show-based hide/reopen keeps the
// session alive in the background instead of tearing it down on every
// close — same pattern CommandLogPanel already uses elsewhere.
const hasOpenedConsole = ref(false)
// Lazy: only fetched once the user actually opens a console, not on every
// guest page visit — xterm.js is otherwise dead weight for the common case.
const ProxmoxConsoleLazy = defineAsyncComponent(() => import('../components/proxmox/ProxmoxConsole.vue'))

function openConsole(): void {
  hasOpenedConsole.value = true
  showConsole.value = true
}

// Guests linked to a ServerSupervisor host already get their domain/IP
// correlation for free from that host's own Exposition tab (same IP, same
// GetHostExposure query) — land there directly instead of the guest's own
// page, mirroring useProxmoxNode's goToHost.
function goToHostLink(): void {
  if (guestLink.value?.host_id) router.push(`/hosts/${guestLink.value.host_id}?tab=exposition`)
}

const ramPct = computed(() => {
  if (!guest.value?.mem_alloc) return null
  return (guest.value.mem_usage / guest.value.mem_alloc) * 100
})

const diskPct = computed(() => {
  if (!guest.value?.disk_alloc) return null
  return (guest.value.disk_usage / guest.value.disk_alloc) * 100
})

// Only ethX interfaces feed the KPI card — same convention as
// ProxmoxNodeGuestsTab's IP column; the collapsible section below shows
// every interface regardless of name.
const guestPrimaryIp = computed(() => {
  const ethIfaces = guestNetworks.value.filter((iface) => /^eth\d+$/i.test(iface?.name ?? ''))
  for (const iface of ethIfaces) {
    const ips = Array.isArray(iface?.ips) ? iface.ips : []
    const first = ips.find((ip) => typeof ip === 'string' && !ip.startsWith('fe80'))
    if (first) return first.split('/')[0]
  }
  return ''
})


function formatBytes(bytes: number): string {
  if (!bytes) return '0 B'
  const unitKeys = ['proxmox.byteUnitKilo', 'proxmox.byteUnitMega', 'proxmox.byteUnitGiga', 'proxmox.byteUnitTera']
  let i = 0
  let v = bytes
  while (v >= 1024 && i < unitKeys.length) {
    v /= 1024
    i++
  }
  const unit = i === 0 ? 'B' : t(unitKeys[i - 1])
  return `${v.toFixed(i === 0 ? 0 : 1)} ${unit}`
}

function formatUptime(seconds: number): string {
  if (!seconds) return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}${t('proxmox.daySuffix')} ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

</script>

<style scoped>
.guest-range-group {
  flex-wrap: wrap;
}

@media (max-width: 768px) {
  .guest-range-group {
    width: 100%;
  }

  .guest-range-group .btn {
    flex: 1 1 0;
    min-width: 56px;
  }
}
</style>

