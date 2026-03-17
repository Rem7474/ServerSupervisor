<template>
  <div>
    <div class="page-header mb-3">
      <div class="d-flex align-items-center justify-content-between gap-3">
        <div>
          <div class="page-pretitle">
            <router-link to="/" class="text-decoration-none">Dashboard</router-link>
            <span class="text-muted mx-1">/</span>
            <span>Tâches planifiées</span>
          </div>
          <h2 class="page-title">Tâches planifiées</h2>
        </div>
        <div class="d-flex align-items-center gap-2">
          <span class="text-muted small">{{ tasks.length }} tâche{{ tasks.length !== 1 ? 's' : '' }}</span>
          <button class="btn btn-outline-secondary btn-sm" @click="loadTasks">
            <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/>
              <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
            </svg>
            Actualiser
          </button>
        </div>
      </div>
    </div>

    <!-- Filters & Sort -->
    <div class="row g-3 mb-3 align-items-center">
      <div class="col-auto">
        <input v-model="filterText" type="text" class="form-control form-control-sm" placeholder="Rechercher…" style="min-width:200px" />
      </div>
      <div class="col-auto">
        <select v-model="filterHost" class="form-select form-select-sm" style="min-width:160px">
          <option value="">Tous les hôtes</option>
          <option v-for="host in hostList" :key="host" :value="host">{{ host }}</option>
        </select>
      </div>
      <div class="col-auto">
        <select v-model="filterModule" class="form-select form-select-sm">
          <option value="">Tous les modules</option>
          <option value="apt">apt</option>
          <option value="docker">docker</option>
          <option value="systemd">systemd</option>
          <option value="journal">journal</option>
          <option value="processes">processes</option>
          <option value="custom">custom</option>
        </select>
      </div>
      <div class="col-auto">
        <select v-model="filterStatus" class="form-select form-select-sm">
          <option value="">Tous les statuts</option>
          <option value="enabled">Activées</option>
          <option value="disabled">Désactivées</option>
          <option value="manual">Manuelles</option>
        </select>
      </div>
      <div class="col-auto ms-auto d-flex gap-2">
        <select v-model="sortKey" class="form-select form-select-sm" style="min-width:160px">
          <option value="name">Nom</option>
          <option value="host_name">Hôte</option>
          <option value="module">Module</option>
          <option value="next_run_at">Prochain run</option>
          <option value="last_run_at">Dernier run</option>
        </select>
        <button class="btn btn-sm btn-outline-secondary" @click="sortDir = sortDir === 'asc' ? 'desc' : 'asc'" :title="sortDir === 'asc' ? 'Croissant' : 'Décroissant'">
          <svg v-if="sortDir === 'asc'" xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 6l7 0"/><path d="M4 12l7 0"/><path d="M4 18l9 0"/><path d="M15 9l3 -3l3 3"/><path d="M18 6l0 12"/></svg>
          <svg v-else xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 6l9 0"/><path d="M4 12l7 0"/><path d="M4 18l7 0"/><path d="M15 15l3 3l3 -3"/><path d="M18 6l0 12"/></svg>
        </button>
      </div>
    </div>

    <div v-if="error" class="alert alert-danger">{{ error }}</div>

    <div class="card">
      <div v-if="loading" class="card-body text-center py-5">
        <span class="spinner-border text-primary"></span>
      </div>
      <div v-else-if="!filteredTasks.length" class="card-body text-center py-5">
        <svg xmlns="http://www.w3.org/2000/svg" class="icon mb-3 text-muted" width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
        </svg>
        <h3 class="mb-1">Aucune tâche trouvée</h3>
        <p class="text-secondary mb-0">
          {{ tasks.length ? 'Modifiez vos filtres.' : 'Créez des tâches depuis la page d\'un hôte.' }}
        </p>
      </div>
      <div v-else class="table-responsive">
        <table class="table table-vcenter table-hover card-table mb-0">
          <thead>
            <tr>
              <th>Hôte</th>
              <th>Nom</th>
              <th>Module / Action</th>
              <th>Planification</th>
              <th>Prochaine exécution</th>
              <th>Dernier résultat</th>
              <th>Activée</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="task in filteredTasks" :key="task.id">
              <td>
                <router-link :to="`/hosts/${task.host_id}`" class="text-decoration-none fw-medium">
                  {{ task.host_name }}
                </router-link>
              </td>
              <td>{{ task.name }}</td>
              <td>
                <span class="badge bg-blue-lt me-1">{{ task.module }}</span>
                <span class="text-secondary small">{{ task.action }}</span>
                <span v-if="task.target" class="text-muted small ms-1">— {{ task.target }}</span>
              </td>
              <td>
                <span v-if="isManualOnly(task)" class="badge bg-secondary-lt text-secondary">Manuel</span>
                <template v-else>
                  <code class="small">{{ task.cron_expression }}</code>
                  <span v-if="describeCron(task.cron_expression)" class="text-muted small ms-1">— {{ describeCron(task.cron_expression) }}</span>
                </template>
              </td>
              <td>
                <span v-if="task.next_run_at && !isManualOnly(task)">{{ formatDate(task.next_run_at) }}</span>
                <span v-else class="text-muted">—</span>
              </td>
              <td>
                <span v-if="task.last_run_status"
                  :class="task.last_run_status === 'completed' ? 'badge bg-success-lt' : task.last_run_status === 'pending' ? 'badge bg-warning-lt' : 'badge bg-danger-lt'">
                  {{ task.last_run_status }}
                  <span v-if="task.last_run_at" class="ms-1 text-muted small">{{ formatDate(task.last_run_at) }}</span>
                </span>
                <span v-else class="text-muted">jamais</span>
              </td>
              <td>
                <span v-if="isManualOnly(task)" class="text-muted small">—</span>
                <span v-else-if="!canManage" class="badge" :class="task.enabled ? 'bg-success-lt' : 'bg-secondary-lt'">
                  {{ task.enabled ? 'Oui' : 'Non' }}
                </span>
                <input v-else type="checkbox" class="form-check-input"
                  :checked="task.enabled" @change="toggleTask(task)" />
              </td>
              <td class="text-end">
                <div class="d-flex gap-1 justify-content-end">
                  <button class="btn btn-sm btn-outline-secondary" @click="openHistory(task)" title="Historique d'exécutions">
                    <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
                    </svg>
                  </button>
                  <button v-if="canManage" class="btn btn-sm btn-outline-primary"
                    :disabled="runningId === task.id" @click="runNow(task)">
                    <span v-if="runningId === task.id" class="spinner-border spinner-border-sm"></span>
                    <span v-else>Exécuter</span>
                  </button>
                  <button v-if="canManage" class="btn btn-sm btn-outline-secondary" @click="openEdit(task)" title="Modifier">
                    <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                    </svg>
                  </button>
                  <button v-if="canManage" class="btn btn-sm btn-outline-danger" @click="confirmDelete(task)" title="Supprimer">
                    <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14H6L5 6"/><path d="M10 11v6"/><path d="M14 11v6"/><path d="M9 6V4h6v2"/>
                    </svg>
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Run result toast -->
    <div v-if="runResult" class="position-fixed bottom-0 end-0 p-3" style="z-index:1100">
      <div class="toast show align-items-center text-bg-success border-0">
        <div class="d-flex">
          <div class="toast-body">
            <strong>{{ runResult.name }}</strong> déclenchée — commande <code>{{ runResult.id }}</code>
          </div>
          <button type="button" class="btn-close btn-close-white me-2 m-auto" @click="runResult = null"></button>
        </div>
      </div>
    </div>

    <!-- Edit task modal -->
    <div v-if="editTask" class="modal modal-blur show d-block" tabindex="-1" style="background:rgba(0,0,0,.5);z-index:1050" @click.self="editTask = null">
      <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">Modifier la tâche</h5>
            <button type="button" class="btn-close" @click="editTask = null"></button>
          </div>
          <div class="modal-body">
            <div class="mb-3">
              <label class="form-label">Nom</label>
              <input v-model="editForm.name" type="text" class="form-control" />
            </div>
            <div class="mb-3">
              <label class="form-label">Expression cron</label>
              <input v-model="editForm.cron_expression" type="text" class="form-control font-monospace" placeholder="ex: 0 3 * * *" />
              <div v-if="editForm.cron_expression && describeCron(editForm.cron_expression)" class="form-hint">
                {{ describeCron(editForm.cron_expression) }}
              </div>
            </div>
            <div class="mb-3 form-check">
              <input v-model="editForm.enabled" type="checkbox" class="form-check-input" id="editEnabled" :disabled="isManualOnly(editTask)" />
              <label class="form-check-label" for="editEnabled">Activée</label>
            </div>
            <div v-if="editError" class="alert alert-danger py-2">{{ editError }}</div>
          </div>
          <div class="modal-footer">
            <button class="btn btn-secondary" @click="editTask = null">Annuler</button>
            <button class="btn btn-primary" :disabled="editSaving" @click="saveEdit">
              <span v-if="editSaving" class="spinner-border spinner-border-sm me-1"></span>
              Enregistrer
            </button>
          </div>
        </div>
      </div>
    </div>


    <!-- Execution history modal -->
    <div v-if="historyTask" class="modal modal-blur show d-block" tabindex="-1" style="background:rgba(0,0,0,.5);z-index:1050" @click.self="historyTask = null">
      <div class="modal-dialog modal-xl modal-dialog-centered modal-dialog-scrollable">
        <div class="modal-content">
          <div class="modal-header">
            <div>
              <h5 class="modal-title mb-0">Historique d'exécutions</h5>
              <div class="text-muted small mt-1">
                <span class="badge bg-blue-lt me-1">{{ historyTask.module }}</span>
                {{ historyTask.name }}
                <span class="text-muted ms-1">— {{ historyTask.host_name }}</span>
              </div>
            </div>
            <button type="button" class="btn-close" @click="historyTask = null"></button>
          </div>
          <div class="modal-body p-0">
            <div v-if="historyLoading" class="text-center py-5">
              <span class="spinner-border text-primary"></span>
            </div>
            <div v-else-if="historyError" class="alert alert-danger m-3">{{ historyError }}</div>
            <div v-else-if="!executions.length" class="text-center py-5 text-muted">
              Aucune exécution enregistrée pour cette tâche.
            </div>
            <div v-else>
              <table class="table table-vcenter table-hover mb-0">
                <thead>
                  <tr>
                    <th>Date</th>
                    <th>Statut</th>
                    <th>Durée</th>
                    <th>Déclenché par</th>
                    <th>Sortie</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="ex in executions" :key="ex.id" :class="expandedId === ex.id ? 'table-active' : ''">
                    <td class="text-nowrap">{{ formatDate(ex.created_at) }}</td>
                    <td>
                      <span :class="statusBadge(ex.status)">{{ ex.status }}</span>
                    </td>
                    <td class="text-nowrap">
                      <span v-if="ex.ended_at && ex.started_at">{{ durationSec(ex.started_at, ex.ended_at) }}s</span>
                      <span v-else class="text-muted">—</span>
                    </td>
                    <td>{{ ex.triggered_by || '—' }}</td>
                    <td style="max-width:400px">
                      <div v-if="!ex.output" class="text-muted small">—</div>
                      <template v-else>
                        <div v-if="expandedId !== ex.id" class="d-flex align-items-center gap-2">
                          <span class="text-truncate small font-monospace" style="max-width:300px">{{ firstLine(ex.output) }}</span>
                          <button class="btn btn-xs btn-ghost-secondary ms-auto flex-shrink-0" @click="expandedId = ex.id">
                            Voir tout
                          </button>
                        </div>
                        <div v-else>
                          <pre class="mb-1 small" style="max-height:300px;overflow-y:auto;white-space:pre-wrap;word-break:break-all">{{ ex.output }}</pre>
                          <button class="btn btn-xs btn-ghost-secondary" @click="expandedId = null">Réduire</button>
                        </div>
                      </template>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
          <div class="modal-footer">
            <span class="text-muted small me-auto">{{ executions.length }} exécution{{ executions.length !== 1 ? 's' : '' }} (20 dernières)</span>
            <button class="btn btn-secondary" @click="historyTask = null">Fermer</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import api from '../api'
