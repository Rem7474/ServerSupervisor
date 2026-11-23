<template>
  <div
    v-if="host && host.status !== 'online'"
    class="alert alert-warning mb-3"
  >
    <IconAlertCircle
      :size="16"
      class="icon me-2"
    />
    {{ t('host.agentOfflineWarning') }}
  </div>
  <HostMetricsPanel
    :host-id="hostId"
    :metrics="effectiveMetrics"
    :metrics-source="effectiveMetricsSource"
    :proxmox-guest-id="proxmoxLink?.guest_id ?? null"
    :refresh-tick="metricsUpdatedAt"
  />
  <DiskMetricsCard
    :host-id="hostId"
    :initial-metrics="diskMetrics"
    class="mb-4"
  />
  <DiskHistoryChart
    :host-id="hostId"
    :mounts="(diskMetrics?.map((d) => d.mount_point) ?? [])"
    :refresh-tick="metricsUpdatedAt"
    class="mb-4"
  />
  <DiskHealthCard
    v-if="hasLocalSmart || !isProxmoxLinked"
    :host-id="hostId"
    :initial-health="diskHealth"
    class="mb-4"
  />
  <ProxmoxHostDiskHealthCard
    v-else
    :host-id="hostId"
    :node-name="proxmoxLink?.node_name ?? null"
    class="mb-4"
  />
</template>

<script setup lang="ts">
import { IconAlertCircle } from '@tabler/icons-vue'
import { useI18n } from 'vue-i18n'
import HostMetricsPanel from './HostMetricsPanel.vue'
import DiskMetricsCard from '../disk/DiskMetricsCard.vue'
import DiskHistoryChart from '../disk/DiskHistoryChart.vue'
import DiskHealthCard from '../disk/DiskHealthCard.vue'
import ProxmoxHostDiskHealthCard from '../proxmox/ProxmoxHostDiskHealthCard.vue'
import type { DiskMetric } from '../../composables/useDiskMetrics'
import type { DiskHealth } from '../../composables/useDiskHealth'
import type { HostRecord } from '../../types/hostDetail'

defineProps<{
  hostId: string
  host: HostRecord | null
  effectiveMetrics: HostRecord | null
  effectiveMetricsSource: string
  proxmoxLink: HostRecord | null
  metricsUpdatedAt: number
  diskMetrics: DiskMetric[] | null
  diskHealth: DiskHealth[] | null
  hasLocalSmart: boolean
  isProxmoxLinked: boolean
}>()

const { t } = useI18n()
</script>
