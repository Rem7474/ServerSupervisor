<template>
  <div>
    <div class="page-header mb-3">
      <div>
        <div class="page-pretitle">
          <router-link to="/git-webhooks" class="text-decoration-none">Git / Automatisation</router-link>
          <span class="text-muted mx-1">/</span>
          <span>Suivi de releases</span>
          <span class="text-muted mx-1">/</span>
          <span>{{ tracker?.name || id }}</span>
        </div>
        <h2 class="page-title d-flex align-items-center gap-2">
          {{ tracker?.name }}
          <span v-if="tracker" class="badge" :class="providerBadge(tracker.provider)">{{ tracker.provider }}</span>
          <span v-if="tracker && !tracker.enabled" class="badge bg-secondary">Désactivé</span>
        </h2>
      </div>
    </div>

    <div v-if="error" class="alert alert-danger mb-3">{{ error }}</div>

    <div v-if="loading" class="text-center py-5">
      <div class="spinner-border text-primary" role="status"></div>
    </div>

    <div v-else-if="tracker" class="row g-3">
      <!-- Left column: config -->
      <div class="col-lg-5">
        <!-- Config card -->
        <div class="card">
          <div class="card-header d-flex align-items-center justify-content-between">
            <h3 class="card-title">Configuration</h3>
            <div class="d-flex gap-2">
              <button class="btn btn-sm btn-ghost-secondary" @click="triggerCheck" :disabled="checking">
                <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24" class="me-1">
                  <polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 11-2.12-9.36L23 10"/>
                </svg>
                {{ checking ? 'Vérification...' : 'Vérifier maintenant' }}
              </button>
              <button class="btn btn-sm btn-primary" @click="runManually" :disabled="running">
                <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24" class="me-1">
                  <polygon points="5 3 19 12 5 21 5 3"/>
                </svg>
                {{ running ? 'Déclenchement...' : 'Exécuter' }}
              </button>
              <button class="btn btn-sm btn-ghost-secondary" @click="openEdit">Modifier</button>
            </div>
          </div>
          <div class="card-body">
            <dl class="row mb-0 small">
              <dt class="col-5 text-muted">Provider</dt>
              <dd class="col-7">{{ tracker.provider }}</dd>
              <dt class="col-5 text-muted">Dépôt</dt>
              <dd class="col-7">
                <a :href="repoURL" target="_blank" class="link-primary">
                  {{ tracker.repo_owner }}/{{ tracker.repo_name }}
                </a>
              </dd>
              <dt class="col-5 text-muted">VM cible</dt>
              <dd class="col-7">{{ tracker.host_name || tracker.host_id }}</dd>
              <template v-if="tracker.docker_image">
                <dt class="col-5 text-muted">Image Docker</dt>
                <dd class="col-7"><code>{{ tracker.docker_image }}</code></dd>
              </template>
              <dt class="col-5 text-muted">Tâche</dt>
              <dd class="col-7"><code>{{ tracker.custom_task_id }}</code></dd>
              <dt class="col-5 text-muted">Dernière release</dt>
              <dd class="col-7">
                <span v-if="tracker.last_release_tag" class="badge bg-green-lt text-green">{{ tracker.last_release_tag }}</span>
                <span v-else class="text-muted">En attente...</span>
              </dd>
              <dt v-if="tracker.last_checked_at" class="col-5 text-muted">Dernier check</dt>
              <dd v-if="tracker.last_checked_at" class="col-7"><RelativeTime :date="tracker.last_checked_at" /></dd>
              <template v-if="tracker.last_error">
                <dt class="col-5 text-muted">Erreur</dt>
                <dd class="col-7 text-danger small">{{ tracker.last_error }}</dd>
              </template>
              <dt v-if="tracker.last_triggered_at" class="col-5 text-muted">Dernier déclench.</dt>
              <dd v-if="tracker.last_triggered_at" class="col-7"><RelativeTime :date="tracker.last_triggered_at" /></dd>
              <dt v-if="tracker.notify_channels?.length" class="col-5 text-muted">Notifications</dt>
              <dd v-if="tracker.notify_channels?.length" class="col-7">
                <span v-for="ch in tracker.notify_channels" :key="ch" class="badge me-1" :class="channelBadge(ch)">{{ ch }}</span>
              </dd>
              <dt class="col-5 text-muted">Créé le</dt>
              <dd class="col-7">{{ formatDateTime(tracker.created_at) }}</dd>
            </dl>
          </div>
        </div>

        <!-- Env vars card -->
        <div class="card mt-3">
          <div class="card-header">
            <h3 class="card-title">Variables disponibles dans le script</h3>
          </div>
          <div class="card-body p-0">
            <div class="table-responsive">
              <table class="table table-sm table-vcenter mb-0">
                <tbody>
                  <tr v-for="v in envVars" :key="v.name">
                    <td><code class="small">{{ v.name }}</code></td>
                    <td class="text-muted small">{{ v.desc }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>

      <!-- Right column: execution history -->
      <div class="col-lg-7">
        <div class="card">
          <div class="card-header d-flex align-items-center justify-content-between">
            <h3 class="card-title">Historique des exécutions</h3>
            <button class="btn btn-sm btn-ghost-secondary" @click="loadExecutions">
              <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 11-2.12-9.36L23 10"/>
              </svg>
              Actualiser
            </button>
          </div>

          <div v-if="executions.length === 0" class="card-body text-center text-muted py-5">
            Aucune exécution enregistrée.
          </div>
          <div v-else class="table-responsive">
            <table class="table table-vcenter card-table table-sm">
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Release</th>
                  <th>Statut</th>
                  <th>Logs</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="ex in executions" :key="ex.id">
                  <td class="text-muted small text-nowrap">
                    <RelativeTime :date="ex.triggered_at" />
                  </td>
                  <td class="small">
                    <div>
                      <a v-if="ex.release_url" :href="ex.release_url" target="_blank" class="link-primary fw-semibold">
                        {{ ex.tag_name || '—' }}
                      </a>
                      <span v-else class="fw-semibold">{{ ex.tag_name || '—' }}</span>
                    </div>
                    <div v-if="ex.release_name && ex.release_name !== ex.tag_name" class="text-muted text-truncate" style="max-width:180px">
                      {{ ex.release_name }}
                    </div>
                  </td>
                  <td>
                    <span class="badge" :class="execStatusBadge(ex.status)">{{ ex.status }}</span>
                  </td>
                  <td>
                    <router-link
                      v-if="ex.command_id"
                      :to="`/audit?command=${ex.command_id}`"
                      class="btn btn-sm btn-ghost-secondary"
                      title="Voir les logs"
                    >
                      <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                        <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/>
                        <line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/>
                      </svg>
                    </router-link>
                    <span v-else class="text-muted">—</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>

    <!-- Edit modal -->
    <div v-if="showModal" class="modal modal-blur show d-block" style="background:rgba(0,0,0,.5)">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">Modifier le tracker</h5>
            <button type="button" class="btn-close" @click="showModal = false"></button>
          </div>
          <div class="modal-body">
            <div v-if="modalError" class="alert alert-danger">{{ modalError }}</div>
            <div class="row g-3">
              <div class="col-12">
                <label class="form-label required">Nom</label>
                <input type="text" class="form-control" v-model="form.name">
              </div>
              <div class="col-md-4">
                <label class="form-label required">Provider</label>
                <select class="form-select" v-model="form.provider">
                  <option value="github">GitHub</option>
                  <option value="gitlab">GitLab</option>
                  <option value="gitea">Gitea (Codeberg)</option>
                </select>
              </div>
              <div class="col-md-4">
                <label class="form-label required">Owner / Org</label>
                <input type="text" class="form-control" v-model="form.repo_owner">
              </div>
              <div class="col-md-4">
                <label class="form-label required">Dépôt</label>
                <input type="text" class="form-control" v-model="form.repo_name">
              </div>
              <div class="col-md-6">
                <label class="form-label required">VM cible</label>
                <select class="form-select" v-model="form.host_id">
                  <option value="">-- Sélectionner --</option>
                  <option v-for="h in hosts" :key="h.id" :value="h.id">{{ h.name }}</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label required">Tâche (tasks.yaml)</label>
                <select v-if="editCustomTasks.length" class="form-select" v-model="form.custom_task_id">
                  <option value="" disabled>-- Sélectionner une tâche --</option>
                  <option v-for="t in editCustomTasks" :key="t.id" :value="t.id">{{ t.name }} ({{ t.id }})</option>
                </select>
                <input v-else type="text" class="form-control" v-model="form.custom_task_id">
              </div>
              <div class="col-12">
                <label class="form-label">Image Docker suivie <span class="text-muted small">(optionnel)</span></label>
                <input type="text" class="form-control" v-model="form.docker_image" placeholder="ex: homeassistant/home-assistant ou ghcr.io/mealie-recipes/mealie">
                <div class="form-hint">Si renseigné, la version du conteneur tournant sera comparée au dernier tag sur le dashboard. Supporte Docker Hub et d'autres registries (ghcr.io, etc.).</div>
              </div>
              <div class="col-12">
                <label class="form-label">Notifications</label>
                <div class="d-flex flex-wrap gap-3 mt-1">
                  <label class="form-check">
                    <input class="form-check-input" type="checkbox" v-model="form.notify_on_release">
                    <span class="form-check-label">Notifier à chaque nouvelle release</span>
                  </label>
                </div>
                <div class="d-flex flex-wrap gap-3 mt-2">
                  <label v-for="ch in ['smtp','ntfy','browser']" :key="ch" class="form-check">
                    <input class="form-check-input" type="checkbox" :value="ch" v-model="form.notify_channels">
                    <span class="form-check-label">{{ ch }}</span>
                  </label>
                </div>
              </div>
              <div class="col-12">
                <label class="form-check form-switch">
                  <input class="form-check-input" type="checkbox" v-model="form.enabled">
                  <span class="form-check-label">Activé</span>
                </label>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button class="btn btn-secondary" @click="showModal = false">Annuler</button>
            <button class="btn btn-primary" @click="saveEdit" :disabled="saving">
              {{ saving ? 'Enregistrement...' : 'Mettre à jour' }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api'
import { formatDateTime } from '../utils/formatters'
import RelativeTime from '../components/RelativeTime.vue'

const route = useRoute()
const id = route.params.id

const tracker = ref(null)
const executions = ref([])
const hosts = ref([])
const loading = ref(false)
const error = ref('')
const checking = ref(false)
const running = ref(false)

const showModal = ref(false)
const saving = ref(false)
const modalError = ref('')
const form = ref({})
const editCustomTasks = ref([])

watch(() => form.value.host_id, async (hostId) => {
  if (!hostId) { editCustomTasks.value = []; return }
  try {
    const { data } = await api.getHostCustomTasks(hostId)
    editCustomTasks.value = Array.isArray(data) ? data : []
  } catch {
    editCustomTasks.value = []
  }
})

const envVars = [
  { name: 'SS_REPO_NAME',    desc: 'owner/repo (ex: home-assistant/core)' },
  { name: 'SS_TAG_NAME',     desc: 'Tag de la nouvelle release (ex: v1.2.3)' },
  { name: 'SS_RELEASE_URL',  desc: 'URL de la release sur le provider' },
  { name: 'SS_RELEASE_NAME', desc: 'Titre de la release' },
  { name: 'SS_TRACKER_NAME', desc: 'Nom du tracker dans ServerSupervisor' },
]

const repoURL = computed(() => {
  if (!tracker.value) return '#'
  switch (tracker.value.provider) {
    case 'gitlab': return `https://gitlab.com/${tracker.value.repo_owner}/${tracker.value.repo_name}`
    case 'gitea': return `https://codeberg.org/${tracker.value.repo_owner}/${tracker.value.repo_name}`
    default: return `https://github.com/${tracker.value.repo_owner}/${tracker.value.repo_name}`
  }
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [res, hostsRes] = await Promise.all([api.getReleaseTracker(id), api.getHosts()])
    tracker.value = res.data.tracker
    executions.value = res.data.executions || []
    hosts.value = hostsRes.data || []
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur lors du chargement'
  } finally {
    loading.value = false
  }
}

async function loadExecutions() {
  try {
    const res = await api.getReleaseTrackerExecutions(id)
    executions.value = res.data.executions || []
  } catch { /* ignore */ }
}

async function runManually() {
  running.value = true
  try {
    await api.runReleaseTracker(id)
    setTimeout(async () => {
      await load()
      running.value = false
    }, 2000)
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur lors du déclenchement'
    running.value = false
  }
}

async function triggerCheck() {
  checking.value = true
  try {
    await api.checkReleaseTrackerNow(id)
    setTimeout(async () => {
      await load()
      checking.value = false
    }, 2000)
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur'
    checking.value = false
  }
}

function openEdit() {
  const t = tracker.value
  form.value = {
    name: t.name, provider: t.provider,
    repo_owner: t.repo_owner, repo_name: t.repo_name, docker_image: t.docker_image || '',
    host_id: t.host_id, custom_task_id: t.custom_task_id,
    notify_channels: [...(t.notify_channels || [])],
    notify_on_release: t.notify_on_release,
    enabled: t.enabled,
  }
  modalError.value = ''
  showModal.value = true
}

async function saveEdit() {
  if (!form.value.name || !form.value.repo_owner || !form.value.repo_name ||
      !form.value.host_id || !form.value.custom_task_id) {
    modalError.value = 'Tous les champs obligatoires doivent être remplis.'
    return
  }
  saving.value = true
  modalError.value = ''
  try {
    await api.updateReleaseTracker(id, form.value)
    showModal.value = false
    await load()
  } catch (e) {
    modalError.value = e.response?.data?.error || 'Erreur'
  } finally {
    saving.value = false
  }
}

function providerBadge(provider) {
  const map = {
    github:  'bg-blue-lt text-blue',
    gitlab:  'bg-orange-lt text-orange',
    gitea:   'bg-teal-lt text-teal',
    forgejo: 'bg-purple-lt text-purple',
    custom:  'bg-secondary-lt text-secondary',
  }
  return map[provider] || 'bg-secondary-lt text-secondary'
}

function channelBadge(ch) {
  const map = {
    smtp:    'bg-blue-lt text-blue',
    ntfy:    'bg-orange-lt text-orange',
    browser: 'bg-purple-lt text-purple',
  }
  return map[ch] || 'bg-secondary-lt text-secondary'
}

function execStatusBadge(status) {
  const map = {
    pending: 'bg-yellow-lt text-yellow', running: 'bg-blue-lt text-blue',
    completed: 'bg-success-lt text-success', failed: 'bg-danger-lt text-danger', skipped: 'bg-secondary-lt text-secondary',
  }
  return map[status] || 'bg-secondary'
}

onMounted(load)
</script>
