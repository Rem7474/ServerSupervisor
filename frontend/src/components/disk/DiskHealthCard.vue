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
      <svg
        xmlns="http://www.w3.org/2000/svg"
        class="mb-2 icon icon-md icon-responsive-lg"
        width="36"
        height="36"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
        style="opacity:.35"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="1.5"
          d="M5 12a7 7 0 1014 0A7 7 0 005 12zm7-3v3l2 2"
        />
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="1.5"
          d="M3 6h18M3 18h18"
        />
      </svg>
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
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import apiClient from '../../api'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import BadgePill from '../common/BadgePill.vue'

interface DiskHealth {
  device: string
  model?: string
  serial_number?: string
  smart_status: string
  temperature: number
  power_on_hours: number
  realloc_sectors: number
  pending_sectors: number
}

const props = withDefaults(defineProps<{
  hostId: string
  initialHealth?: DiskHealth[] | null
}>(), {
  initialHealth: null,
})

const health = ref<DiskHealth[]>(props.initialHealth || [])
const loading = ref(!props.initialHealth)

onMounted(async () => {
  if (props.initialHealth) return
  await loadDiskHealth()
})

async function loadDiskHealth(): Promise<void> {
  try {
    loading.value = true
    const res = await apiClient.getDiskHealth(props.hostId)
    health.value = res.data || []
  } catch (err) {
    console.error('Failed to load disk health:', err)
  } finally {
    loading.value = false
  }
}

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


