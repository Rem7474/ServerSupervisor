<template>
  <div>
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <div class="page-pretitle">
            <router-link to="/" class="text-decoration-none">Dashboard</router-link>
            <span class="text-muted mx-1">/</span>
            <router-link :to="`/hosts/${hostId}`" class="text-decoration-none">Hôte</router-link>
            <span class="text-muted mx-1">/</span>
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
      <div v-else-if="!tasks.length" class="card-body text-center py-5">
        <svg xmlns="http://www.w3.org/2000/svg" class="icon mb-3 text-muted" width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
        </svg>
        <h3 class="mb-1">Aucune tâche planifiée</h3>
        <p class="text-secondary mb-3">Automatisez vos opérations en créant une tâche planifiée.</p>
        <button v-if="canManage" class="btn btn-primary" @click="openCreate">Nouvelle tâche</button>
      </div>
      <div v-else class="table-responsive">
        <table class="table table-vcenter table-hover card-table mb-0">
          <thead>
            <tr>
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
            <tr v-for="task in tasks" :key="task.id">
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
                  :class="task.last_run_status === 'completed' ? 'badge bg-success-lt' : 'badge bg-danger-lt'">
                  {{ task.last_run_status }}
                  <span v-if="task.last_run_at" class="ms-1 text-muted small">{{ formatDate(task.last_run_at) }}</span>
                </span>
                <span v-else class="text-muted">jamais</span>
              </td>
              <td>
                <input v-if="canManage && !isManualOnly(task)" type="checkbox" class="form-check-input"
                  :checked="task.enabled" @change="toggleTask(task)" />
                <span v-else-if="isManualOnly(task)" class="text-muted small">—</span>
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
      <div class="modal-dialog modal-dialog-centered modal-lg">
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
                <select v-model="form.module" class="form-select" @change="onModuleChange">
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
                <!-- Dropdown for known modules -->
                <select v-if="moduleActions[form.module]" v-model="form.action" class="form-select">
                  <option v-for="a in moduleActions[form.module]" :key="a" :value="a">{{ a }}</option>
                </select>
                <!-- Free-text for custom -->
                <input v-else v-model="form.action" type="text" class="form-control" placeholder="run" />
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

            <!-- Manual-only toggle -->
            <div class="mb-3">
              <label class="form-check form-switch">
                <input v-model="manualOnly" type="checkbox" class="form-check-input" />
                <span class="form-check-label">Exécution manuelle uniquement (pas de planification automatique)</span>
              </label>
            </div>

            <!-- CronBuilder (hidden if manual-only) -->
            <div v-if="!manualOnly" class="mb-3">
              <CronBuilder v-model="form.cron_expression" />
            </div>

            <div class="form-check form-switch mb-1" v-if="!manualOnly">
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
            <strong>{{ runResult.name }}</strong> déclenchée — commande <code>{{ runResult.id }}</code>
          </div>
          <button type="button" class="btn-close btn-close-white me-2 m-auto" @click="runResult = null"></button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import api from '../api'
import CronBuilder from '../components/CronBuilder.vue'

const route = useRoute()
const auth = useAuthStore()
const dialog = useConfirmDialog()
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
const manualOnly = ref(false)

const MANUAL_SENTINEL = '0 0 29 2 *'

const form = ref({ name: '', module: 'apt', action: 'update', target: '', cron_expression: '0 3 * * 0', enabled: true })

const moduleActions = {
  apt:       ['update', 'upgrade', 'dist-upgrade'],
  docker:    ['start', 'stop', 'restart', 'logs', 'pull'],
  systemd:   ['start', 'stop', 'restart', 'status', 'enable', 'disable'],
  journal:   ['read'],
  processes: ['list'],
  custom:    null  // free-text
}

const canManage = computed(() => auth.role === 'admin' || auth.role === 'operator')

function onModuleChange() {
  const actions = moduleActions[form.value.module]
  if (actions) {
    form.value.action = actions[0]
  } else {
    form.value.action = ''
  }
}

function isManualOnly(task) {
  return task.cron_expression === MANUAL_SENTINEL && !task.enabled
}

// Watch manualOnly toggle
watch(manualOnly, (val) => {
  if (val) {
    form.value.enabled = false
    form.value.cron_expression = MANUAL_SENTINEL
  } else {
    form.value.enabled = true
    if (form.value.cron_expression === MANUAL_SENTINEL) {
      form.value.cron_expression = '0 3 * * 0'
    }
  }
})

function describeCron(expr) {
  if (!expr) return ''
  const presets = {
    '@daily': 'tous les jours à minuit',
    '@hourly': 'toutes les heures',
    '@weekly': 'hebdomadaire (dim. minuit)',
    '@monthly': 'mensuel (1er à minuit)'
  }
  if (presets[expr]) return presets[expr]
  const parts = expr.split(' ')
  if (parts.length !== 5) return ''
  const [min, hour, dom, , dow] = parts
  const dayNames = ['dim', 'lun', 'mar', 'mer', 'jeu', 'ven', 'sam']
  if (dom === '*' && dow === '*' && hour !== '*' && min !== '*') {
    return `tous les jours à ${hour.padStart(2, '0')}h${min.padStart(2, '0')}`
  }
  if (dom !== '*' && dow === '*' && hour !== '*' && min !== '*') {
    return `le ${dom} du mois à ${hour.padStart(2, '0')}h${min.padStart(2, '0')}`
  }
  if (dom === '*' && dow !== '*') {
    const days = dow.split(',').map(d => {
      const n = parseInt(d)
      return !isNaN(n) && n <= 6 ? dayNames[n] : d
    })
    if (hour !== '*' && min !== '*') {
      return `chaque ${days.join(', ')} à ${hour.padStart(2, '0')}h${min.padStart(2, '0')}`
    }
    return `chaque ${days.join(', ')}`
  }
  return ''
}

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
  manualOnly.value = false
  form.value = { name: '', module: 'apt', action: 'update', target: '', cron_expression: '0 3 * * 0', enabled: true }
  modalError.value = ''
  showModal.value = true
}

function openEdit(task) {
  editingTask.value = task
  manualOnly.value = isManualOnly(task)
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
  if (!form.value.name || !form.value.action) {
    modalError.value = 'Nom et action sont obligatoires.'
    return
  }
  if (!manualOnly.value && !form.value.cron_expression) {
    modalError.value = 'Expression cron obligatoire.'
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
    runResult.value = { id: data.command_id, name: task.name }
    setTimeout(() => { runResult.value = null }, 5000)
    await loadTasks()
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur'
  } finally {
    runningId.value = null
  }
}

async function confirmDelete(task) {
  const confirmed = await dialog.confirm({
    title: 'Supprimer la tâche',
    message: `Supprimer la tâche "${task.name}" ?\nCette action est irréversible.`,
    variant: 'danger'
  })
  if (!confirmed) return
  try {
    await api.deleteScheduledTask(task.id)
    await loadTasks()
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur de suppression'
  }
}

onMounted(loadTasks)
</script>
