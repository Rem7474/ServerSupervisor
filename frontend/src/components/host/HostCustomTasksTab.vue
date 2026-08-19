<template>
  <div>
    <div
      v-if="tasksError"
      class="alert alert-danger"
    >
      {{ tasksError }}
    </div>
    <div class="card">
      <div
        v-if="tasksLoading"
        class="card-body"
      >
        <LoadingSkeleton variant="table" />
      </div>
      <div
        v-else-if="!tasks.length"
        class="card-body"
      >
        <EmptyState
          :icon="IconTerminal2"
          title="Aucune tâche personnalisée"
          subtitle="Déclarez des tâches dans le tasks.yaml de l'agent pour les exécuter ici à la demande."
        />
      </div>
      <div
        v-else
        class="table-responsive"
      >
        <table class="table table-vcenter card-table mb-0">
          <thead>
            <tr>
              <th>Nom</th>
              <th>ID</th>
              <th class="text-end" />
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="task in tasks"
              :key="task.id"
            >
              <td>{{ task.name }}</td>
              <td>
                <code class="small">{{ task.id }}</code>
              </td>
              <td class="text-end">
                <button
                  v-if="canRunApt"
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-success"
                  :disabled="runningId === task.id"
                  title="Exécuter"
                  aria-label="Exécuter la tâche maintenant"
                  @click="runTaskNow(task)"
                >
                  <span
                    v-if="runningId === task.id"
                    class="spinner-border spinner-border-sm"
                  />
                  <IconPlayerPlay
                    v-else
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
  </div>

  <Teleport to="body">
    <div
      v-if="taskRunResult"
      class="position-fixed bottom-0 end-0 p-3"
      style="z-index: var(--tblr-zindex-toast, 1090);"
    >
      <div class="toast show align-items-center text-bg-success border-0">
        <div class="d-flex">
          <div class="toast-body">
            <strong>{{ taskRunResult.name }}</strong> déclenchée — commande <code>{{ taskRunResult.id }}</code>
          </div>
          <button
            type="button"
            class="btn-close me-2 m-auto"
            @click="taskRunResult = null"
          />
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { IconPlayerPlay, IconTerminal2 } from '@tabler/icons-vue'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import apiClient from '../../api'
import { useToast } from '../../composables/useToast'
import { usePendingCommand } from '../../composables/usePendingCommand'
import { getApiErrorMessage } from '../../api/client'
import type { CustomTaskSummary } from '../../types/task'

interface TaskRunResult {
  id: string | number
  name: string
}

const emit = defineEmits<{
  (e: 'open-command', payload: Record<string, unknown>): void
  (e: 'tasks-count', count: number): void
  (e: 'history-changed'): void
}>()

const props = withDefaults(defineProps<{
  hostId: string | number
  canRunApt?: boolean
  active?: boolean
}>(), {
  canRunApt: false,
  active: false,
})

const pendingCommand = usePendingCommand()
const tasks = ref<CustomTaskSummary[]>([])
const tasksLoading = ref(false)
const tasksError = ref('')
const runningId = ref<string | null>(null)
const { value: taskRunResult, showToast: showTaskRunResult } = useToast<TaskRunResult | null>(null)

watch(
  tasks,
  (value) => {
    emit('tasks-count', value.length)
  },
  { deep: true }
)

watch(
  () => props.active,
  (active) => {
    if (active && !tasks.value.length && !tasksLoading.value) {
      loadTasks()
    }
  },
  { immediate: true }
)

async function loadTasks(): Promise<void> {
  tasksLoading.value = true
  tasksError.value = ''
  try {
    const { data } = await apiClient.getHostCustomTasks(String(props.hostId))
    tasks.value = data || []
  } catch (e: unknown) {
    tasksError.value = getApiErrorMessage(e, 'Erreur de chargement')
  } finally {
    tasksLoading.value = false
  }
}

async function runTaskNow(task: CustomTaskSummary): Promise<void> {
  runningId.value = task.id
  try {
    const { data } = await apiClient.runCustomTask(String(props.hostId), task.id)
    showTaskRunResult({ id: data.command_id, name: task.name }, 5000)
    emit('open-command', {
      id: data.command_id,
      module: 'custom',
      action: '',
      target: task.id,
      status: 'pending',
      output: '',
    })
    emit('history-changed')
    await pendingCommand.track(data.command_id)
  } catch (e: unknown) {
    tasksError.value = getApiErrorMessage(e, 'Erreur')
  } finally {
    runningId.value = null
  }
}
</script>
