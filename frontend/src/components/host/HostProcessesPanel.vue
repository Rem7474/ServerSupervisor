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
        <button
          type="button"
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
      v-if="processes.length && !loading"
      class="card-body"
    >
      <ProcessesTable :processes="processes" />
    </div>
  </div>
</template>

<script setup lang="ts">
import LoadingSkeleton from '../LoadingSkeleton.vue'
import ProcessesTable from './ProcessesTable.vue'
import { useHostProcesses } from '../../composables/useHostProcesses'

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

async function loadProcesses(): Promise<void> {
  await load()
  emit('history-changed')
}
</script>
