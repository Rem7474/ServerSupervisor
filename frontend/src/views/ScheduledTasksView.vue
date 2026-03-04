<template>
  <div>
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <div class="text-secondary small">
            <router-link to="/" class="text-decoration-none">Dashboard</router-link>
            <span class="mx-1">/</span>
            <router-link :to="`/hosts/${hostId}`" class="text-decoration-none">Hôte</router-link>
            <span class="mx-1">/</span>
            <span>Tâches planifiées</span>
          </div>
          <h2 class="page-title">Tâches planifiées</h2>
        </div>
        <div class="d-flex gap-2">
          <router-link :to="`/hosts/${hostId}`" class="btn btn-outline-secondary">Retour à l'hôte</router-link>
          <button v-if="canManage" class="btn btn-primary" @click="openCreate">
            Nouvelle tâche
          </button>
        </div>
      </div>
    </div>

    <div v-if="error" class="alert alert-danger">{{ error }}</div>

    <div class="card">
      <div v-if="loading" class="card-body text-center py-5">
        <span class="spinner-border text-primary"></span>
      </div>
      <div v-else-if="!tasks.length" class="card-body text-secondary">
        Aucune tâche planifiée. Cliquez sur "Nouvelle tâche" pour en créer une.
      </div>
      <div v-else class="table-responsive">
        <table class="table table-vcenter table-hover card-table mb-0">
          <thead>
            <tr>
              <th>Nom</th>
              <th>Module / Action</th>
              <th>Cron</th>
              <th>Prochaine exécution</th>
              <th>Dernier résultat</th>
              <th>Activée</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="task in tasks" :key="task.id">
              <td>{{ task.name }}</td>
              <td>
                <span class="badge bg-blue-lt me-1">{{ task.module }}</span>
                <span class="text-secondary small">{{ task.action }}</span>
                <span v-if="task.target" class="text-muted small ms-1">— {{ task.target }}</span>
              </td>
              <td><code>{{ task.cron_expression }}</code></td>
              <td>
                <span v-if="task.next_run_at">{{ formatDate(task.next_run_at) }}</span>
                <span v-else class="text-muted">—</span>
              </td>
              <td>
                <span v-if="task.last_run_status"
                  :class="task.last_run_status === 'completed' ? 'badge bg-success-lt' : 'badge bg-danger-lt'">
                  {{ task.last_run_status }}
                  <span v-if="task.last_run_at" class="ms-1 text-muted small">{{ formatDate(task.last_run_at) }}</span>
                </span>
                <span v-else class="text-muted">jamais</span>
              </td>
              <td>
                <input v-if="canManage" type="checkbox" class="form-check-input"
                  :checked="task.enabled" @change="toggleTask(task)" />
                <span v-else>{{ task.enabled ? 'Oui' : 'Non' }}</span>
              </td>
              <td class="text-end">
                <div class="d-flex gap-1 justify-content-end">
                  <button v-if="canManage" class="btn btn-sm btn-outline-primary"
                    :disabled="runningId === task.id" @click="runNow(task)">
                    <span v-if="runningId === task.id" class="spinner-border spinner-border-sm"></span>
                    <span v-else>Exécuter</span>
                  </button>
                  <button v-if="canManage" class="btn btn-sm btn-outline-secondary" @click="openEdit(task)">
                    Modifier
                  </button>
                  <button v-if="canManage" class="btn btn-sm btn-outline-danger" @click="confirmDelete(task)">
                    Supprimer
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Create / Edit modal -->
    <div v-if="showModal" class="modal modal-blur show d-block" tabindex="-1" style="background: rgba(0,0,0,.5)">
      <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ editingTask ? 'Modifier la tâche' : 'Nouvelle tâche planifiée' }}</h5>
            <button type="button" class="btn-close" @click="closeModal"></button>
          </div>
          <div class="modal-body">
            <div v-if="modalError" class="alert alert-danger">{{ modalError }}</div>

            <div class="mb-3">
              <label class="form-label">Nom</label>
              <input v-model="form.name" type="text" class="form-control" placeholder="Mise à jour APT hebdomadaire" />
            </div>

            <div class="row g-3 mb-3">
              <div class="col">
                <label class="form-label">Module</label>
                <select v-model="form.module" class="form-select">
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
                <input v-model="form.action" type="text" class="form-control" :placeholder="actionPlaceholder" />
              </div>
            </div>

            <div v-if="form.module !== 'apt' && form.module !== 'processes'" class="mb-3">
              <label class="form-label">
                {{ form.module === 'custom' ? 'ID de la tâche (tasks.yaml)' : 'Cible' }}
              </label>
              <input v-model="form.target" type="text" class="form-control"
                :placeholder="form.module === 'custom' ? 'cleanup_logs' : 'nginx.service'" />
              <div v-if="form.module === 'custom'" class="form-hint">
                L'ID doit correspondre à une tâche définie dans <code>tasks.yaml</code> sur l'agent.
              </div>
            </div>

            <div class="mb-3">
              <label class="form-label">Expression cron</label>
              <input v-model="form.cron_expression" type="text" class="form-control" placeholder="0 3 * * 1" />
              <div class="form-hint">
                Format : minute heure jour-du-mois mois jour-de-la-semaine
                <span v-if="cronDescription" class="ms-1 text-primary">— {{ cronDescription }}</span>
              </div>
            </div>

            <div class="form-check form-switch mb-1">
              <input v-model="form.enabled" type="checkbox" class="form-check-input" id="taskEnabled" />
              <label class="form-check-label" for="taskEnabled">Activée</label>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-outline-secondary" @click="closeModal">Annuler</button>
            <button type="button" class="btn btn-primary" :disabled="saving" @click="saveTask">
              <span v-if="saving" class="spinner-border spinner-border-sm me-1"></span>
              {{ editingTask ? 'Enregistrer' : 'Créer' }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Run result toast -->
    <div v-if="runResult" class="position-fixed bottom-0 end-0 p-3" style="z-index:1100">
      <div class="toast show align-items-center text-bg-success border-0">
        <div class="d-flex">
          <div class="toast-body">
            Tâche déclenchée — commande <code>{{ runResult }}</code>
          </div>
          <button type="button" class="btn-close btn-close-white me-2 m-auto" @click="runResult = null"></button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api'

const route = useRoute()
const auth = useAuthStore()
const hostId = route.params.id

const tasks = ref([])
const loading = ref(false)
const error = ref('')
const runningId = ref(null)
const runResult = ref(null)

const showModal = ref(false)
const editingTask = ref(null)
const saving = ref(false)
const modalError = ref('')

const form = ref({ name: '', module: 'apt', action: 'update', target: '', cron_expression: '', enabled: true })

const canManage = computed(() => auth.role === 'admin' || auth.role === 'operator')

const actionPlaceholder = computed(() => {
  const hints = { apt: 'update | upgrade | dist-upgrade', docker: 'start | stop | restart | logs', systemd: 'start | stop | restart | status', journal: 'read', processes: 'list', custom: 'run' }
  return hints[form.value.module] || ''
})

// Minimal cron description without external lib
const cronDescription = computed(() => {
  const expr = form.value.cron_expression.trim()
  if (!expr) return ''
  const presets = { '@daily': 'tous les jours à minuit', '@hourly': 'toutes les heures', '@weekly': 'hebdomadaire (dimanche minuit)', '@monthly': 'mensuel (1er du mois à minuit)' }
  if (presets[expr]) return presets[expr]
  const parts = expr.split(' ')
  if (parts.length !== 5) return ''
  const [min, hour, dom, , dow] = parts
  if (dom === '*' && dow !== '*') {
    const days = ['dim', 'lun', 'mar', 'mer', 'jeu', 'ven', 'sam']
    const d = parseInt(dow)
    const dayName = !isNaN(d) && d < 7 ? days[d] : dow
    return `chaque ${dayName} à ${hour.padStart(2,'0')}h${min.padStart(2,'0')}`
  }
  if (hour !== '*' && min !== '*' && dom === '*' && dow === '*') {
    return `tous les jours à ${hour.padStart(2,'0')}h${min.padStart(2,'0')}`
  }
  return ''
})

function formatDate(iso) {
  if (!iso) return ''
  return new Date(iso).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })
}