import { isManualOnly, describeCron } from '../utils/cron'
import { useConfirmDialog } from '../composables/useConfirmDialog'

const auth = useAuthStore()
const dialog = useConfirmDialog()

const tasks = ref([])
const loading = ref(false)
const error = ref('')
const runningId = ref(null)
const runResult = ref(null)

const filterText = ref('')
const filterHost = ref('')
const filterModule = ref('')
const filterStatus = ref('')
const sortKey = ref('name')
const sortDir = ref('asc')

// Edit modal state
const editTask = ref(null)
const editForm = ref({ name: '', cron_expression: '', enabled: false })
const editSaving = ref(false)
const editError = ref('')

// History modal state
const historyTask = ref(null)
const executions = ref([])
const historyLoading = ref(false)
const historyError = ref('')
const expandedId = ref(null)

const canManage = computed(() => auth.role === 'admin' || auth.role === 'operator')

const hostList = computed(() => {
  const names = [...new Set(tasks.value.map(t => t.host_name))]
  return names.sort()
})

const filteredTasks = computed(() => {
  const filtered = tasks.value.filter(task => {
    if (filterHost.value && task.host_name !== filterHost.value) return false
    if (filterModule.value && task.module !== filterModule.value) return false
    if (filterStatus.value === 'enabled' && (!task.enabled || isManualOnly(task))) return false
    if (filterStatus.value === 'disabled' && (task.enabled || isManualOnly(task))) return false
    if (filterStatus.value === 'manual' && !isManualOnly(task)) return false
    if (filterText.value) {
      const q = filterText.value.toLowerCase()
      if (!task.name.toLowerCase().includes(q) &&
          !task.host_name.toLowerCase().includes(q) &&
          !task.module.toLowerCase().includes(q) &&
          !(task.target || '').toLowerCase().includes(q)) return false
    }
    return true
  })

  return [...filtered].sort((a, b) => {
    const key = sortKey.value
    const av = a[key] ?? ''
    const bv = b[key] ?? ''
    const cmp = String(av).localeCompare(String(bv), 'fr', { numeric: true })
    return sortDir.value === 'asc' ? cmp : -cmp
  })
})

