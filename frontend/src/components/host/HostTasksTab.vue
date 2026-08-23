<template>
  <div>
    <div class="d-flex justify-content-between align-items-center mb-3">
      <div
        v-if="tasksError"
        class="alert alert-danger mb-0 flex-fill me-3"
      >
        {{ tasksError }}
      </div>
      <div
        v-else
        class="flex-fill"
      />
      <button
        v-if="canRunApt"
        type="button"
        class="btn btn-primary"
        @click="openCreateTask"
      >
        Nouvelle tâche
      </button>
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
          :icon="IconClock"
          title="Aucune tâche planifiée"
          subtitle="Automatisez vos opérations en créant une tâche planifiée."
          :cta-label="canRunApt ? 'Nouvelle tâche' : ''"
          @cta="openCreateTask"
        />
      </div>
      <div
        v-else
        class="table-responsive"
      >
        <table class="table table-vcenter card-table mb-0">
          <thead>
            <tr>
              <th>
                <SortableHeader
                  label="Nom"
                  :active="sortKey === 'name'"
                  :direction="sortDir"
                  @toggle="toggleSort('name')"
                />
              </th>
              <th>Module / Action</th>
              <th>Planification</th>
              <th>
                <SortableHeader
                  label="Prochaine exécution"
                  :active="sortKey === 'next_run_at'"
                  :direction="sortDir"
                  @toggle="toggleSort('next_run_at')"
                />
              </th>
              <th>Dernier résultat</th>
              <th>Activée</th>
              <th />
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="task in sortedTasks"
              :key="task.id"
              :class="{ 'opacity-60': !task.enabled && !isManualOnly(task) }"
            >
              <td>{{ task.name }}</td>
              <td>
                <span class="badge bg-blue-lt me-1">{{ task.module }}</span>
                <span class="text-secondary small">{{ task.action }}</span>
                <span
                  v-if="task.target"
                  class="text-muted small ms-1"
                >- {{ task.target }}</span>
              </td>
              <td>
                <span
                  v-if="isManualOnly(task)"
                  class="badge bg-secondary-lt text-secondary"
                >Manuel</span>
                <template v-else>
                  <code class="small">{{ task.cron_expression }}</code>
                  <span
                    v-if="describeCron(task.cron_expression)"
                    class="text-muted small ms-1"
                  >- {{ describeCron(task.cron_expression) }}</span>
                </template>
              </td>
              <td>
                <span v-if="task.next_run_at && !isManualOnly(task)">{{ formatTaskDate(task.next_run_at) }}</span>
                <span
                  v-else
                  class="text-muted"
                >-</span>
              </td>
              <td>
                <span
                  v-if="task.last_run_status"
                  :class="getExecutionStateClass(task.last_run_status)"
                >
                  {{ commandStatusLabel(task.last_run_status) }}
                  <span
                    v-if="task.last_run_at"
                    class="ms-1 text-muted small"
                  >{{ formatTaskDate(task.last_run_at) }}</span>
                </span>
                <span
                  v-else
                  class="text-muted"
                >jamais</span>
              </td>
              <td>
                <input
                  v-if="canRunApt && !isManualOnly(task)"
                  type="checkbox"
                  class="form-check-input"
                  :checked="task.enabled"
                  @change="toggleTask(task)"
                >
                <span
                  v-else-if="isManualOnly(task)"
                  class="text-muted small"
                >-</span>
                <span v-else>{{ task.enabled ? 'Oui' : 'Non' }}</span>
              </td>
              <td class="text-end">
                <div class="d-flex gap-1 justify-content-end">
                  <button
                    v-if="task.last_command_id"
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-secondary"
                    title="Voir les logs"
                    @click="openTaskLogs(task)"
                  >
                    <IconList
                      :size="16"
                      class="icon icon-sm"
                    />
                  </button>
                  <button
                    v-if="canRunApt"
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-success"
                    :disabled="taskRunningId === task.id"
                    title="Exécuter"
                    aria-label="Exécuter la tâche maintenant"
                    @click="runTaskNow(task)"
                  >
                    <span
                      v-if="taskRunningId === task.id"
                      class="spinner-border spinner-border-sm"
                    />
                    <IconPlayerPlay
                      v-else
                      :size="16"
                      class="icon icon-sm"
                    />
                  </button>
                  <button
                    v-if="canRunApt"
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-secondary"
                    title="Modifier"
                    aria-label="Modifier la tâche"
                    @click="openEditTask(task)"
                  >
                    <IconPencil
                      :size="16"
                      class="icon icon-sm"
                    />
                  </button>
                  <button
                    v-if="canRunApt"
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-danger"
                    title="Supprimer"
                    aria-label="Supprimer la tâche"
                    @click="confirmDeleteTask(task)"
                  >
                    <IconTrash
                      :size="16"
                      class="icon icon-sm"
                    />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>

  <Teleport to="body">
    <template v-if="showTaskModal">
      <div
        ref="modalRef"
        class="modal modal-blur fade show d-block"
        tabindex="-1"
        role="dialog"
        aria-modal="true"
      >
        <div class="modal-dialog modal-dialog-centered modal-lg">
          <div class="modal-content">
            <div class="modal-header">
              <h5 class="modal-title">
                {{ editingTask ? 'Modifier la tâche' : 'Nouvelle tâche planifiée' }}
              </h5>
              <button
                type="button"
                class="btn-close"
                @click="closeTaskModal"
              />
            </div>
            <div class="modal-body">
              <div
                v-if="taskModalError"
                class="alert alert-danger"
              >
                {{ taskModalError }}
              </div>
              <div class="mb-3">
                <label class="form-label">Nom</label>
                <input
                  v-model="taskForm.name"
                  type="text"
                  class="form-control"
                  placeholder="Mise a jour APT hebdomadaire"
                >
              </div>
              <div class="mb-3">
                <DispatchStepEditor
                  v-model:module="taskForm.module"
                  v-model:action="taskForm.action"
                  v-model:target="taskForm.target"
                  :host-id="String(props.hostId)"
                  :actions-for-module="scheduledTaskActions"
                  :target-config="scheduledTaskTargetConfig"
                  :modules="availableModules"
                  :show-host="false"
                />
              </div>
              <div class="mb-3">
                <label class="form-check form-switch">
                  <input
                    v-model="taskManualOnly"
                    type="checkbox"
                    class="form-check-input"
                  >
                  <span class="form-check-label">Exécution manuelle uniquement (pas de planification automatique)</span>
                </label>
              </div>
              <div
                v-if="!taskManualOnly"
                class="mb-3"
              >
                <CronBuilder v-model="taskForm.cron_expression" />
              </div>
              <div
                v-if="!taskManualOnly"
                class="form-check form-switch mb-1"
              >
                <input
                  id="taskEnabled"
                  v-model="taskForm.enabled"
                  type="checkbox"
                  class="form-check-input"
                >
                <label
                  class="form-check-label"
                  for="taskEnabled"
                >Activee</label>
              </div>
            </div>
            <div class="modal-footer">
              <button
                type="button"
                class="btn btn-outline-secondary"
                @click="closeTaskModal"
              >
                Annuler
              </button>
              <button
                type="button"
                class="btn btn-primary"
                :disabled="taskSaving"
                @click="saveTask"
              >
                <span
                  v-if="taskSaving"
                  class="spinner-border spinner-border-sm me-1"
                />
                {{ editingTask ? 'Enregistrer' : 'Créer' }}
              </button>
            </div>
          </div>
        </div>
      </div>
      <div class="modal-backdrop fade show" />
    </template>

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
import { computed, ref, watch } from 'vue'
import { IconClock, IconList, IconPencil, IconPlayerPlay, IconTrash } from '@tabler/icons-vue'
import CronBuilder from '../CronBuilder.vue'
import DispatchStepEditor from '../DispatchStepEditor.vue'
import SortableHeader from '../common/SortableHeader.vue'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import apiClient from '../../api'
import { useConfirmDialog } from '../../composables/useConfirmDialog'
import { useDateFormatter } from '../../composables/useDateFormatter'
import { useToast } from '../../composables/useToast'
import { useModalChrome } from '../../composables/useModalChrome'
import { usePendingCommand } from '../../composables/usePendingCommand'
import { MANUAL_SENTINEL, isManualOnly, describeCron } from '../../utils/cron'
import { getApiErrorMessage } from '../../api/client'
import { getExecutionStateClass } from '../../utils/statusClasses'
import { commandStatusLabel } from '../../utils/commandStatus'
import { SCHEDULED_TASK_MODULES, availableScheduledTaskModules, scheduledTaskActions, scheduledTaskTargetConfig } from '../../utils/scheduledTaskDispatch'

