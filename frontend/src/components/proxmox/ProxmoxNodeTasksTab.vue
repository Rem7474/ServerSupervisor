<template>
  <div class="table-responsive scroll-table">
    <table class="table table-vcenter card-table">
      <thead>
        <tr>
          <th>
            <SortableHeader
              :label="t('proxmox.typeColumn')"
              :active="sortKey === 'task_type'"
              :direction="sortDir"
              @toggle="toggleSort('task_type')"
            />
          </th>
          <th>
            <SortableHeader
              :label="t('proxmox.objectColumn')"
              :active="sortKey === 'object_id'"
              :direction="sortDir"
              @toggle="toggleSort('object_id')"
            />
          </th>
          <th>
            <SortableHeader
              :label="t('proxmox.userColumn')"
              :active="sortKey === 'user_name'"
              :direction="sortDir"
              @toggle="toggleSort('user_name')"
            />
          </th>
          <th>
            <SortableHeader
              :label="t('proxmox.startColumn')"
              :active="sortKey === 'start_time'"
              :direction="sortDir"
              @toggle="toggleSort('start_time')"
            />
          </th>
          <th>
            <SortableHeader
              :label="t('proxmox.durationColumn')"
              :active="sortKey === 'duration'"
              :direction="sortDir"
              @toggle="toggleSort('duration')"
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
          <th />
        </tr>
      </thead>
      <tbody>
        <template v-if="!tasks.length">
          <tr>
            <td colspan="7">
              <EmptyState :title="t('proxmox.noTasksTitle')" />
            </td>
          </tr>
        </template>
        <tr
          v-for="task in sortedTasks"
          v-else
          :key="task.id"
          :class="activeUpid === task.upid ? 'table-active' : ''"
        >
          <td><span class="badge bg-azure-lt text-azure font-monospace">{{ task.task_type }}</span></td>
          <td class="text-muted">
            {{ task.object_id || '—' }}
          </td>
          <td class="text-muted small">
            {{ task.user_name }}
          </td>
          <td class="text-muted small">
            {{ formatDate(task.start_time) }}
          </td>
          <td class="text-muted small">
            {{ taskDuration(task) }}
          </td>
          <td>
            <span
              class="badge task-status-badge"
              :class="taskStatusBadgeClass(task)"
              :title="taskStatusDetail(task) || taskStatusLabel(task)"
            >{{ taskStatusLabel(task) }}</span>
            <div
              v-if="taskStatusDetail(task)"
              class="text-secondary small task-status-detail"
              :title="taskStatusDetail(task)"
            >
              {{ taskStatusDetail(task) }}
            </div>
          </td>
          <td>
            <button
              type="button"
              class="btn btn-icon btn-sm btn-ghost-secondary"
              :title="t('proxmox.viewLogsTooltip')"
              @click="emit('view-logs', { upid: task.upid, action: task.task_type, label: task.object_id })"
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
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconList } from '@tabler/icons-vue'
import SortableHeader from '../common/SortableHeader.vue'
import EmptyState from '../EmptyState.vue'
import type { ProxmoxTask } from '../../types/proxmox'
import { compareValues } from '../../utils/sort'

const { t, locale } = useI18n()

const props = defineProps<{
  tasks: ProxmoxTask[]
  activeUpid?: string | null
}>()

const emit = defineEmits<{
  (e: 'view-logs', payload: { upid: string; action: string; label: string }): void
}>()

const sortKey = ref('start_time')
const sortDir = ref<'asc' | 'desc'>('desc')

function toggleSort(key: string) {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
    return
  }
  sortKey.value = key
  sortDir.value = 'asc'
}

function taskDurationSeconds(task: ProxmoxTask): number | null {
  if (!task?.start_time) return null
  const startMs = new Date(task.start_time).getTime()
  if (!Number.isFinite(startMs)) return null
  const endMs = task.end_time
    ? new Date(task.end_time).getTime()
    : (task.status === 'running' ? Date.now() : null)
  if (endMs == null || !Number.isFinite(endMs)) return null
  return Math.max(0, Math.floor((endMs - startMs) / 1000))
}

const sortedTasks = computed(() => {
  const list = [...(props.tasks ?? [])]
  list.sort((a, b) => {
    if (sortKey.value === 'duration') {
      return compareValues(taskDurationSeconds(a), taskDurationSeconds(b), sortDir.value)
    }
    return compareValues(
      (a as unknown as Record<string, unknown>)?.[sortKey.value],
      (b as unknown as Record<string, unknown>)?.[sortKey.value],
      sortDir.value,
    )
  })
  return list
})

function formatDate(iso?: string): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString(locale.value, { dateStyle: 'short', timeStyle: 'short' })
}

function taskDuration(task: ProxmoxTask): string {
  if (!task.start_time) return '—'
  const end = task.end_time ? new Date(task.end_time) : (task.status === 'running' ? new Date() : null)
  if (!end) return '—'
  const secs = Math.floor((end.getTime() - new Date(task.start_time).getTime()) / 1000)
  if (secs < 60) return `${secs}s`
  const m = Math.floor(secs / 60)
  const s = secs % 60
  if (m < 60) return `${m}m ${s}s`
  const h = Math.floor(m / 60)
  return `${h}h ${m % 60}m`
}

// PVE reports a task's outcome in `exit_status` on the per-task endpoint and
// in `status` on the list endpoint, which is why both are consulted. A status
// that is merely a lifecycle word is not an outcome — treating it as one would
// report a queued task as failed.
const LIFECYCLE_STATUSES = new Set(['running', 'stopped', 'queued', 'pending'])

function taskOutcome(task: ProxmoxTask): string {
  if (task.exit_status) return String(task.exit_status)
  const status = String(task.status ?? '')
  return LIFECYCLE_STATUSES.has(status.toLowerCase()) ? '' : status
}

// The badge carries the state, in the same vocabulary as the backups table and
// the live task panel; the raw PVE wording goes underneath and in the tooltip
// rather than becoming an untranslated, unbounded badge of its own.
function taskStatusLabel(task: ProxmoxTask): string {
  if (task.status === 'running') return t('proxmox.runningStatusLabel')
  const outcome = taskOutcome(task)
  // No outcome yet: show the lifecycle word PVE gave rather than inventing one.
  if (!outcome) return String(task.status || '—')
  if (outcome === 'OK') return 'OK'
  if (outcome.toUpperCase().startsWith('WARNINGS')) return t('common.stateWarnings')
  return t('common.stateFailed')
}

function taskStatusBadgeClass(task: ProxmoxTask): string {
  if (task.status === 'running') return 'bg-primary-lt text-primary'
  const outcome = taskOutcome(task)
  if (!outcome) return 'bg-secondary-lt text-secondary'
  if (outcome === 'OK') return 'bg-success-lt text-success'
  if (outcome.toUpperCase().startsWith('WARNINGS')) return 'bg-warning-lt text-warning'
  return 'bg-danger-lt text-danger'
}

// Only worth showing when it says more than the badge already does.
function taskStatusDetail(task: ProxmoxTask): string {
  const outcome = taskOutcome(task)
  return outcome === 'OK' ? '' : outcome
}
</script>

<style scoped>
/* The badge now holds a short state; the raw PVE outcome sits below it and is
   what needs capping — it can be a full storage error. */
.task-status-detail {
  max-width: 22rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
