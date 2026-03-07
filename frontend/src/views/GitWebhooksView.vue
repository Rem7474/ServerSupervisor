<template>
  <div>
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <h2 class="page-title">Git Webhooks</h2>
          <div class="text-muted">Déclenchez automatiquement des scripts sur vos VMs depuis GitHub, GitLab, Gitea ou Forgejo.</div>
        </div>
        <button class="btn btn-primary" @click="openCreate">
          <svg xmlns="http://www.w3.org/2000/svg" class="icon me-1" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M12 5v14M5 12h14"/>
          </svg>
          Nouveau webhook
        </button>
      </div>
    </div>

    <div v-if="error" class="alert alert-danger mb-3">{{ error }}</div>

    <div v-if="loading" class="text-center py-5">
      <div class="spinner-border text-primary" role="status"></div>
    </div>

    <div v-else-if="webhooks.length === 0" class="card">
      <div class="card-body text-center py-5 text-muted">
        <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24" class="mb-3 d-block mx-auto opacity-50">
          <circle cx="12" cy="5" r="3"/><circle cx="5" cy="19" r="3"/><circle cx="19" cy="19" r="3"/>
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v3m0 0l-4.5 5.5M12 11l4.5 5.5"/>
        </svg>
        <p class="mb-2">Aucun webhook configuré.</p>
        <button class="btn btn-sm btn-primary" @click="openCreate">Créer le premier webhook</button>
      </div>
    </div>

    <div v-else class="row row-cards">
      <div v-for="wh in webhooks" :key="wh.id" class="col-md-6 col-xl-4">
        <div class="card h-100" :class="{ 'opacity-50': !wh.enabled }">
          <div class="card-header">
            <div class="d-flex align-items-center gap-2 flex-grow-1 min-w-0">
              <span class="badge" :class="providerBadge(wh.provider)">{{ wh.provider }}</span>
              <span class="fw-medium text-truncate">{{ wh.name }}</span>
            </div>
            <div class="ms-auto d-flex gap-1">
              <span v-if="!wh.enabled" class="badge bg-secondary">Désactivé</span>
            </div>
          </div>
          <div class="card-body">
            <div class="mb-2 small">
              <div class="d-flex gap-2 mb-1">
                <span class="text-muted" style="min-width:60px">Repo</span>
                <span class="text-truncate">{{ wh.repo_filter || '<tous>' }}</span>
              </div>
              <div class="d-flex gap-2 mb-1">
                <span class="text-muted" style="min-width:60px">Branche</span>
                <span>{{ wh.branch_filter || '<toutes>' }}</span>
              </div>
              <div class="d-flex gap-2 mb-1">
                <span class="text-muted" style="min-width:60px">VM</span>
                <span class="text-truncate">{{ wh.host_name || wh.host_id }}</span>
              </div>
              <div class="d-flex gap-2 mb-1">
                <span class="text-muted" style="min-width:60px">Tâche</span>
                <code class="small text-truncate">{{ wh.custom_task_id }}</code>
              </div>
            </div>
            <div v-if="wh.last_execution" class="mt-2 pt-2 border-top small">
              <span class="text-muted">Dernière exécution :</span>
              <span class="ms-1 badge" :class="execStatusBadge(wh.last_execution.status)">{{ wh.last_execution.status }}</span>
              <span class="ms-1 text-muted">
                <RelativeTime :date="wh.last_execution.triggered_at" />
              </span>
            </div>
            <div v-else class="mt-2 pt-2 border-top small text-muted">Jamais déclenché</div>
          </div>
          <div class="card-footer d-flex gap-2">
            <router-link :to="`/git-webhooks/${wh.id}`" class="btn btn-sm btn-outline-primary">Détails</router-link>
            <button class="btn btn-sm btn-outline-secondary" @click="openEdit(wh)">Modifier</button>
            <button class="btn btn-sm" :class="wh.enabled ? 'btn-outline-warning' : 'btn-outline-success'" @click="toggleEnabled(wh)">
              {{ wh.enabled ? 'Désactiver' : 'Activer' }}
            </button>
            <button class="btn btn-sm btn-outline-danger ms-auto" @click="confirmDelete(wh)">
              <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <polyline points="3,6 5,6 21,6"/><path d="M19 6l-1 14H6L5 6"/><path d="M10 11v6M14 11v6"/><path d="M9 6V4h6v2"/>
              </svg>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Create / Edit Modal -->
    <div v-if="showModal" class="modal modal-blur show d-block" style="background:rgba(0,0,0,.5)">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ editingWebhook ? 'Modifier le webhook' : 'Nouveau webhook Git' }}</h5>
            <button type="button" class="btn-close" @click="closeModal"></button>
          </div>
          <div class="modal-body">
            <div v-if="modalError" class="alert alert-danger">{{ modalError }}</div>
            <div class="row g-3">
              <!-- Name -->
              <div class="col-12">
                <label class="form-label required">Nom</label>
                <input type="text" class="form-control" v-model="form.name" placeholder="ex: Deploy mon-app">
              </div>
              <!-- Provider + Event -->
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
              <!-- Filters -->
              <div class="col-md-6">
                <label class="form-label">Filtre repo <span class="text-muted">(optionnel)</span></label>
                <input type="text" class="form-control" v-model="form.repo_filter" placeholder="ex: monorg/mon-app ou monorg/*">
              </div>
              <div class="col-md-6">
                <label class="form-label">Filtre branche <span class="text-muted">(optionnel)</span></label>
                <input type="text" class="form-control" v-model="form.branch_filter" placeholder="ex: main">
              </div>
              <!-- Host + Task -->
              <div class="col-md-6">
                <label class="form-label required">VM cible</label>
                <select class="form-select" v-model="form.host_id">
                  <option value="">-- Sélectionner un hôte --</option>
                  <option v-for="h in hosts" :key="h.id" :value="h.id">{{ h.name }}</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label required">ID Tâche (tasks.yaml)</label>
                <input type="text" class="form-control" v-model="form.custom_task_id" placeholder="ex: deploy-mon-app">
                <div class="form-hint">Correspond à l'<code>id</code> dans le fichier <code>tasks.yaml</code> de l'agent.</div>
              </div>
              <!-- Notifications -->
              <div class="col-12">
                <label class="form-label">Notifications</label>
                <div class="d-flex flex-wrap gap-3 mt-1">
                  <label class="form-check">
                    <input class="form-check-input" type="checkbox" v-model="form.notify_on_success">
                    <span class="form-check-label">En cas de succès</span>
                  </label>
                  <label class="form-check">
                    <input class="form-check-input" type="checkbox" v-model="form.notify_on_failure">
                    <span class="form-check-label">En cas d'échec</span>
                  </label>
                </div>
                <div class="d-flex flex-wrap gap-3 mt-2">
                  <label v-for="ch in ['smtp','ntfy','browser']" :key="ch" class="form-check">
                    <input class="form-check-input" type="checkbox" :value="ch" v-model="form.notify_channels">
                    <span class="form-check-label">{{ ch }}</span>
                  </label>
                </div>
              </div>
              <!-- Enabled -->
              <div class="col-12">
                <label class="form-check form-switch">
                  <input class="form-check-input" type="checkbox" v-model="form.enabled">
                  <span class="form-check-label">Activé</span>
                </label>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button class="btn btn-secondary" @click="closeModal">Annuler</button>
            <button class="btn btn-primary" @click="saveWebhook" :disabled="saving">
              {{ saving ? 'Enregistrement...' : (editingWebhook ? 'Mettre à jour' : 'Créer') }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Post-creation secret reveal modal -->
    <div v-if="newWebhookSecret" class="modal modal-blur show d-block" style="background:rgba(0,0,0,.7)">
      <div class="modal-dialog modal-md">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">Webhook créé</h5>
          </div>
          <div class="modal-body">
            <div class="alert alert-warning">
              Copiez ce secret maintenant — il ne sera plus affiché en clair.
            </div>
            <WebhookUrlCard :webhook-id="newWebhookId" :secret="newWebhookSecret" :initial-secret="true" />
          </div>
          <div class="modal-footer">
            <button class="btn btn-primary" @click="closeSecretModal">J'ai copié le secret</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import api from '../api'
import RelativeTime from '../components/RelativeTime.vue'
import WebhookUrlCard from '../components/WebhookUrlCard.vue'

const auth = useAuthStore()
const dialog = useConfirmDialog()

const webhooks = ref([])
const hosts = ref([])
const loading = ref(false)
const error = ref('')
const showModal = ref(false)
const editingWebhook = ref(null)
const saving = ref(false)
const modalError = ref('')

// Post-creation secret display
const newWebhookSecret = ref('')
const newWebhookId = ref('')

const defaultForm = () => ({
  name: '',
  provider: 'github',
  event_filter: 'push',
  repo_filter: '',
  branch_filter: '',
  host_id: '',
  custom_task_id: '',
  notify_channels: [],
  notify_on_success: false,
  notify_on_failure: true,
  enabled: true,
})
const form = ref(defaultForm())

async function loadData() {
  loading.value = true
  error.value = ''
  try {
    const [whRes, hostsRes] = await Promise.all([api.getGitWebhooks(), api.getHosts()])
    webhooks.value = whRes.data.webhooks || []
    hosts.value = hostsRes.data || []
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur lors du chargement'
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingWebhook.value = null
  form.value = defaultForm()
  modalError.value = ''
  showModal.value = true
}

function openEdit(wh) {
  editingWebhook.value = wh
  form.value = {
    name: wh.name,
    provider: wh.provider,
    event_filter: wh.event_filter,
    repo_filter: wh.repo_filter,
    branch_filter: wh.branch_filter,
    host_id: wh.host_id,
    custom_task_id: wh.custom_task_id,
    notify_channels: [...(wh.notify_channels || [])],
    notify_on_success: wh.notify_on_success,
    notify_on_failure: wh.notify_on_failure,
    enabled: wh.enabled,
  }
  modalError.value = ''
  showModal.value = true
}

function closeModal() {
  showModal.value = false
  editingWebhook.value = null
}

async function saveWebhook() {
  if (!form.value.name || !form.value.host_id || !form.value.custom_task_id) {
    modalError.value = 'Nom, VM cible et ID de tâche sont obligatoires.'
    return
  }
  saving.value = true
  modalError.value = ''
  try {
    if (editingWebhook.value) {
      await api.updateGitWebhook(editingWebhook.value.id, form.value)
    } else {
      const res = await api.createGitWebhook(form.value)
      const created = res.data.webhook
      if (created?.secret) {
        newWebhookId.value = created.id
        newWebhookSecret.value = created.secret
      }
    }
    closeModal()
    await loadData()
  } catch (e) {
    modalError.value = e.response?.data?.error || 'Erreur'
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(wh) {
  try {
    await api.updateGitWebhook(wh.id, { ...wh, enabled: !wh.enabled })
    await loadData()
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur'
  }
}

async function confirmDelete(wh) {
  const ok = await dialog.confirm({
    title: `Supprimer le webhook "${wh.name}" ?`,
    message: 'Toutes les exécutions associées seront également supprimées.',
    variant: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteGitWebhook(wh.id)
    await loadData()
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur lors de la suppression'
  }
}

function closeSecretModal() {
  newWebhookSecret.value = ''
  newWebhookId.value = ''
}

function providerBadge(provider) {
  const map = {
    github: 'bg-dark',
    gitlab: 'bg-warning text-dark',
    gitea: 'bg-success',
    forgejo: 'bg-info text-dark',
    custom: 'bg-secondary',
  }
  return map[provider] || 'bg-secondary'
}

function execStatusBadge(status) {
  const map = {
    pending: 'bg-yellow text-dark',
    running: 'bg-blue',
    completed: 'bg-success',
    failed: 'bg-danger',
    skipped: 'bg-secondary',
  }
  return map[status] || 'bg-secondary'
}

onMounted(loadData)
</script>
