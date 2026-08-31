<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">
        {{ t('monitoring.diskUsageTitle') }}
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
      class="card-body"
    >
      <EmptyState
        :icon="IconClock"
        :title="t('monitoring.noDiskData')"
        :subtitle="t('monitoring.noDiskDataHint')"
      />
    </div>
    <div
      v-else
      class="table-responsive"
    >
      <table class="table table-vcenter card-table mb-0">
        <thead>
          <tr>
            <th>{{ t('monitoring.mountPoint') }}</th>
            <th>{{ t('monitoring.usage') }}</th>
            <th style="width: 220px;">
              {{ t('monitoring.spaceUsage') }}
            </th>
            <th style="width: 220px;">
              {{ t('monitoring.inodes') }}
            </th>
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
              <div class="d-flex align-items-center gap-2">
                <div class="progress progress-xs flex-grow-1 disk-metrics-progress-min">
                  <div
                    class="progress-bar"
                    :class="getProgressBarClass(metric.used_percent)"
                    :style="{ width: metric.used_percent + '%' }"
                  />
                </div>
                <span class="text-muted small">{{ metric.used_percent.toFixed(1) }}%</span>
              </div>
              <div
                v-if="metric.forecast_days_until_full != null"
                class="small mt-1"
                :class="forecastClass(metric.forecast_days_until_full)"
                :title="t('monitoring.forecastTooltip')"
              >
                {{ t('monitoring.forecastFull', { days: Math.round(metric.forecast_days_until_full) }) }}
              </div>
            </td>
            <td>
              <div
                v-if="metric.inodes_total > 0"
                class="d-flex align-items-center gap-2"
              >
                <div class="progress progress-xs flex-grow-1 disk-metrics-progress-min">
                  <div
                    class="progress-bar"
                    :class="getProgressBarClass(metric.inodes_percent)"
                    :style="{ width: metric.inodes_percent + '%' }"
                  />
                </div>
                <span class="text-muted small">{{ metric.inodes_used }} / {{ metric.inodes_total }}</span>
              </div>
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
import { useI18n } from 'vue-i18n'
import { IconClock } from '@tabler/icons-vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import EmptyState from '../EmptyState.vue'
import { useDiskMetrics, type DiskMetric } from '../../composables/useDiskMetrics'

const { t } = useI18n()

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
  if (percent >= 70) return 'bg-primary'
  return 'bg-success'
}

function forecastClass(days: number): string {
  if (days <= 14) return 'text-danger'
  if (days <= 30) return 'text-warning'
  return 'text-muted'
}
</script>

<style scoped>
/* Same min-width convention as DiskHealthCard/ProxmoxNodeStorageTab's own
   progress-bar columns — without it, "Utilisation espace" and "Inodes"
   render at different widths depending on how much room the row's other
   columns leave them, instead of lining up with each other. */
.disk-metrics-progress-min {
  min-width: 100px;
}
</style>

