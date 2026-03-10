<template>
  <div>
    <div class="page-header mb-3">
      <div>
        <div class="page-pretitle">
          <router-link to="/git-webhooks" class="text-decoration-none">Git Webhooks</router-link>
          <span class="text-muted mx-1">/</span>
          <span>{{ webhook?.name || id }}</span>
        </div>
        <h2 class="page-title d-flex align-items-center gap-2">
          {{ webhook?.name }}
          <span v-if="webhook" class="badge" :class="providerBadge(webhook.provider)">{{ webhook.provider }}</span>
          <span v-if="webhook && !webhook.enabled" class="badge bg-secondary">Désactivé</span>
        </h2>
      </div>
    </div>

    <div v-if="error" class="alert alert-danger mb-3">{{ error }}</div>

    <div v-if="loading" class="text-center py-5">
      <div class="spinner-border text-primary" role="status"></div>
    </div>

    <div v-else-if="webhook" class="row g-3">
      <!-- Left column: URL card + config -->
      <div class="col-lg-5">
        <!-- URL + Secret card -->
        <WebhookUrlCard
          :webhook-id="id"
          :secret="revealedSecret"
          :provider="webhook.provider"
          @secret-regenerated="onSecretRegenerated"
        />

        <!-- Config summary -->
        <div class="card mt-3">
          <div class="card-header d-flex align-items-center justify-content-between">
            <h3 class="card-title">Configuration</h3>
            <router-link to="/git-webhooks" class="btn btn-sm btn-ghost-secondary" @click.prevent="openEdit">Modifier</router-link>
          </div>
          <div class="card-body">
            <dl class="row mb-0 small">
              <dt class="col-5 text-muted">Événement</dt>
              <dd class="col-7">{{ webhook.event_filter }}</dd>
              <dt class="col-5 text-muted">Filtre repo</dt>
              <dd class="col-7">{{ webhook.repo_filter || '<tous>' }}</dd>
              <dt class="col-5 text-muted">Filtre branche</dt>
              <dd class="col-7">{{ webhook.branch_filter || '<toutes>' }}</dd>
              <dt class="col-5 text-muted">VM cible</dt>
              <dd class="col-7">{{ webhook.host_name || webhook.host_id }}</dd>
              <dt class="col-5 text-muted">Tâche</dt>
              <dd class="col-7"><code>{{ webhook.custom_task_id }}</code></dd>
              <dt v-if="webhook.notify_channels?.length" class="col-5 text-muted">Notifications</dt>
              <dd v-if="webhook.notify_channels?.length" class="col-7">
                <span v-for="ch in webhook.notify_channels" :key="ch" class="badge me-1" :class="channelBadge(ch)">{{ ch }}</span>
                <span class="text-muted">({{ [webhook.notify_on_success && 'succès', webhook.notify_on_failure && 'échec'].filter(Boolean).join(', ') || 'aucune' }})</span>
              </dd>
              <dt class="col-5 text-muted">Créé le</dt>
              <dd class="col-7">{{ formatDateTime(webhook.created_at) }}</dd>
              <dt v-if="webhook.last_triggered_at" class="col-5 text-muted">Dernier déclench.</dt>
              <dd v-if="webhook.last_triggered_at" class="col-7"><RelativeTime :date="webhook.last_triggered_at" /></dd>
            </dl>
          </div>
        </div>

        <!-- Variables disponibles -->
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
                  <th>Repo / Branche</th>
                  <th>Commit</th>
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
                    <div class="text-truncate" style="max-width:160px" :title="ex.repo_name">{{ ex.repo_name || '—' }}</div>
                    <div class="text-muted">{{ ex.branch }}</div>
                  </td>
                  <td class="small">
                    <div v-if="ex.commit_sha" class="font-monospace text-muted">{{ ex.commit_sha.slice(0, 7) }}</div>
                    <div class="text-truncate" style="max-width:140px" :title="ex.commit_message">{{ ex.commit_message || '—' }}</div>
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
                        <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/>
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

    <!-- Edit modal (inline, reuses same form as list view) -->
    <div v-if="showModal" class="modal modal-blur show d-block" style="background:rgba(0,0,0,.5)">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">Modifier le webhook</h5>
            <button type="button" class="btn-close" @click="showModal = false"></button>
          </div>
          <div class="modal-body">
            <div v-if="modalError" class="alert alert-danger">{{ modalError }}</div>
            <div class="row g-3">
              <div class="col-12">
                <label class="form-label required">Nom</label>
                <input type="text" class="form-control" v-model="form.name">
              </div>
              <div class="col-md-6">
                <label class="form-label required">Provider</label>
                <select class="form-select" v-model="form.provider">
                  <option value="github">GitHub</option>
                  <option value="gitlab">GitLab</option>
                  <option value="gitea">Gitea</option>
                  <option value="forgejo">Forgejo</option>
                  <option value="custom">Custom</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label">Événement</label>
                <select class="form-select" v-model="form.event_filter">
                  <option value="push">push</option>
                  <option value="tag">tag / create</option>
                  <option value="release">release</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label">Filtre repo</label>
                <input type="text" class="form-control" v-model="form.repo_filter">
              </div>
              <div class="col-md-6">
                <label class="form-label">Filtre branche</label>
                <input type="text" class="form-control" v-model="form.branch_filter">
              </div>
              <div class="col-md-6">
                <label class="form-label required">VM cible</label>
                <select class="form-select" v-model="form.host_id">
                  <option value="">-- Sélectionner --</option>
                  <option v-for="h in hosts" :key="h.id" :value="h.id">{{ h.name }}</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label required">ID Tâche</label>
                <input type="text" class="form-control" v-model="form.custom_task_id">
              </div>
              <div class="col-12">
                <label class="form-label">Notifications</label>
                <div class="d-flex flex-wrap gap-3 mt-1">
                  <label class="form-check"><input class="form-check-input" type="checkbox" v-model="form.notify_on_success"><span class="form-check-label">Succès</span></label>
                  <label class="form-check"><input class="form-check-input" type="checkbox" v-model="form.notify_on_failure"><span class="form-check-label">Échec</span></label>
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
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api'
import { formatDateTime } from '../utils/formatters'
import RelativeTime from '../components/RelativeTime.vue'
import WebhookUrlCard from '../components/WebhookUrlCard.vue'

