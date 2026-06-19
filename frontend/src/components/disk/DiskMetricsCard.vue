<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">
        Santé des disques
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
      v-else-if="metrics.length === 0"
      class="card-body text-center text-muted py-5"
    >
      <IconClock
        :size="36"
        class="mb-2 icon icon-md icon-responsive-lg"
        :stroke-width="1.5"
      />
      <div class="small fw-medium">
        Aucune donnée de disque disponible
      </div>
      <div class="mt-1 opacity-75 small">
        L'agent doit être actif pour collecter les métriques disque
      </div>
    </div>
    <div
      v-else
      class="table-responsive"
    >
      <table class="table table-vcenter card-table mb-0">
        <thead>
          <tr>
            <th>Point de montage</th>
            <th>Utilisation</th>
            <th>Utilisation espace</th>
            <th>Inodes</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="metric in metrics"
            :key="metric.mount_point"
          >
            <td>
              <div class="fw-bold">
                {{ metric.mount_point }}
              </div>
              <div class="text-muted small">
                {{ metric.filesystem }}
              </div>
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
                />
              </div>
              <div class="text-muted small">
                {{ metric.used_percent.toFixed(1) }}%
              </div>
            </td>
            <td>
              <span
                v-if="metric.inodes_total > 0"
                class="text-muted small"
              >
                {{ metric.inodes_used }} / {{ metric.inodes_total }}
                <div class="progress progress-sm mt-1">
                  <div 
                    class="progress-bar" 
                    :class="getProgressBarClass(metric.inodes_percent)"
                    :style="{ width: metric.inodes_percent + '%' }"
                  />
                </div>
              </span>
              <span
                v-else
                class="text-muted small"
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
import { useDiskMetrics, type DiskMetric } from '../../composables/useDiskMetrics'

const props = withDefaults(defineProps<{
  hostId: string
  initialMetrics?: DiskMetric[] | null
}>(), {
  initialMetrics: null,
})

const { metrics, loading, load } = useDiskMetrics(props.hostId, props.initialMetrics)

onMounted(async () => {
  if (props.initialMetrics) return
  await load()
})

function formatGB(bytes: number): string {
  return `${bytes.toFixed(1)}G`
}

function getProgressBarClass(percent: number): string {
  if (percent >= 90) return 'bg-danger'
  if (percent >= 80) return 'bg-warning'
  if (percent >= 70) return 'bg-info'
  return 'bg-success'
}
</script>

