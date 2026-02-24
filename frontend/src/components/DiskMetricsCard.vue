<template>
  <div class="card">
    <div class="card-header">
      <h3 class="card-title">Santé des disques</h3>
    </div>
    <div v-if="loading" class="card-body text-center py-4">
      <div class="spinner-border spinner-border-sm text-muted"></div>
    </div>
    <div v-else-if="metrics.length === 0" class="card-body text-center text-muted py-4">
      Aucune donnée de disque disponible
    </div>
    <div v-else class="table-responsive">
      <table class="table table-vcenter card-table table-sm">
        <thead>
          <tr>
            <th>Point de montage</th>
            <th>Utilisation</th>
            <th style="width: 20%">Utilisation espace</th>
            <th style="width: 15%">Inodes</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="metric in metrics" :key="metric.mount_point">
            <td>
              <div class="fw-bold">{{ metric.mount_point }}</div>
              <div class="text-muted small">{{ metric.filesystem }}</div>
            </td>
            <td>
              <span class="text-muted">{{ formatGB(metric.used_gb) }} / {{ formatGB(metric.size_gb) }}</span>
            </td>
            <td>
              <div class="progress progress-sm">
                <div 
                  class="progress-bar" 
                  :class="getProgressBarClass(metric.used_percent)"
                  :style="{ width: metric.used_percent + '%' }"
                ></div>
              </div>
              <div class="text-muted small">{{ metric.used_percent.toFixed(1) }}%</div>
            </td>
            <td>
              <span v-if="metric.inodes_total > 0" class="text-muted small">
                {{ metric.inodes_used }} / {{ metric.inodes_total }}
                <div class="progress progress-sm mt-1">
                  <div 
                    class="progress-bar" 
                    :class="getProgressBarClass(metric.inodes_percent)"
                    :style="{ width: metric.inodes_percent + '%' }"
                  ></div>
                </div>
              </span>
              <span v-else class="text-muted small">N/A</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import apiClient from '../api'

const props = defineProps({
  hostId: {
    type: String,
    required: true
  }
})

const metrics = ref([])
const loading = ref(true)

onMounted(async () => {
  await loadDiskMetrics()
})

async function loadDiskMetrics() {
  try {
    loading.value = true
    const res = await apiClient.getDiskMetrics(props.hostId)
    metrics.value = res.data || []
  } catch (err) {
    console.error('Failed to load disk metrics:', err)
  } finally {
    loading.value = false
  }
}

function formatGB(bytes) {
  return `${bytes.toFixed(1)}G`
}

function getProgressBarClass(percent) {
  if (percent >= 90) return 'bg-danger'
  if (percent >= 80) return 'bg-warning'
  if (percent >= 70) return 'bg-info'
  return 'bg-success'
}
</script>
