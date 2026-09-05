<template>
  <div>
    <div
      v-if="error"
      class="card-body pb-0"
    >
      <div class="alert alert-danger mb-0">
        {{ error }}
      </div>
    </div>
    <div
      v-if="loading && !jobs.length && !runs.length"
      class="card-body"
    >
      <LoadingSkeleton
        variant="table"
        :lines="4"
      />
    </div>

    <template v-if="!loading || jobs.length || runs.length">
      <div class="card-body">
        <div class="fw-semibold mb-2">
          {{ t('proxmox.configuredJobsLabel') }}
        </div>
        <div
          v-if="!jobs.length"
          class="text-secondary small"
        >
          {{ t('proxmox.noBackupJobsText') }}
        </div>
        <div
          v-else
          class="table-responsive scroll-table"
        >
          <table class="table table-sm table-vcenter card-table mb-0">
            <thead>
              <tr>
                <th>{{ t('proxmox.jobColumn') }}</th>
                <th>{{ t('proxmox.scheduleColumn') }}</th>
                <th>{{ t('proxmox.storageColumn') }}</th>
                <th>{{ t('proxmox.modeColumn') }}</th>
                <th>{{ t('proxmox.targetsColumn') }}</th>
                <th>{{ t('proxmox.statusColumn') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="j in jobs"
                :key="j.id"
              >
                <td class="font-monospace small">
                  {{ j.job_id }}
                </td>
                <td>
                  <code>{{ j.schedule || '—' }}</code>
                </td>
                <td class="text-secondary small">
                  {{ j.storage || '—' }}
                </td>
                <td class="text-secondary small">
                  {{ j.mode || '—' }}
                </td>
                <td class="small">
                  {{ j.vmids === 'all' ? t('proxmox.allLabel') : (j.vmids || '—') }}
                </td>
                <td>
                  <span
                    class="badge"
                    :class="j.enabled ? 'bg-success-lt text-success' : 'bg-secondary-lt text-secondary'"
                  >{{ j.enabled ? t('proxmox.enabledBadge') : t('proxmox.disabledBadge') }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="card-body border-top">
        <div class="fw-semibold mb-2">
          {{ t('proxmox.lastResultPerVmLabel') }}
        </div>
        <EmptyState
          v-if="!runs.length"
          :title="t('proxmox.noBackupRunsTitle')"
        />
        <div
          v-else
          class="table-responsive scroll-table"
        >
          <table class="table table-sm table-vcenter card-table mb-0">
            <thead>
              <tr>
                <th>
                  <SortableHeader
                    :label="t('proxmox.vmColumn')"
                    :active="sortKey === 'guest_name'"
                    :direction="sortDir"
                    @toggle="toggleSort('guest_name')"
                  />
                </th>
                <th>
                  <SortableHeader
                    :label="t('proxmox.statusColumn')"
                    :active="sortKey === 'status'"
                    :direction="sortDir"
                    @toggle="toggleSort('status')"
                  />
                </th>
                <th>
                  <SortableHeader
                    :label="t('proxmox.endColumn')"
                    :active="sortKey === 'end_time'"
                    :direction="sortDir"
                    @toggle="toggleSort('end_time')"
                  />
                </th>
                <th />
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="r in sortedRuns"
                :key="r.id"
              >
                <td>{{ r.guest_name || `VM ${r.vmid}` }}</td>
                <td>
                  <span
                    class="badge"
                    :class="getExecutionStateClass(r.status)"
                  >{{ getExecutionStateLabel(r.status) }}</span>
                </td>
                <td class="text-secondary small">
                  {{ formatDate(r.end_time) }}
                </td>
                <td class="text-end">
                  <button
                    v-if="r.task_upid"
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-secondary"
                    :title="t('proxmox.viewLogsTooltip')"
                    @click="emit('view-logs', { upid: r.task_upid, action: 'vzdump', label: r.guest_name || `VM ${r.vmid}` })"
                  >
                    <IconList
                      :size="16"
                      class="icon icon-sm"
                    />
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconList } from '@tabler/icons-vue'
import SortableHeader from '../common/SortableHeader.vue'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import { getExecutionStateClass, getExecutionStateLabel } from '../../utils/statusClasses'
import { compareValues } from '../../utils/sort'
import type { ProxmoxBackupJob, ProxmoxBackupRun } from '../../types/proxmox'

const { t, locale } = useI18n()

const props = defineProps<{
  jobs: ProxmoxBackupJob[]
  runs: ProxmoxBackupRun[]
  loading: boolean
  error: string
}>()

const emit = defineEmits<{
  (e: 'view-logs', payload: { upid: string; action: string; label: string }): void
}>()

const sortKey = ref<'guest_name' | 'status' | 'end_time'>('end_time')
const sortDir = ref<'asc' | 'desc'>('desc')

function toggleSort(key: typeof sortKey.value): void {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
    return
  }
  sortKey.value = key
  sortDir.value = 'asc'
}

const sortedRuns = computed(() => {
  const list = [...props.runs]
  list.sort((a, b) => {
    if (sortKey.value === 'guest_name') {
      return compareValues(a.guest_name || `VM ${a.vmid}`, b.guest_name || `VM ${b.vmid}`, sortDir.value)
    }
    return compareValues(a[sortKey.value], b[sortKey.value], sortDir.value)
  })
  return list
})

function formatDate(iso?: string): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString(locale.value, { dateStyle: 'short', timeStyle: 'short' })
}
</script>
