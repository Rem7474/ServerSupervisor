<template>
  <div class="table-responsive">
    <table class="table table-vcenter card-table">
      <thead>
        <tr>
          <th>
            <SortableHeader
              label="Type"
              :active="sortKey === 'task_type'"
              :direction="sortDir"
              @toggle="toggleSort('task_type')"
            />
          </th>
          <th>
            <SortableHeader
              label="Objet"
              :active="sortKey === 'object_id'"
              :direction="sortDir"
              @toggle="toggleSort('object_id')"
            />
          </th>
          <th>
            <SortableHeader
              label="Utilisateur"
              :active="sortKey === 'user_name'"
              :direction="sortDir"
              @toggle="toggleSort('user_name')"
            />
          </th>
          <th>
            <SortableHeader
              label="Début"
              :active="sortKey === 'start_time'"
              :direction="sortDir"
              @toggle="toggleSort('start_time')"
            />
          </th>
          <th>
            <SortableHeader
              label="Durée"
              :active="sortKey === 'duration'"
              :direction="sortDir"
              @toggle="toggleSort('duration')"
            />
          </th>
          <th>
            <SortableHeader
              label="Statut"
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
            <td
              colspan="7"
              class="text-center text-muted py-4"
            >
              Aucune tâche récente pour ce nœud.
            </td>
          </tr>
        </template>
        <tr
          v-for="t in sortedTasks"
          v-else
          :key="t.id"
          :class="activeUpid === t.upid ? 'table-active' : ''"
        >
          <td><span class="badge bg-azure-lt text-azure font-monospace">{{ t.task_type }}</span></td>
          <td class="text-muted">
            {{ t.object_id || '—' }}
          </td>
          <td class="text-muted small">
            {{ t.user_name }}
          </td>
          <td class="text-muted small">
            {{ formatDate(t.start_time) }}
          </td>
          <td class="text-muted small">
            {{ taskDuration(t) }}
          </td>
          <td>
            <span
              class="badge task-status-badge"
              :class="taskStatusBadgeClass(t)"
              :title="taskStatusLabel(t)"
            >{{ taskStatusLabel(t) }}</span>
          </td>
          <td>
            <button
              type="button"
              class="btn btn-sm btn-ghost-secondary"
              title="Voir les logs"
              @click="emit('view-logs', { upid: t.upid, action: t.task_type, label: t.object_id })"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="icon icon-sm"
                width="16"
                height="16"
                viewBox="0 0 24 24"
                stroke-width="2"
                stroke="currentColor"
                fill="none"
              ><path
                stroke="none"
                d="M0 0h24v24H0z"
                fill="none"
              /><path d="M4 6l16 0" /><path d="M4 12l16 0" /><path d="M4 18l12 0" /></svg>
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import SortableHeader from '../common/SortableHeader.vue'

type Task = Record<string, any>

const props = defineProps<{
  tasks: Task[]
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

function compareValues(a: unknown, b: unknown, direction: 'asc' | 'desc' = 'asc'): number {
  const dir = direction === 'asc' ? 1 : -1
  if (a == null && b == null) return 0
  if (a == null) return 1 * dir
  if (b == null) return -1 * dir
  if (typeof a === 'string' || typeof b === 'string') {
    return String(a).localeCompare(String(b), 'fr', { sensitivity: 'base' }) * dir
  }
  if (a < b) return -1 * dir
  if (a > b) return 1 * dir
  return 0
}

function taskDurationSeconds(task: Task): number | null {
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
    return compareValues(a?.[sortKey.value], b?.[sortKey.value], sortDir.value)
  })
  return list
})

function formatDate(iso: string): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })
}

function taskDuration(t: Task): string {
  if (!t.start_time) return '—'
  const end = t.end_time ? new Date(t.end_time) : (t.status === 'running' ? new Date() : null)
  if (!end) return '—'
  const secs = Math.floor((end.getTime() - new Date(t.start_time).getTime()) / 1000)
  if (secs < 60) return `${secs}s`
  const m = Math.floor(secs / 60)
  const s = secs % 60
  if (m < 60) return `${m}m ${s}s`
  const h = Math.floor(m / 60)
  return `${h}h ${m % 60}m`
}

function taskStatusLabel(t: Task): string {
  if (t.status === 'running') return 'En cours'
  if (t.exit_status === 'OK' || t.status === 'OK') return 'OK'
  if (t.exit_status) return String(t.exit_status)
  return String(t.status || '—')
}

function taskStatusBadgeClass(t: Task): string {
  if (t.status === 'running') return 'bg-blue-lt text-blue'
  if (t.exit_status === 'OK' || t.status === 'OK') return 'bg-success-lt text-success'
  if (t.exit_status) return 'bg-danger-lt text-danger'
  return 'bg-secondary-lt text-secondary'
}
</script>

<style scoped>
.task-status-badge {
  max-width: 11rem;
  display: inline-block;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: bottom;
}
</style>
