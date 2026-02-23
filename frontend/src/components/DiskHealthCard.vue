<template>
  <div class="card">
    <div class="card-header">
      <h3 class="card-title">État SMART des disques</h3>
    </div>
    <div v-if="loading" class="card-body text-center py-4">
      <div class="spinner-border spinner-border-sm text-muted"></div>
    </div>
    <div v-else-if="health.length === 0" class="card-body text-center text-muted py-4">
      Aucune donnée SMART disponible (smartctl peut ne pas être installé)
    </div>
    <div v-else class="card-body">
      <div class="space-y-3">
        <div 
          v-for="disk in health" 
          :key="disk.device"
          class="border rounded p-3"
          :class="getCardClass(disk.smart_status)"
        >
          <div class="d-flex align-items-start justify-content-between">
            <div>
              <div class="fw-bold">{{ disk.device }}</div>
              <div class="text-muted small">
                {{ disk.model }}
                <span v-if="disk.serial_number" class="ms-2">{{ disk.serial_number }}</span>
              </div>
            </div>
            <span :class="getStatusBadgeClass(disk.smart_status)" class="badge">
              {{ disk.smart_status }}
            </span>
          </div>

          <div class="row mt-3 text-sm">
            <div class="col-6">
              <div class="text-muted" style="font-size: 0.8rem">Température</div>
              <div class="fw-bold">
                <span v-if="disk.temperature > 0">{{ disk.temperature }}°C</span>
                <span v-else class="text-muted">N/A</span>
              </div>
            </div>
            <div class="col-6">
              <div class="text-muted" style="font-size: 0.8rem">Heures d'utilisation</div>
              <div class="fw-bold">
                <span v-if="disk.power_on_hours > 0">{{ disk.power_on_hours.toLocaleString() }}h</span>
                <span v-else class="text-muted">N/A</span>
              </div>
            </div>
            <div class="col-6 mt-2">
              <div class="text-muted" style="font-size: 0.8rem">Secteurs réalloués</div>
              <div class="fw-bold" :class="{ 'text-danger': disk.realloc_sectors > 10 }">
                {{ disk.realloc_sectors }}
              </div>
            </div>
            <div class="col-6 mt-2">
              <div class="text-muted" style="font-size: 0.8rem">Secteurs en attente</div>
              <div class="fw-bold" :class="{ 'text-danger': disk.pending_sectors > 0 }">
                {{ disk.pending_sectors }}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'

defineProps({
  hostId: {
    type: String,
    required: true
  }
})

const health = ref([])
const loading = ref(true)

onMounted(async () => {
  await loadDiskHealth()
})

async function loadDiskHealth() {
  try {
    loading.value = true
    const res = await api.get(`/api/v1/hosts/${hostId}/disk/health`)
    health.value = res.data || []
  } catch (err) {
    console.error('Failed to load disk health:', err)
  } finally {
    loading.value = false
  }
}

function getStatusBadgeClass(status) {
  switch (status) {
    case 'PASSED': return 'bg-success'
    case 'FAILED': return 'bg-danger'
    case 'UNKNOWN': return 'bg-warning'
    case 'NOT_AVAILABLE': return 'bg-secondary'
    default: return 'bg-secondary'
  }
}

function getCardClass(status) {
  switch (status) {
    case 'FAILED': return 'bg-danger-lt border-danger'
    case 'UNKNOWN': return 'bg-warning-lt border-warning'
    case 'PASSED': return 'bg-success-lt border-success'
    default: return 'bg-light'
  }
}
</script>

<style scoped>
.space-y-3 > div + div {
  margin-top: 1rem;
}
</style>