interface Task {
  id: string | number
  name: string
  module: string
  action: string
  target?: string
  cron_expression: string
  enabled: boolean
  last_command_id?: string | number
  last_run_status?: string | null
  last_run_at?: string | null
  next_run_at?: string | null
}

interface TaskForm {
  name: string
  module: string
  action: string
  target: string
  cron_expression: string
  enabled: boolean
}

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
  collectors?: Record<string, boolean>
}>(), {
  canRunApt: false,
  active: false,
  collectors: undefined,
})

const dialog = useConfirmDialog()
const pendingCommand = usePendingCommand()
const { formatExactDate: formatTaskDate } = useDateFormatter()
const tasks = ref<Task[]>([])
type TaskSortKey = 'name' | 'next_run_at'
const sortKey = ref<TaskSortKey>('name')
const sortDir = ref<'asc' | 'desc'>('asc')

function toggleSort(key: TaskSortKey): void {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
    return
  }
  sortKey.value = key
  sortDir.value = 'asc'
}

const sortedTasks = computed(() => {
  const dir = sortDir.value === 'asc' ? 1 : -1
  return [...tasks.value].sort((a, b) => {
    if (sortKey.value === 'next_run_at') {
      const av = a.next_run_at ? new Date(a.next_run_at).getTime() : 0
      const bv = b.next_run_at ? new Date(b.next_run_at).getTime() : 0
      return (av - bv) * dir
    }
    return (a.name || '').toLowerCase().localeCompare((b.name || '').toLowerCase()) * dir
  })
})

