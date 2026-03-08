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

    <!-- Filters -->
    <div class="row g-2 mb-3">
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
                  <button v-if="canManage" class="btn btn-sm btn-outline-primary"
                    :disabled="runningId === task.id" @click="runNow(task)">
                    <span v-if="runningId === task.id" class="spinner-border spinner-border-sm"></span>
                    <span v-else>Exécuter</span>
                  </button>
                  <router-link :to="`/hosts/${task.host_id}/scheduled-tasks`" class="btn btn-sm btn-outline-secondary">
                    Gérer
                  </router-link>
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
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import api from '../api'

const auth = useAuthStore()

const tasks = ref([])
const loading = ref(false)
const error = ref('')
const runningId = ref(null)
const runResult = ref(null)

const filterText = ref('')
const filterHost = ref('')
const filterModule = ref('')
const filterStatus = ref('')

const MANUAL_SENTINEL = '0 0 29 2 *'

const canManage = computed(() => auth.role === 'admin' || auth.role === 'operator')

const hostList = computed(() => {
  const names = [...new Set(tasks.value.map(t => t.host_name))]
  return names.sort()
})

const filteredTasks = computed(() => {
  return tasks.value.filter(task => {
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
})

function isManualOnly(task) {
  return task.cron_expression === MANUAL_SENTINEL && !task.enabled
}

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
