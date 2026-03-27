<template>
  <div>
    <div class="d-flex justify-content-between align-items-center mb-3">
      <div v-if="tasksError" class="alert alert-danger mb-0 flex-fill me-3">{{ tasksError }}</div>
      <div v-else class="flex-fill"></div>
      <button v-if="canRunApt" class="btn btn-primary" @click="openCreateTask">
        Nouvelle tache
      </button>
    </div>
    <div class="card">
      <div v-if="tasksLoading" class="card-body text-center py-5">
        <span class="spinner-border text-primary"></span>
      </div>
      <div v-else-if="!tasks.length" class="card-body text-center py-5">
        <svg xmlns="http://www.w3.org/2000/svg" class="icon mb-3 text-muted" width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
        </svg>
        <h3 class="mb-1">Aucune tache planifiee</h3>
        <p class="text-secondary mb-3">Automatisez vos operations en creant une tache planifiee.</p>
        <button v-if="canRunApt" class="btn btn-primary" @click="openCreateTask">Nouvelle tache</button>
      </div>
      <div v-else class="table-responsive">
        <table class="table table-vcenter table-hover card-table mb-0">
          <thead>
            <tr>
              <th>Nom</th>
              <th>Module / Action</th>
              <th>Planification</th>
              <th>Prochaine exécution</th>
              <th>Dernier resultat</th>
              <th>Activee</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="task in tasks" :key="task.id">
              <td>{{ task.name }}</td>
              <td>
                <span class="badge bg-blue-lt me-1">{{ task.module }}</span>
                <span class="text-secondary small">{{ task.action }}</span>
                <span v-if="task.target" class="text-muted small ms-1">- {{ task.target }}</span>
              </td>
              <td>
                <span v-if="isManualOnly(task)" class="badge bg-secondary-lt text-secondary">Manuel</span>
                <template v-else>
                  <code class="small">{{ task.cron_expression }}</code>
                  <span v-if="describeCron(task.cron_expression)" class="text-muted small ms-1">- {{ describeCron(task.cron_expression) }}</span>
                </template>
              </td>
              <td>
                <span v-if="task.next_run_at && !isManualOnly(task)">{{ formatTaskDate(task.next_run_at) }}</span>
                <span v-else class="text-muted">-</span>
              </td>
              <td>
                <span v-if="task.last_run_status"
                  :class="task.last_run_status === 'completed' ? 'badge bg-success-lt' : task.last_run_status === 'pending' || task.last_run_status === 'running' ? 'badge bg-warning-lt' : 'badge bg-danger-lt'">
                  {{ task.last_run_status }}
                  <span v-if="task.last_run_at" class="ms-1 text-muted small">{{ formatTaskDate(task.last_run_at) }}</span>
                </span>
                <span v-else class="text-muted">jamais</span>
              </td>
              <td>
                <input v-if="canRunApt && !isManualOnly(task)" type="checkbox" class="form-check-input"
                  :checked="task.enabled" @change="toggleTask(task)">
                <span v-else-if="isManualOnly(task)" class="text-muted small">-</span>
                <span v-else>{{ task.enabled ? 'Oui' : 'Non' }}</span>
              </td>
              <td class="text-end">
                <div class="d-flex gap-1 justify-content-end">
                  <button v-if="task.last_command_id" class="btn btn-sm btn-ghost-secondary" title="Voir les logs" @click="openTaskLogs(task)">
                    <svg class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M4 6l16 0" /><path d="M4 12l16 0" /><path d="M4 18l12 0" /></svg>
                  </button>
                  <button v-if="canRunApt" class="btn btn-sm btn-outline-primary"
                    :disabled="taskRunningId === task.id" @click="runTaskNow(task)">
                    <span v-if="taskRunningId === task.id" class="spinner-border spinner-border-sm"></span>
                    <span v-else>Executer</span>
                  </button>
                  <button v-if="canRunApt" class="btn btn-sm btn-outline-secondary" @click="openEditTask(task)">Modifier</button>
                  <button v-if="canRunApt" class="btn btn-sm btn-outline-danger" @click="confirmDeleteTask(task)">Supprimer</button>
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
      <div class="modal modal-blur fade show d-block" tabindex="-1" role="dialog" aria-modal="true">
        <div class="modal-dialog modal-dialog-centered modal-lg">
          <div class="modal-content">
            <div class="modal-header">
              <h5 class="modal-title">{{ editingTask ? 'Modifier la tache' : 'Nouvelle tache planifiee' }}</h5>
              <button type="button" class="btn-close" @click="closeTaskModal"></button>
            </div>
            <div class="modal-body">
              <div v-if="taskModalError" class="alert alert-danger">{{ taskModalError }}</div>
              <div class="mb-3">
                <label class="form-label">Nom</label>
                <input v-model="taskForm.name" type="text" class="form-control" placeholder="Mise a jour APT hebdomadaire">
              </div>
              <div class="row g-3 mb-3">
                <div class="col">
                  <label class="form-label">Module</label>
                  <select v-model="taskForm.module" class="form-select" @change="onTaskModuleChange">
                    <option value="apt">apt</option>
                    <option value="docker">docker</option>
                    <option value="systemd">systemd</option>
                    <option value="journal">journal</option>
                    <option value="processes">processes</option>
                    <option value="custom">custom</option>
                  </select>
                </div>
                <div class="col">
                  <label class="form-label">Action</label>
                  <select v-if="taskModuleActions[taskForm.module]" v-model="taskForm.action" class="form-select">
                    <option v-for="a in taskModuleActions[taskForm.module]" :key="a" :value="a">{{ a }}</option>
                  </select>
                  <input v-else v-model="taskForm.action" type="text" class="form-control" placeholder="run">
                </div>
              </div>
              <div v-if="taskForm.module !== 'apt' && taskForm.module !== 'processes'" class="mb-3">
                <label class="form-label">{{ taskForm.module === 'custom' ? 'Tache (tasks.yaml)' : 'Cible' }}</label>
                <template v-if="taskForm.module === 'custom'">
                  <select v-if="customTaskOptions.length" v-model="taskForm.target" class="form-select">
                    <option value="" disabled>-- Selectionner une tache --</option>
                    <option v-for="t in customTaskOptions" :key="t.id" :value="t.id">{{ t.name }} ({{ t.id }})</option>
                  </select>
                  <template v-else>
                    <input v-model="taskForm.target" type="text" class="form-control" placeholder="cleanup_logs">
                    <div class="form-hint">Aucune tache detectee dans <code>tasks.yaml</code> - saisissez l'ID manuellement.</div>
                  </template>
                </template>
                <input v-else v-model="taskForm.target" type="text" class="form-control" placeholder="nginx.service">
              </div>
              <div class="mb-3">
                <label class="form-check form-switch">
                  <input v-model="taskManualOnly" type="checkbox" class="form-check-input">
                  <span class="form-check-label">Exécution manuelle uniquement (pas de planification automatique)</span>
                </label>
              </div>
              <div v-if="!taskManualOnly" class="mb-3">
                <CronBuilder v-model="taskForm.cron_expression" />
              </div>
              <div class="form-check form-switch mb-1" v-if="!taskManualOnly">
                <input v-model="taskForm.enabled" type="checkbox" class="form-check-input" id="taskEnabled">
                <label class="form-check-label" for="taskEnabled">Activee</label>
              </div>
            </div>
            <div class="modal-footer">
              <button type="button" class="btn btn-outline-secondary" @click="closeTaskModal">Annuler</button>
              <button type="button" class="btn btn-primary" :disabled="taskSaving" @click="saveTask">
                <span v-if="taskSaving" class="spinner-border spinner-border-sm me-1"></span>
                {{ editingTask ? 'Enregistrer' : 'Creer' }}
              </button>
            </div>
          </div>
        </div>
      </div>
      <div class="modal-backdrop fade show"></div>
    </template>

    <div v-if="taskRunResult" class="position-fixed bottom-0 end-0 p-3" style="z-index: var(--tblr-zindex-toast, 1090);">
      <div class="toast show align-items-center text-bg-success border-0">
        <div class="d-flex">
          <div class="toast-body">
            <strong>{{ taskRunResult.name }}</strong> declenchee - commande <code>{{ taskRunResult.id }}</code>
          </div>
          <button type="button" class="btn-close btn-close-white me-2 m-auto" @click="taskRunResult = null"></button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, watch } from 'vue'