function formatDate(iso) {
  if (!iso) return ''
  return new Date(iso).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })
}

function statusBadge(status) {
  if (status === 'completed') return 'badge bg-success-lt'
  if (status === 'failed') return 'badge bg-danger-lt'
  if (status === 'running') return 'badge bg-info-lt'
  return 'badge bg-warning-lt'
}

function durationSec(start, end) {
  const ms = new Date(end) - new Date(start)
  return (ms / 1000).toFixed(1)
}

function firstLine(output) {
  return (output || '').split('\n')[0].trim()
}

function openEdit(task) {
  editTask.value = task
  editForm.value = { name: task.name, cron_expression: task.cron_expression, enabled: task.enabled }
  editError.value = ''
}

async function saveEdit() {
  editSaving.value = true
  editError.value = ''
  try {
    await api.updateScheduledTask(editTask.value.id, {
      name: editForm.value.name,
      module: editTask.value.module,
      action: editTask.value.action,
      target: editTask.value.target,
      payload: editTask.value.payload,
      cron_expression: editForm.value.cron_expression,
      enabled: editForm.value.enabled,
    })
    editTask.value = null
    await loadTasks()
  } catch (e) {
    editError.value = e.response?.data?.error || 'Erreur lors de la sauvegarde'
  } finally {
    editSaving.value = false
  }
}