async function loadTasks() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await api.getScheduledTasks(hostId)
    tasks.value = data
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur de chargement'
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingTask.value = null
  form.value = { name: '', module: 'apt', action: 'update', target: '', cron_expression: '0 3 * * 0', enabled: true }
  modalError.value = ''
  showModal.value = true
}

function openEdit(task) {
  editingTask.value = task
  form.value = {
    name: task.name,
    module: task.module,
    action: task.action,
    target: task.target,
    cron_expression: task.cron_expression,
    enabled: task.enabled,
  }
  modalError.value = ''
  showModal.value = true
}

function closeModal() {
  showModal.value = false
}

async function saveTask() {
  if (!form.value.name || !form.value.cron_expression || !form.value.action) {
    modalError.value = 'Nom, action et expression cron sont obligatoires.'
    return
  }
  saving.value = true
  modalError.value = ''
  try {
    if (editingTask.value) {
      await api.updateScheduledTask(editingTask.value.id, form.value)
    } else {
      await api.createScheduledTask(hostId, form.value)
    }
    closeModal()
    await loadTasks()
  } catch (e) {
    modalError.value = e.response?.data?.error || e.response?.data?.warning || 'Erreur lors de la sauvegarde'
  } finally {
    saving.value = false
  }
}

async function toggleTask(task) {
  try {
    await api.updateScheduledTask(task.id, { ...task, enabled: !task.enabled })
    await loadTasks()
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur'
  }
}

async function runNow(task) {
  runningId.value = task.id
  try {
    const { data } = await api.runScheduledTask(task.id)
    runResult.value = data.command_id
    setTimeout(() => { runResult.value = null }, 5000)
    await loadTasks()
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur'
  } finally {
    runningId.value = null
  }
}

async function confirmDelete(task) {
  if (!confirm(`Supprimer la tâche "${task.name}" ?`)) return
  try {
    await api.deleteScheduledTask(task.id)
    await loadTasks()
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur de suppression'
  }
}

onMounted(loadTasks)
</script>