import CronBuilder from '../CronBuilder.vue'
import apiClient from '../../api'
import { useConfirmDialog } from '../../composables/useConfirmDialog'
import { useDateFormatter } from '../../composables/useDateFormatter'
import { useToast } from '../../composables/useToast'
import { MANUAL_SENTINEL, isManualOnly, describeCron } from '../../utils/cron'

const taskModuleActions = {
  apt: ['update', 'upgrade', 'dist-upgrade'],
  docker: ['start', 'stop', 'restart', 'logs', 'pull'],
  systemd: ['start', 'stop', 'restart', 'status', 'enable', 'disable'],
  journal: ['read'],
  processes: ['list'],
  custom: null,
}

const emit = defineEmits(['open-command', 'tasks-count', 'history-changed'])

const props = defineProps({
  hostId: {
    type: [String, Number],
    required: true,
  },
  canRunApt: {
    type: Boolean,
    default: false,
  },
  active: {
    type: Boolean,
    default: false,
  },
})

const dialog = useConfirmDialog()
const { formatExactDate: formatTaskDate } = useDateFormatter()
const tasks = ref([])
const tasksLoading = ref(false)
const tasksError = ref('')
const taskRunningId = ref(null)
const { value: taskRunResult, showToast: showTaskRunResult } = useToast(null)
const showTaskModal = ref(false)
const editingTask = ref(null)
const taskSaving = ref(false)
const taskModalError = ref('')
const taskManualOnly = ref(false)
const customTaskOptions = ref([])
const taskForm = ref({ name: '', module: 'apt', action: 'update', target: '', cron_expression: '0 3 * * 0', enabled: true })

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