const tasksLoading = ref(false)
const tasksError = ref('')
const taskRunningId = ref<string | number | null>(null)
const { value: taskRunResult, showToast: showTaskRunResult } = useToast<TaskRunResult | null>(null)
const showTaskModal = ref(false)
const modalRef = ref<HTMLElement | null>(null)
useModalChrome(modalRef, () => showTaskModal.value, { onClose: closeTaskModal })
const editingTask = ref<Task | null>(null)
const taskSaving = ref(false)
const taskModalError = ref('')
const taskManualOnly = ref(false)
const taskForm = ref<TaskForm>({ name: '', module: 'apt', action: 'update', target: '', cron_expression: '0 3 * * 0', enabled: true })

// Narrowed to what this host's agent actually reports as active (collectors)
// — a host with e.g. collect_docker off shouldn't offer that module. Always
// keeps the currently selected module visible too, so editing an existing
// task whose collector was disabled afterwards doesn't silently blank it.
const availableModules = computed(() => {
  const filtered = availableScheduledTaskModules(props.collectors)
  if (filtered.some((m) => m.value === taskForm.value.module)) return filtered
  const current = SCHEDULED_TASK_MODULES.find((m) => m.value === taskForm.value.module)
  return current ? [...filtered, current] : filtered
})

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

watch(taskManualOnly, (val) => {
  if (val) {
    taskForm.value.enabled = false
    taskForm.value.cron_expression = MANUAL_SENTINEL
  } else {
    taskForm.value.enabled = true
    if (taskForm.value.cron_expression === MANUAL_SENTINEL) {
      taskForm.value.cron_expression = '0 3 * * 0'
    }
  }
})

