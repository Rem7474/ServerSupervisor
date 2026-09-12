<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">
        {{ t('proxmox.diskSmartStatusTitle') }}
        <span
          v-if="nodeName"
          class="text-muted fw-normal ms-1"
        >{{ t('proxmox.nodeSuffixLabel', { name: nodeName }) }}</span>
      </h3>
    </div>
    <div
      v-if="loading"
      class="card-body text-center py-4"
    >
      <LoadingSkeleton
        variant="card"
        :lines="3"
      />
    </div>
    <div
      v-else-if="disks.length === 0"
      class="card-body"
    >
      <EmptyState
        :icon="IconDisc"
        :title="t('proxmox.noDiskDataTitle')"
        :subtitle="t('proxmox.noDiskDataSubtitle')"
      />
    </div>
    <template v-else>
      <div class="card-body pb-0">
        <div class="mb-0 small text-muted">
          {{ t('proxmox.cantReadSmartLocallyHint') }}
        </div>
      </div>
      <div class="table-responsive scroll-table">
        <table class="table table-vcenter card-table table-sm">
          <thead>
            <tr>
              <th>{{ t('proxmox.deviceColumn') }}</th>
              <th>{{ t('proxmox.statusColumn') }}</th>
              <th>{{ t('proxmox.sizeColumn') }}</th>
              <th>{{ t('proxmox.remainingLifeColumn') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="disk in disks"
              :key="disk.id"
            >
              <td class="text-truncate">
                <div class="fw-semibold">
                  {{ disk.dev_path }}
                  <span
                    v-if="disk.disk_type"
                    class="badge bg-secondary-lt text-uppercase ms-1"
                  >{{ disk.disk_type }}</span>
                </div>
                <div class="text-muted small text-truncate">
                  {{ disk.model }}
                  <span
                    v-if="disk.serial"
                    class="ms-1"
                  >{{ disk.serial }}</span>
                </div>
              </td>
              <td>
                <BadgePill
                  :tone="smartStatusTone(disk.health)"
                  :text="disk.health"
                  compact
                />
              </td>
              <td>
                {{ formatBytes(disk.size_bytes) }}
              </td>
              <td>
                <template v-if="disk.wearout >= 0">
                  <div class="d-flex align-items-center gap-2">
                    <div class="progress progress-xs flex-grow-1 disk-wear-progress-min-60">
                      <div
                        class="progress-bar"
                        :class="wearoutColor(disk.wearout)"
                        :style="`width:${disk.wearout}%`"
                      />
                    </div>
                    <span class="text-muted small">{{ disk.wearout }}%</span>
                  </div>
                </template>
                <span
                  v-else
                  class="text-muted"
                >N/A</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import BadgePill from '../common/BadgePill.vue'
import EmptyState from '../EmptyState.vue'
import { IconDisc } from '@tabler/icons-vue'
import { useProxmoxHostDisks } from '../../composables/useProxmoxHostDisks'
import { smartStatusTone } from '../../utils/diskHealth'

const { t } = useI18n()

const props = defineProps<{
  hostId: string
  nodeName?: string | null
}>()

const { disks, loading, load } = useProxmoxHostDisks(props.hostId)

onMounted(load)

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return 'N/A'
  const unitKeys = ['proxmox.byteUnitBytes', 'proxmox.byteUnitKilo', 'proxmox.byteUnitMega', 'proxmox.byteUnitGiga', 'proxmox.byteUnitTera', 'proxmox.byteUnitPeta']
  let value = bytes
  let i = 0
  while (value >= 1024 && i < unitKeys.length - 1) {
    value /= 1024
    i++
  }
  return `${value.toFixed(value >= 100 || i === 0 ? 0 : 1)} ${t(unitKeys[i])}`
}

// wearout is remaining life (100 = new, 0 = worn out) — same direction and
// thresholds as ProxmoxNodeDisksTab.vue's own wearoutColor for the cluster
// disks table, so a disk reads the same severity color whether it's shown
// there or here.
function wearoutColor(wearout: number): string {
  if (wearout < 20) return 'bg-danger'
  if (wearout < 50) return 'bg-warning'
  return 'bg-success'
}
</script>

<style scoped>
.disk-wear-progress-min-60 {
  min-width: 60px;
}
</style>
