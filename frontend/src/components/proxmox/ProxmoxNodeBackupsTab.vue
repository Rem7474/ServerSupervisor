<template>
  <div class="table-responsive scroll-table">
    <table class="table table-vcenter card-table">
      <thead>
        <tr>
          <th>
            <SortableHeader
              label="VM / CT"
              :active="sortKey === 'guest_name'"
              :direction="sortDir"
              @toggle="toggleSort('guest_name')"
            />
          </th>
          <th>
            <SortableHeader
              label="ID"
              :active="sortKey === 'vmid'"
              :direction="sortDir"
              @toggle="toggleSort('vmid')"
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
          <th class="text-end" />
        </tr>
      </thead>
      <tbody>
        <template v-if="loading && !runs.length">
          <tr>
            <td colspan="6">
              <LoadingSkeleton
                variant="table"
                :lines="3"
              />
            </td>
          </tr>
        </template>
        <template v-else-if="!runs.length">
          <tr>
            <td colspan="6">
              <EmptyState title="Aucune sauvegarde connue pour les VM/CT de ce nœud." />
            </td>
          </tr>
        </template>
        <tr
          v-for="r in sortedRuns"
          v-else
          :key="r.id"
        >
          <td class="fw-medium">
            {{ r.guest_name || `VM ${r.vmid}` }}
          </td>
          <td class="text-muted font-monospace">
            {{ r.vmid }}
          </td>
          <td class="text-muted small">
            {{ formatDate(r.start_time) }}
          </td>
          <td class="text-muted small">
            {{ runDuration(r) }}
          </td>
          <td>
            <span
              class="badge"
              :class="statusBadgeClass(r)"
            >{{ statusLabel(r) }}</span>
          </td>
          <td class="text-end">
            <button
              v-if="r.task_upid"
              type="button"
              class="btn btn-icon btn-sm btn-ghost-secondary"
              title="Voir les logs"
              aria-label="Voir les logs"
              @click="emit('view-logs', { upid: r.task_upid, action: 'backup', label: r.guest_name || String(r.vmid) })"
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
import { IconList } from '@tabler/icons-vue'
import SortableHeader from '../common/SortableHeader.vue'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import type { ProxmoxBackupRun } from '../../types/proxmox'
import { compareValues } from '../../utils/sort'

const props = defineProps<{
  runs: ProxmoxBackupRun[]
  loading?: boolean
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

function durationSeconds(r: ProxmoxBackupRun): number | null {
  if (!r.start_time) return null
  const startMs = new Date(r.start_time).getTime()
  if (!Number.isFinite(startMs)) return null
  const endMs = r.end_time
    ? new Date(r.end_time).getTime()
    : (r.status === 'running' ? Date.now() : null)
  if (endMs == null || !Number.isFinite(endMs)) return null
  return Math.max(0, Math.floor((endMs - startMs) / 1000))
}

const sortedRuns = computed(() => {
  const list = [...(props.runs ?? [])]
  list.sort((a, b) => {
    if (sortKey.value === 'duration') {
      return compareValues(durationSeconds(a), durationSeconds(b), sortDir.value)
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
  return new Date(iso).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })
}

function runDuration(r: ProxmoxBackupRun): string {
  const secs = durationSeconds(r)
  if (secs == null) return '—'
  if (secs < 60) return `${secs}s`
  const m = Math.floor(secs / 60)
  const s = secs % 60
  if (m < 60) return `${m}m ${s}s`
  const h = Math.floor(m / 60)
  return `${h}h ${m % 60}m`
}

function statusLabel(r: ProxmoxBackupRun): string {
  if (r.status === 'running') return 'En cours'
  if (r.exit_status === 'OK' || r.status === 'OK') return 'OK'
  if (r.exit_status) return String(r.exit_status)
  return String(r.status || '—')
}

function statusBadgeClass(r: ProxmoxBackupRun): string {
  if (r.status === 'running') return 'bg-primary-lt text-primary'
  if (r.exit_status === 'OK' || r.status === 'OK') return 'bg-success-lt text-success'
  if (r.exit_status) return 'bg-danger-lt text-danger'
  return 'bg-secondary-lt text-secondary'
}
</script>