async function loadTasks(): Promise<void> {
  tasksLoading.value = true
  tasksError.value = ''
  try {
    const { data } = await apiClient.getScheduledTasks(String(props.hostId))
    tasks.value = data
  } catch (e: unknown) {
    tasksError.value = getApiErrorMessage(e, 'Erreur de chargement')
  } finally {
    tasksLoading.value = false
  }
}

function openCreateTask(): void {
  editingTask.value = null
  taskManualOnly.value = false
  const defaultModule = availableScheduledTaskModules(props.collectors)[0]?.value || 'apt'
  taskForm.value = {
    name: '', module: defaultModule, action: scheduledTaskActions(defaultModule)[0]?.value || '',
    target: '', cron_expression: '0 3 * * 0', enabled: true,
  }
  taskModalError.value = ''
  showTaskModal.value = true
}

function openEditTask(task: Task): void {
  editingTask.value = task
  taskManualOnly.value = isManualOnly(task)
  taskForm.value = { name: task.name, module: task.module, action: task.action, target: task.target || '', cron_expression: task.cron_expression, enabled: task.enabled }
  taskModalError.value = ''
  showTaskModal.value = true
}

function closeTaskModal(): void {
  showTaskModal.value = false
}

async function saveTask(): Promise<void> {
  if (!taskForm.value.name || (!taskForm.value.action && taskForm.value.module !== 'custom')) {
    taskModalError.value = 'Nom et action sont obligatoires.'
    return
  }
  if (!taskManualOnly.value && !taskForm.value.cron_expression) {
    taskModalError.value = 'Expression cron obligatoire.'
    return
  }
  taskSaving.value = true
  taskModalError.value = ''
  try {
    if (editingTask.value) {
      await apiClient.updateScheduledTask(String(editingTask.value.id), taskForm.value)
    } else {
      await apiClient.createScheduledTask(String(props.hostId), taskForm.value)
    }
    closeTaskModal()
    await loadTasks()
  } catch (e: unknown) {
    const data = (e as { response?: { data?: { error?: string; warning?: string } } }).response?.data
    taskModalError.value = data?.error || data?.warning || 'Erreur lors de la sauvegarde'
  } finally {
    taskSaving.value = false
  }
}

async function toggleTask(task: Task): Promise<void> {
  try {
    await apiClient.updateScheduledTask(String(task.id), { ...task, enabled: !task.enabled })
    await loadTasks()
  } catch (e: unknown) {
    tasksError.value = getApiErrorMessage(e, 'Erreur')
  }
}

async function runTaskNow(task: Task): Promise<void> {
  taskRunningId.value = task.id
  try {
    const { data } = await apiClient.runScheduledTask(String(task.id))
    showTaskRunResult({ id: data.command_id, name: task.name }, 5000)
    emit('open-command', {
      id: data.command_id,
      module: task.module,
      action: task.action,
      target: task.target,
      status: 'pending',
      output: '',
    })
    emit('history-changed')
    await loadTasks()
    await pendingCommand.track(data.command_id)
    await loadTasks()
  } catch (e: unknown) {
    tasksError.value = getApiErrorMessage(e, 'Erreur')
  } finally {
    taskRunningId.value = null
  }
}

function openTaskLogs(task: Task): void {
  if (!task.last_command_id) return
  emit('open-command', {
    id: task.last_command_id,
    module: task.module,
    action: task.action,
    target: task.target,
    status: task.last_run_status || 'completed',
    output: '',
  })
}

async function confirmDeleteTask(task: Task): Promise<void> {
  const confirmed = await dialog.confirm({
    title: 'Supprimer la tâche',
    message: `Supprimer la tâche "${task.name}" ?\nCette action est irréversible.`,
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await apiClient.deleteScheduledTask(String(task.id))
    await loadTasks()
  } catch (e: unknown) {
    tasksError.value = getApiErrorMessage(e, 'Erreur de suppression')
  }
}
</script>

