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
      class="card-body text-center text-muted py-5"
    >
      <IconClock
        :size="36"
        class="mb-2 icon icon-md icon-responsive-lg"
        :stroke-width="1.5"
      />
      <div class="small fw-medium">
        Aucune donnée SMART disponible
      </div>
      <div class="mt-1 opacity-75 small">
        Vérifie que l'agent collecte SMART et que smartmontools est installé.
      </div>
    </div>
    <div
      v-else
      class="card-body"
    >
      <div class="d-flex flex-column gap-3">
        <div
          v-for="disk in health"
          :key="disk.device"
          class="border rounded-3 p-3 shadow-sm"
          :class="getCardClass(disk.smart_status)"
        >
          <div class="d-flex flex-wrap align-items-start justify-content-between gap-2">
            <div class="min-w-0">
              <div class="fw-semibold text-truncate">
                {{ disk.device }}
              </div>
              <div class="text-muted small text-truncate">
                {{ disk.model }}
                <span
                  v-if="disk.serial_number"
                  class="ms-2"
                >{{ disk.serial_number }}</span>
              </div>
            </div>
            <BadgePill
              :tone="getStatusBadgeClass(disk.smart_status)"
              :text="disk.smart_status"
              compact
            />
          </div>

          <div class="row mt-3 g-3 text-sm">
            <div class="col-6">
              <div
                class="text-muted small"
              >
                Température
              </div>
              <div class="fw-bold">
                <span v-if="disk.temperature > 0">{{ disk.temperature }}°C</span>
                <span
                  v-else
                  class="text-muted"
                >N/A</span>
              </div>
            </div>
            <div class="col-6">
              <div
                class="text-muted small"
              >
                Heures d'utilisation
              </div>
              <div class="fw-bold">
                <span v-if="disk.power_on_hours > 0">{{ disk.power_on_hours.toLocaleString() }}h</span>
                <span
                  v-else
                  class="text-muted"
                >N/A</span>
              </div>
            </div>
            <div class="col-6 mt-2">
              <div
                class="text-muted small"
              >
                Secteurs réalloués
              </div>
              <div
                class="fw-bold"
                :class="{ 'text-danger': disk.realloc_sectors > 10 }"
              >
                {{ disk.realloc_sectors }}
              </div>
            </div>
            <div class="col-6 mt-2">
              <div
                class="text-muted small"
              >
                Secteurs en attente
              </div>
              <div
                class="fw-bold"
                :class="{ 'text-danger': disk.pending_sectors > 0 }"
              >
                {{ disk.pending_sectors }}
              </div>
            </div>
            <div class="col-6 mt-2">
              <div
                class="text-muted small"
              >
                Secteurs incorrigibles
              </div>
              <div
                class="fw-bold"
                :class="{ 'text-danger': (disk.uncorrectable_sectors ?? 0) > 0 }"
              >
                {{ disk.uncorrectable_sectors ?? 0 }}
              </div>
            </div>
            <div class="col-6 mt-2">
              <div
                class="text-muted small"
              >
                Cycles d'alimentation
              </div>
              <div class="fw-bold">
                <span v-if="(disk.power_cycles ?? 0) > 0">{{ disk.power_cycles!.toLocaleString() }}</span>
                <span
                  v-else
                  class="text-muted"
                >N/A</span>
              </div>
            </div>
            <div class="col-6 mt-2">
              <div
                class="text-muted small"
              >
                Usure SSD/NVMe
              </div>
              <div
                class="fw-bold"
                :class="{ 'text-danger': (disk.percentage_used ?? 0) >= 80, 'text-warning': (disk.percentage_used ?? 0) >= 50 && (disk.percentage_used ?? 0) < 80 }"
              >
                <span v-if="(disk.percentage_used ?? 0) > 0">{{ disk.percentage_used }}%</span>
                <span
                  v-else
                  class="text-muted"
                >N/A</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { IconClock } from '@tabler/icons-vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import BadgePill from '../common/BadgePill.vue'
import { useDiskHealth, type DiskHealth } from '../../composables/useDiskHealth'

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

type Tone = 'success' | 'danger' | 'warning' | 'secondary'

function getStatusBadgeClass(status: string): Tone {
  switch (status) {
    case 'PASSED': return 'success'
    case 'FAILED': return 'danger'
    case 'UNKNOWN': return 'warning'
    case 'NOT_AVAILABLE': return 'secondary'
    default: return 'secondary'
  }
}

function getCardClass(status: string): string {
  switch (status) {
    case 'FAILED': return 'bg-danger-lt border-danger'
    case 'UNKNOWN': return 'bg-warning-lt border-warning'
    case 'PASSED': return 'bg-success-lt border-success'
    default: return 'bg-secondary-lt border-secondary'
  }
}
</script>


