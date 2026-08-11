<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">
        État SMART des disques
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
      v-else-if="health.length === 0"
      class="card-body"
    >
      <EmptyState
        :icon="IconClock"
        title="Aucune donnée SMART disponible"
        subtitle="Vérifie que l'agent collecte SMART et que smartmontools est installé."
      />
    </div>
    <div
      v-else
      class="table-responsive scroll-table"
    >
      <table class="table table-vcenter card-table table-sm">
        <thead>
          <tr>
            <th>Périphérique</th>
            <th>Statut</th>
            <th>Température</th>
            <th>Heures d'utilisation</th>
            <th>Secteurs (réal. / attente / incorr.)</th>
            <th>Cycles d'alim.</th>
            <th>Usure SSD/NVMe</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="disk in health"
            :key="disk.device"
          >
            <td class="text-truncate">
              <div class="fw-semibold">
                {{ disk.device }}
              </div>
              <div class="text-muted small text-truncate">
                {{ disk.model }}
                <span
                  v-if="disk.serial_number"
                  class="ms-1"
                >{{ disk.serial_number }}</span>
              </div>
            </td>
            <td>
              <BadgePill
                :tone="smartStatusTone(disk.smart_status)"
                :text="disk.smart_status"
                compact
              />
            </td>
            <td>
              <span v-if="disk.temperature > 0">{{ disk.temperature }}°C</span>
              <span
                v-else
                class="text-muted"
              >N/A</span>
            </td>
            <td>
              <span v-if="disk.power_on_hours > 0">{{ disk.power_on_hours.toLocaleString() }}h</span>
              <span
                v-else
                class="text-muted"
              >N/A</span>
            </td>
            <td>
              <span :class="{ 'text-danger fw-medium': disk.realloc_sectors > 10 }">{{ disk.realloc_sectors }}</span>
              <span class="text-muted mx-1">/</span>
              <span :class="{ 'text-danger fw-medium': disk.pending_sectors > 0 }">{{ disk.pending_sectors }}</span>
              <span class="text-muted mx-1">/</span>
              <span :class="{ 'text-danger fw-medium': (disk.uncorrectable_sectors ?? 0) > 0 }">{{ disk.uncorrectable_sectors ?? 0 }}</span>
            </td>
            <td>
              <span v-if="(disk.power_cycles ?? 0) > 0">{{ disk.power_cycles!.toLocaleString() }}</span>
              <span
                v-else
                class="text-muted"
              >N/A</span>
            </td>
            <td>
              <template v-if="(disk.percentage_used ?? 0) > 0">
                <div class="d-flex align-items-center gap-2">
                  <div class="progress progress-xs flex-grow-1 disk-wear-progress-min-60">
                    <div
                      class="progress-bar"
                      :class="wearColor(disk.percentage_used ?? 0)"
                      :style="`width:${disk.percentage_used}%`"
                    />
                  </div>
                  <span class="text-muted small">{{ disk.percentage_used }}%</span>
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
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { IconClock } from '@tabler/icons-vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import EmptyState from '../EmptyState.vue'
import BadgePill from '../common/BadgePill.vue'
import { useDiskHealth, type DiskHealth } from '../../composables/useDiskHealth'
import { smartStatusTone } from '../../utils/diskHealth'

const props = withDefaults(defineProps<{
  hostId: string
  initialHealth?: DiskHealth[] | null
}>(), {
  initialHealth: null,
})

const { health, loading, load } = useDiskHealth(props.hostId, props.initialHealth)

onMounted(async () => {
  if (props.initialHealth) return
  await load()
})

// percentage_used is SSD/NVMe wear *consumed* (0 = new, high = worn) — same
// "high is bad" direction as a usage metric, unlike Proxmox's own wearout
// column (ProxmoxNodeDisksTab.vue's wearoutColor), which is remaining life
// and therefore inverted. Keeps this file's original 50/80 thresholds
// instead of switching to utils/metricColor.ts's 75/90 — those are tuned
// for CPU/RAM/disk *usage*, not SMART wear-out, which fails at a much lower
// tolerance than "the disk is 80% busy."
function wearColor(pct: number): string {
  if (pct >= 80) return 'bg-danger'
  if (pct >= 50) return 'bg-warning'
  return 'bg-success'
}
</script>

<style scoped>
.disk-wear-progress-min-60 {
  min-width: 60px;
}
</style>