async function loadTasks() {
  tasksLoading.value = true
  tasksError.value = ''
  try {
    const { data } = await apiClient.getScheduledTasks(props.hostId)
    tasks.value = data
  } catch (e) {
    tasksError.value = e.response?.data?.error || 'Erreur de chargement'
  } finally {
    tasksLoading.value = false
  }
}

async function loadCustomTasks() {
  try {
    const { data } = await apiClient.getHostCustomTasks(props.hostId)
    customTaskOptions.value = Array.isArray(data) ? data : []
    if (customTaskOptions.value.length && !taskForm.value.target) {
      taskForm.value.target = customTaskOptions.value[0].id
    }
  } catch {
    customTaskOptions.value = []
  }
}

async function onTaskModuleChange() {
  const actions = taskModuleActions[taskForm.value.module]
  taskForm.value.action = actions ? actions[0] : 'run'
  if (taskForm.value.module === 'custom') await loadCustomTasks()
}

function openCreateTask() {
  editingTask.value = null
  taskManualOnly.value = false
  customTaskOptions.value = []
  taskForm.value = { name: '', module: 'apt', action: 'update', target: '', cron_expression: '0 3 * * 0', enabled: true }
  taskModalError.value = ''
  showTaskModal.value = true
}

async function openEditTask(task) {
  editingTask.value = task
  taskManualOnly.value = isManualOnly(task)
  customTaskOptions.value = []
  taskForm.value = { name: task.name, module: task.module, action: task.action, target: task.target, cron_expression: task.cron_expression, enabled: task.enabled }
  taskModalError.value = ''
  showTaskModal.value = true
  if (task.module === 'custom') await loadCustomTasks()
}

function closeTaskModal() {
  showTaskModal.value = false
}

async function saveTask() {
  if (!taskForm.value.name || !taskForm.value.action) {
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
      await apiClient.updateScheduledTask(editingTask.value.id, taskForm.value)
    } else {
      await apiClient.createScheduledTask(props.hostId, taskForm.value)
    }
    closeTaskModal()
    await loadTasks()
  } catch (e) {
    taskModalError.value = e.response?.data?.error || e.response?.data?.warning || 'Erreur lors de la sauvegarde'
  } finally {
    taskSaving.value = false
  }
}

async function toggleTask(task) {
  try {
    await apiClient.updateScheduledTask(task.id, { ...task, enabled: !task.enabled })
    await loadTasks()
  } catch (e) {
    tasksError.value = e.response?.data?.error || 'Erreur'
  }
}

async function runTaskNow(task) {
  taskRunningId.value = task.id
  try {
    const { data } = await apiClient.runScheduledTask(task.id)
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
  } catch (e) {
    tasksError.value = e.response?.data?.error || 'Erreur'
  } finally {
    taskRunningId.value = null
  }
}

function openTaskLogs(task) {
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

async function confirmDeleteTask(task) {
  const confirmed = await dialog.confirm({
    title: 'Supprimer la tache',
    message: `Supprimer la tache "${task.name}" ?\nCette action est irreversible.`,
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await apiClient.deleteScheduledTask(task.id)
    await loadTasks()
  } catch (e) {
    tasksError.value = e.response?.data?.error || 'Erreur de suppression'
  }
}
</script>

