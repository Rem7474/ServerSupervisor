<template>
  <div>
    <div class="d-flex justify-content-end mb-2">
      <label
        for="process-table-filter"
        class="visually-hidden"
      >{{ t('host.filterProcessesLabel') }}</label>
      <input
        id="process-table-filter"
        v-model="filter"
        type="text"
        class="form-control form-control-sm"
        :placeholder="t('host.filterPlaceholder')"
        style="width: 160px;"
      >
    </div>
    <div
      v-if="filteredProcesses.length"
      class="table-responsive scroll-table"
    >
      <table
        class="table table-vcenter card-table mb-0"
        style="font-size: 0.82rem;"
      >
        <thead>
          <tr>
            <th>
              <SortableHeader
                label="PID"
                :active="sortKey === 'pid'"
                :direction="sortDirLabel"
                @toggle="sortBy('pid')"
              />
            </th>
            <th>
              <SortableHeader
                :label="t('host.nameColumn')"
                :active="sortKey === 'name'"
                :direction="sortDirLabel"
                @toggle="sortBy('name')"
              />
            </th>
            <th>{{ t('host.userColumn') }}</th>
            <th>
              <SortableHeader
                label="CPU%"
                :active="sortKey === 'cpu_pct'"
                :direction="sortDirLabel"
                @toggle="sortBy('cpu_pct')"
              />
            </th>
            <th>
              <SortableHeader
                label="MEM%"
                :active="sortKey === 'mem_pct'"
                :direction="sortDirLabel"
                @toggle="sortBy('mem_pct')"
              />
            </th>
            <th>
              <SortableHeader
                label="RSS (KB)"
                :active="sortKey === 'mem_rss_kb'"
                :direction="sortDirLabel"
                @toggle="sortBy('mem_rss_kb')"
              />
            </th>
            <th>{{ t('host.stateColumn') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="proc in filteredProcesses"
            :key="proc.pid"
          >
            <td class="text-secondary font-monospace">
              {{ proc.pid }}
            </td>
            <td class="fw-semibold font-monospace">
              {{ proc.name }}
            </td>
            <td class="text-secondary">
              {{ proc.user }}
            </td>
            <td>
              <span :class="proc.cpu_pct > 50 ? 'text-danger fw-bold' : proc.cpu_pct > 10 ? 'text-warning' : ''">
                {{ proc.cpu_pct.toFixed(1) }}%
              </span>
            </td>
            <td>
              <span :class="proc.mem_pct > 50 ? 'text-danger fw-bold' : proc.mem_pct > 20 ? 'text-warning' : ''">
                {{ proc.mem_pct.toFixed(1) }}%
              </span>
            </td>
            <td class="text-secondary">
              {{ proc.mem_rss_kb.toLocaleString() }}
            </td>
            <td>
              <span
                class="badge"
                :class="proc.state.startsWith('S') || proc.state.startsWith('I') ? 'bg-secondary-lt text-secondary' : proc.state.startsWith('R') ? 'bg-success-lt text-success' : proc.state.startsWith('Z') ? 'bg-danger-lt text-danger' : 'bg-warning-lt text-warning'"
              >
                {{ proc.state }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div
      v-else
      class="text-secondary small"
    >
      {{ t('host.noProcesses') }}
    </div>
    <div
      v-if="processes.length"
      class="text-secondary small mt-2"
    >
      {{ t('host.filteredOfTotalProcesses', { filtered: filteredProcesses.length, total: processes.length }) }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import SortableHeader from '../common/SortableHeader.vue'
import type { HostProcess } from '../../composables/useHostProcesses'

type SortKey = keyof HostProcess

const { t } = useI18n()

const props = defineProps<{
  processes: HostProcess[]
}>()

const filter = ref('')
const sortKey = ref<SortKey>('cpu_pct')
const sortDir = ref(-1)
// SortableHeader only knows 'asc'/'desc' — sortDir itself stays the original
// signed multiplier so the sort formula below (and its direction for each
// column) is untouched by this presentational-only migration.
const sortDirLabel = computed<'asc' | 'desc'>(() => (sortDir.value === -1 ? 'desc' : 'asc'))

const filteredProcesses = computed(() => {
  let list = props.processes
  if (filter.value) {
    const q = filter.value.toLowerCase()
    list = list.filter((p) => p.name.toLowerCase().includes(q) || p.user.toLowerCase().includes(q))
  }
  return [...list].sort((a, b) => {
    const av = a[sortKey.value]
    const bv = b[sortKey.value]
    if (typeof av === 'string') return sortDir.value * av.localeCompare(String(bv))
    return sortDir.value * ((bv as number) - (av as number))
  })
})

function sortBy(key: SortKey): void {
  if (sortKey.value === key) {
    sortDir.value *= -1
  } else {
    sortKey.value = key
    sortDir.value = key === 'name' || key === 'user' ? 1 : -1
  }
}
</script>
