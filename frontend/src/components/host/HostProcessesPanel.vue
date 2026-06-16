<template>
  <div
    v-if="canRun"
    class="card mt-4"
  >
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">
        Processus
      </h3>
      <div class="d-flex align-items-center gap-2">
        <label
          for="process-filter"
          class="visually-hidden"
        >Filtrer processus</label>
        <input
          id="process-filter"
          v-model="processFilter"
          type="text"
          class="form-control form-control-sm"
          placeholder="Filtrer..."
          style="width: 160px;"
        >
        <button
          class="btn btn-sm btn-outline-secondary"
          :disabled="loading"
          @click="loadProcesses"
        >
          <span
            v-if="loading"
            class="spinner-border spinner-border-sm me-1"
          />
          {{ loading ? 'Chargement...' : (processes.length ? 'Actualiser' : 'Charger') }}
        </button>
      </div>
    </div>
    <div
      v-if="loading"
      class="card-body"
    >
      <LoadingSkeleton
        variant="list"
        :lines="3"
      />
    </div>
    <div
      v-if="error"
      class="card-body pb-0"
    >
      <div class="alert alert-danger mb-0">
        {{ error }}
      </div>
    </div>
    <div
      v-if="!processes.length && !loading && !error"
      class="card-body"
    >
      <div class="text-secondary small">
        Cliquez sur "Charger" pour afficher les processus actifs de cet hôte.
      </div>
    </div>
    <div
      v-if="filteredProcesses.length"
      class="table-responsive"
    >
      <table
        class="table table-vcenter table-hover card-table mb-0"
        style="font-size: 0.82rem;"
      >
        <thead>
          <tr>
            <th
              class="cursor-pointer"
              @click="sortBy('pid')"
            >
              PID <span class="text-secondary">{{ sortIcon('pid') }}</span>
            </th>
            <th
              class="cursor-pointer"
              @click="sortBy('name')"
            >
              Nom <span class="text-secondary">{{ sortIcon('name') }}</span>
            </th>
            <th>Utilisateur</th>
            <th
              class="cursor-pointer"
              @click="sortBy('cpu_pct')"
            >
              CPU% <span class="text-secondary">{{ sortIcon('cpu_pct') }}</span>
            </th>
            <th
              class="cursor-pointer"
              @click="sortBy('mem_pct')"
            >
              MEM% <span class="text-secondary">{{ sortIcon('mem_pct') }}</span>
            </th>
            <th
              class="cursor-pointer"
              @click="sortBy('mem_rss_kb')"
            >
              RSS (KB) <span class="text-secondary">{{ sortIcon('mem_rss_kb') }}</span>
            </th>
            <th>État</th>
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
                :class="proc.state.startsWith('S') || proc.state.startsWith('I') ? 'bg-secondary-lt text-secondary' : proc.state.startsWith('R') ? 'bg-success-lt text-success' : proc.state.startsWith('Z') ? 'bg-danger-lt text-danger' : 'bg-yellow-lt text-yellow'"
              >
                {{ proc.state }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div
      v-if="processes.length"
      class="card-footer text-secondary small"
    >
      {{ filteredProcesses.length }} / {{ processes.length }} processus
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import { useHostProcesses, type HostProcess } from '../../composables/useHostProcesses'

type SortKey = keyof HostProcess

const props = withDefaults(defineProps<{
  hostId: string
  canRun?: boolean
}>(), {
  canRun: false,
})

const emit = defineEmits<{
  (e: 'history-changed'): void
}>()

const { processes, loading, error, load } = useHostProcesses(props.hostId)
const processFilter = ref('')
const sortKey = ref<SortKey>('cpu_pct')
const sortDir = ref(-1)

const filteredProcesses = computed(() => {
  let list = processes.value
  if (processFilter.value) {
    const q = processFilter.value.toLowerCase()
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

function sortIcon(key: SortKey): string {
  if (sortKey.value !== key) return ''
  return sortDir.value === -1 ? '▼' : '▲'
}

async function loadProcesses(): Promise<void> {
  await load()
  emit('history-changed')
}
</script>