async function confirmDelete(task) {
  const ok = await dialog.confirm({
    title: 'Supprimer la tâche',
    message: `Supprimer « ${task.name} » sur ${task.host_name} ?`,
    variant: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteScheduledTask(task.id)
    await loadTasks()
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur lors de la suppression'
  }
}

async function openHistory(task) {
  historyTask.value = task
  executions.value = []
  expandedId.value = null
  historyError.value = ''
  historyLoading.value = true
  try {
    const { data } = await api.getScheduledTaskExecutions(task.id, 20)
    executions.value = data
  } catch (e) {
    historyError.value = e.response?.data?.error || 'Erreur de chargement'
  } finally {
    historyLoading.value = false
  }
}

async function loadTasks() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await api.getAllScheduledTasks()
    tasks.value = data
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur de chargement'
  } finally {
    loading.value = false
  }
}

async function toggleTask(task) {
  try {
    await api.updateScheduledTask(task.id, {
      name: task.name, module: task.module, action: task.action,
      target: task.target, payload: task.payload,
      cron_expression: task.cron_expression, enabled: !task.enabled,
    })
    await loadTasks()
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur'
  }
}

async function runNow(task) {
  runningId.value = task.id
  try {
    const { data } = await api.runScheduledTask(task.id)
    runResult.value = { id: data.command_id, name: task.name }
    setTimeout(() => { runResult.value = null }, 5000)
    await loadTasks()
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur'
  } finally {
    runningId.value = null
  }
}

onMounted(loadTasks)
</script>
