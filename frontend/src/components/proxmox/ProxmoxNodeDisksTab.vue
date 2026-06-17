<template>
  <div class="table-responsive">
    <table class="table table-vcenter card-table">
      <thead>
        <tr>
          <th>
            <SortableHeader
              label="Périphérique"
              :active="sortKey === 'dev_path'"
              :direction="sortDir"
              @toggle="toggleSort('dev_path')"
            />
          </th>
          <th>
            <SortableHeader
              label="Modèle"
              :active="sortKey === 'model'"
              :direction="sortDir"
              @toggle="toggleSort('model')"
            />
          </th>
          <th>
            <SortableHeader
              label="Type"
              :active="sortKey === 'disk_type'"
              :direction="sortDir"
              @toggle="toggleSort('disk_type')"
            />
          </th>
          <th>
            <SortableHeader
              label="Taille"
              :active="sortKey === 'size_bytes'"
              :direction="sortDir"
              @toggle="toggleSort('size_bytes')"
            />
          </th>
          <th>
            <SortableHeader
              label="Santé SMART"
              :active="sortKey === 'health'"
              :direction="sortDir"
              @toggle="toggleSort('health')"
            />
          </th>
          <th>
            <SortableHeader
              label="Usure SSD"
              :active="sortKey === 'wearout'"
              :direction="sortDir"
              @toggle="toggleSort('wearout')"
            />
          </th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="!sortedDisks.length">
          <td
            colspan="6"
            class="text-center text-muted py-4"
          >
            Aucun disque détecté sur ce nœud.
          </td>
        </tr>
        <tr
          v-for="d in sortedDisks"
          :key="d.id"
        >
          <td class="fw-medium font-monospace">
            {{ d.dev_path }}
          </td>
          <td>
            {{ d.model || '—' }}<div class="text-muted small">
              {{ d.serial }}
            </div>
          </td>
          <td><span class="badge bg-secondary-lt text-secondary text-uppercase">{{ d.disk_type || '?' }}</span></td>
          <td>{{ formatBytes(d.size_bytes) }}</td>
          <td>
            <span
              v-if="d.health === 'PASSED'"
              class="badge bg-success-lt text-success"
            >PASSED</span>
            <span
              v-else-if="d.health === 'FAILED'"
              class="badge bg-danger-lt text-danger"
            >FAILED</span>
            <span
              v-else
              class="badge bg-secondary-lt text-secondary"
            >{{ d.health }}</span>
          </td>
          <td>
            <template v-if="d.wearout >= 0">
              <div class="d-flex align-items-center gap-2">
                <div class="progress progress-xs flex-grow-1 proxmox-progress-min-60">
                  <div
                    class="progress-bar"
                    :class="wearoutColor(d.wearout)"
                    :style="`width:${d.wearout}%`"
                  />
                </div>
                <span class="text-muted small">{{ d.wearout }}%</span>
              </div>
            </template>
            <span
              v-else
              class="text-muted"
            >—</span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import SortableHeader from '../common/SortableHeader.vue'
import type { ProxmoxDisk } from '../../types/proxmox'

const props = defineProps<{ disks: ProxmoxDisk[] }>()

const sortKey = ref('dev_path')
const sortDir = ref<'asc' | 'desc'>('asc')

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

const sortedDisks = computed(() => {
  const list = [...(props.disks ?? [])]
  list.sort((a, b) => compareValues(
    (a as unknown as Record<string, unknown>)?.[sortKey.value],
    (b as unknown as Record<string, unknown>)?.[sortKey.value],
    sortDir.value,
  ))
  return list
})

function formatBytes(bytes: number): string {
  if (!bytes) return '0 B'
  const units = ['B', 'Ko', 'Mo', 'Go', 'To']
  let i = 0
  let v = bytes
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

function wearoutColor(wearout: number): string {
  if (wearout < 20) return 'bg-danger'
  if (wearout < 50) return 'bg-warning'
  return 'bg-success'
}
</script>

<style scoped>
.proxmox-progress-min-60 {
  min-width: 60px;
}
</style>
