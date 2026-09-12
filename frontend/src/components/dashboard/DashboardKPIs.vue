<template>
  <div class="row row-cards mb-4">
    <div class="col-6 col-lg-3">
      <div class="card card-sm h-100">
        <div class="card-body">
          <div class="subheader">
            {{ t('dashboard.hostsLabel') }}
          </div>
          <div class="h1 mb-0">
            {{ hosts.length }}
          </div>
          <div class="text-secondary small mt-1">
            <span class="text-success me-2">{{ t('dashboard.onlineCount', { count: onlineCount }) }}</span>
            <span
              v-if="offlineCount > 0"
              class="text-danger"
            >{{ t('dashboard.offlineCount', { count: offlineCount }) }}</span>
          </div>
        </div>
      </div>
    </div>
    <div class="col-6 col-lg-3">
      <div class="card card-sm h-100">
        <div class="card-body">
          <div class="subheader">
            {{ t('dashboard.updatesLabel') }}
          </div>
          <div
            class="h1 mb-0"
            :class="outdatedVersions > 0 ? 'text-warning' : 'text-success'"
          >
            {{ outdatedVersions }}
          </div>
          <div class="text-secondary small mt-1">
            <span
              v-if="aptPending > 0"
              class="me-2"
            >{{ t('dashboard.aptPendingPackages', aptPending) }}</span>
            <span v-if="outdatedDockerImages > 0">{{ t('dashboard.outdatedDockerImages', outdatedDockerImages) }}</span>
            <span v-if="outdatedVersions === 0">{{ t('dashboard.allUpToDate') }}</span>
          </div>
          <div
            v-if="cveSummary && ((cveSummary.critical_count || 0) > 0 || (cveSummary.hosts_with_critical || 0) > 0 || (cveSummary.high_count || 0) > 0 || (cveSummary.hosts_with_high || 0) > 0)"
            class="small mt-1 text-secondary d-flex flex-wrap align-items-center gap-1"
          >
            <span
              v-if="(cveSummary.critical_count || 0) > 0 || (cveSummary.hosts_with_critical || 0) > 0"
              class="badge bg-danger-lt text-danger"
            >CRIT {{ cveSummary.critical_count || 0 }}</span>
            <span v-if="(cveSummary.hosts_with_critical || 0) > 0">{{ t('dashboard.hostCount', cveSummary.hosts_with_critical || 0) }}</span>
            <span v-if="(cveSummary.high_count || 0) > 0 || (cveSummary.hosts_with_high || 0) > 0">
              <span v-if="(cveSummary.critical_count || 0) > 0 || (cveSummary.hosts_with_critical || 0) > 0">·</span>
              <span class="badge bg-warning-lt text-warning">HIGH {{ cveSummary.high_count || 0 }}</span>
            </span>
          </div>
        </div>
      </div>
    </div>
    <!-- Proxmox KPIs (hidden when not configured) -->
    <template v-if="hasProxmox">
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              {{ t('dashboard.proxmoxNodesLabel') }}
            </div>
            <div
              class="h1 mb-0"
              :class="(proxmoxSummary?.nodes_down ?? 0) > 0 ? 'text-danger' : 'text-success'"
            >
              {{ (proxmoxSummary?.node_count ?? 0) - (proxmoxSummary?.nodes_down ?? 0) }}
              <span class="text-secondary fs-4">/ {{ proxmoxSummary?.node_count ?? 0 }}</span>
            </div>
            <div class="text-secondary small mt-1">
              {{ proxmoxSummary?.vm_count ?? 0 }} VM · {{ proxmoxSummary?.lxc_count ?? 0 }} LXC
            </div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              {{ t('dashboard.proxmoxStorageLabel') }}
            </div>
            <div
              class="h1 mb-0"
              :class="proxmoxStoragePct > 80 ? 'text-danger' : proxmoxStoragePct > 60 ? 'text-warning' : 'text-success'"
            >
              {{ proxmoxStoragePct.toFixed(0) }}%
            </div>
            <div class="text-secondary small mt-1">
              {{ formatBytes(proxmoxSummary?.storage_used ?? 0) }} / {{ formatBytes(proxmoxSummary?.storage_total ?? 0) }}
            </div>
          </div>
        </div>
      </div>
    </template>
    <template v-else>
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              {{ t('common.statusOnline') }}
            </div>
            <div class="h1 mb-0 text-success">
              {{ onlineCount }}
            </div>
          </div>
        </div>
      </div>
      <div class="col-6 col-lg-3">
        <div class="card card-sm h-100">
          <div class="card-body">
            <div class="subheader">
              {{ t('common.statusOffline') }}
            </div>
            <div class="h1 mb-0 text-danger">
              {{ offlineCount }}
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { formatBytes } from '../../utils/formatters'
import { useDashboardStore } from '../../stores/dashboard'

interface CVESummary {
  critical_count?: number
  hosts_with_critical?: number
  high_count?: number
  hosts_with_high?: number
}

withDefaults(defineProps<{
  cveSummary?: CVESummary | null
  cveTimestampText?: string
}>(), {
  cveSummary: null,
  cveTimestampText: '',
})

const { t } = useI18n()

const dashboardStore = useDashboardStore()
const {
  hosts,
  aptPending,
  proxmoxSummary,
  hasProxmox,
  onlineCount,
  offlineCount,
  outdatedDockerImages,
  outdatedVersions,
  proxmoxStoragePct,
} = storeToRefs(dashboardStore)
</script>