const route = useRoute()
const id = route.params.id

const webhook = ref(null)
const executions = ref([])
const hosts = ref([])
const loading = ref(false)
const error = ref('')
const revealedSecret = ref('')

// Edit modal
const showModal = ref(false)
const saving = ref(false)
const modalError = ref('')
const form = ref({})

const envVars = [
  { name: 'SS_REPO_NAME', desc: 'Nom complet du dépôt (ex: monorg/mon-app)' },
  { name: 'SS_BRANCH', desc: 'Branche ou tag déclencheur' },
  { name: 'SS_COMMIT_SHA', desc: 'SHA du commit (40 chars)' },
  { name: 'SS_COMMIT_MESSAGE', desc: 'Première ligne du message de commit' },
  { name: 'SS_PUSHER', desc: 'Nom d\'utilisateur du pousseur' },
  { name: 'SS_WEBHOOK_NAME', desc: 'Nom du webhook dans ServerSupervisor' },
  { name: 'SS_EVENT_TYPE', desc: 'Type d\'événement (push, tag, release)' },
]

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [whRes, hostsRes] = await Promise.all([api.getGitWebhook(id), api.getHosts()])
    webhook.value = whRes.data.webhook
    executions.value = whRes.data.executions || []
    hosts.value = hostsRes.data || []
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur lors du chargement'
  } finally {
    loading.value = false
  }
}

async function loadExecutions() {
  try {
    const res = await api.getWebhookExecutions(id)
    executions.value = res.data.executions || []
  } catch { /* ignore */ }
}

function openEdit() {
  const wh = webhook.value
  form.value = {
    name: wh.name, provider: wh.provider, event_filter: wh.event_filter,
    repo_filter: wh.repo_filter, branch_filter: wh.branch_filter,
    host_id: wh.host_id, custom_task_id: wh.custom_task_id,
    notify_channels: [...(wh.notify_channels || [])],
    notify_on_success: wh.notify_on_success, notify_on_failure: wh.notify_on_failure,
    enabled: wh.enabled,
  }
  modalError.value = ''
  showModal.value = true
}

async function saveEdit() {
  if (!form.value.name || !form.value.host_id || !form.value.custom_task_id) {
    modalError.value = 'Nom, VM cible et ID de tâche sont obligatoires.'
    return
  }
  saving.value = true
  modalError.value = ''
  try {
    await api.updateGitWebhook(id, form.value)
    showModal.value = false
    await load()
  } catch (e) {
    modalError.value = e.response?.data?.error || 'Erreur'
  } finally {
    saving.value = false
  }
}

function onSecretRegenerated(secret) {
  revealedSecret.value = secret
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
  const map = { pending: 'bg-yellow-lt text-yellow', running: 'bg-blue-lt text-blue', completed: 'bg-success-lt text-success', failed: 'bg-danger-lt text-danger', skipped: 'bg-secondary-lt text-secondary' }
  return map[status] || 'bg-secondary-lt text-secondary'
}

onMounted(load)
</script>
