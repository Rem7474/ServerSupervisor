<template>
  <div>
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <h2 class="page-title">Git / Automatisation</h2>
          <div class="text-muted">Webhooks entrants et suivi de releases pour déclencher des scripts sur vos VMs.</div>
        </div>
        <button class="btn btn-primary" @click="activeTab === 'webhooks' ? openCreateWebhook() : openCreateTracker()">
          <svg xmlns="http://www.w3.org/2000/svg" class="icon me-1" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M12 5v14M5 12h14"/>
          </svg>
          {{ activeTab === 'webhooks' ? 'Nouveau webhook' : 'Nouveau tracker' }}
        </button>
      </div>
    </div>

    <!-- Tabs -->
    <ul class="nav nav-tabs mb-3">
      <li class="nav-item">
        <a class="nav-link" :class="{ active: activeTab === 'webhooks' }" href="#" @click.prevent="activeTab = 'webhooks'">
          <svg xmlns="http://www.w3.org/2000/svg" class="icon me-1" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <circle cx="12" cy="5" r="3"/><circle cx="5" cy="19" r="3"/><circle cx="19" cy="19" r="3"/>
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v3m0 0l-4.5 5.5M12 11l4.5 5.5"/>
          </svg>
          Webhooks entrants
          <span v-if="webhooks.length" class="badge bg-secondary ms-1">{{ webhooks.length }}</span>
        </a>
      </li>
      <li class="nav-item">
        <a class="nav-link" :class="{ active: activeTab === 'trackers' }" href="#" @click.prevent="activeTab = 'trackers'">
          <svg xmlns="http://www.w3.org/2000/svg" class="icon me-1" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path d="M22 12h-4l-3 9L9 3l-3 9H2"/>
          </svg>
          Suivi de releases
          <span v-if="trackers.length" class="badge bg-secondary ms-1">{{ trackers.length }}</span>
        </a>
      </li>
    </ul>

    <div v-if="error" class="alert alert-danger mb-3">{{ error }}</div>

    <!-- ========== TAB: WEBHOOKS ========== -->
    <div v-show="activeTab === 'webhooks'">
      <div v-if="loadingWebhooks" class="text-center py-5">
        <div class="spinner-border text-primary" role="status"></div>
      </div>

      <div v-else-if="webhooks.length === 0" class="card">
        <div class="card-body text-center py-5 text-muted">
          <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24" class="mb-3 d-block mx-auto opacity-50">
            <circle cx="12" cy="5" r="3"/><circle cx="5" cy="19" r="3"/><circle cx="19" cy="19" r="3"/>
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v3m0 0l-4.5 5.5M12 11l4.5 5.5"/>
          </svg>
          <p class="mb-2">Aucun webhook configuré.</p>
          <p class="text-muted small">Recevez des événements depuis GitHub, GitLab, Gitea ou Forgejo pour déclencher des scripts sur vos VMs.</p>
          <button class="btn btn-sm btn-primary" @click="openCreateWebhook">Créer le premier webhook</button>
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
                <span class="ms-1 text-muted"><RelativeTime :date="wh.last_execution.triggered_at" /></span>
              </div>
              <div v-else class="mt-2 pt-2 border-top small text-muted">Jamais déclenché</div>
            </div>
            <div class="card-footer d-flex gap-2">
              <router-link :to="`/git-webhooks/${wh.id}`" class="btn btn-sm btn-outline-primary">Détails</router-link>
              <button class="btn btn-sm btn-outline-secondary" @click="openEditWebhook(wh)">Modifier</button>
              <button class="btn btn-sm" :class="wh.enabled ? 'btn-outline-warning' : 'btn-outline-success'" @click="toggleWebhook(wh)">
                {{ wh.enabled ? 'Désactiver' : 'Activer' }}
              </button>
              <button class="btn btn-sm btn-outline-danger ms-auto" @click="confirmDeleteWebhook(wh)">
                <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                  <polyline points="3,6 5,6 21,6"/><path d="M19 6l-1 14H6L5 6"/><path d="M10 11v6M14 11v6"/><path d="M9 6V4h6v2"/>
                </svg>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ========== TAB: RELEASE TRACKERS ========== -->
    <div v-show="activeTab === 'trackers'">
      <div v-if="loadingTrackers" class="text-center py-5">
        <div class="spinner-border text-primary" role="status"></div>
      </div>

      <div v-else-if="trackers.length === 0" class="card">
        <div class="card-body text-center py-5 text-muted">
          <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24" class="mb-3 d-block mx-auto opacity-50">
            <path d="M22 12h-4l-3 9L9 3l-3 9H2"/>
          </svg>
          <p class="mb-2">Aucun tracker de release configuré.</p>
          <p class="text-muted small">Surveillez les releases de dépôts externes et déclenchez automatiquement un script sur une VM lors d'une nouvelle version.</p>
          <button class="btn btn-sm btn-primary" @click="openCreateTracker">Créer le premier tracker</button>
        </div>
      </div>

      <div v-else class="row row-cards">
        <div v-for="t in trackers" :key="t.id" class="col-md-6 col-xl-4">
          <div class="card h-100" :class="{ 'opacity-50': !t.enabled }">
            <div class="card-header">
              <div class="d-flex align-items-center gap-2 flex-grow-1 min-w-0">
                <span class="badge" :class="providerBadge(t.provider)">{{ t.provider }}</span>
                <span class="fw-medium text-truncate">{{ t.name }}</span>
              </div>
              <div class="ms-auto">
                <span v-if="!t.enabled" class="badge bg-secondary">Désactivé</span>
              </div>
            </div>
            <div class="card-body">
              <div class="mb-2 small">
                <div class="d-flex gap-2 mb-1">
                  <span class="text-muted" style="min-width:60px">Repo</span>
                  <a :href="repoURL(t)" target="_blank" class="link-primary text-truncate">{{ t.repo_owner }}/{{ t.repo_name }}</a>
                </div>
                <div class="d-flex gap-2 mb-1">
                  <span class="text-muted" style="min-width:60px">VM</span>
                  <span class="text-truncate">{{ t.host_name || t.host_id }}</span>
                </div>
                <div class="d-flex gap-2 mb-1">
                  <span class="text-muted" style="min-width:60px">Tâche</span>
                  <code class="small text-truncate">{{ t.custom_task_id }}</code>
                </div>
                <div v-if="t.last_release_tag" class="d-flex gap-2 mb-1">
                  <span class="text-muted" style="min-width:60px">Dernière</span>
                  <span class="badge bg-green-lt text-green">{{ t.last_release_tag }}</span>
                </div>
              </div>
              <div class="mt-2 pt-2 border-top small">
                <template v-if="t.last_execution">
                  <span class="text-muted">Dernière exécution :</span>
                  <span class="ms-1 badge" :class="execStatusBadge(t.last_execution.status)">{{ t.last_execution.status }}</span>
                  <span class="ms-1 text-muted"><RelativeTime :date="t.last_execution.triggered_at" /></span>
                </template>
                <template v-else-if="t.last_checked_at">
                  <span class="text-muted">Dernière vérif : <RelativeTime :date="t.last_checked_at" /></span>
                  <span v-if="t.last_error" class="ms-1 badge bg-danger" :title="t.last_error">erreur</span>
                  <span v-else-if="!t.last_release_tag" class="ms-1 badge bg-warning text-dark">aucune release trouvée</span>
                </template>
                <template v-else>
                  <span class="text-muted">En attente du premier check...</span>
                </template>
              </div>
            </div>
            <div class="card-footer d-flex gap-2">
              <router-link :to="`/release-trackers/${t.id}`" class="btn btn-sm btn-outline-primary">Détails</router-link>
              <button class="btn btn-sm btn-outline-secondary" @click="openEditTracker(t)">Modifier</button>
              <button class="btn btn-sm btn-outline-info" @click="checkNow(t)" title="Vérifier maintenant">
                <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                  <polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 11-2.12-9.36L23 10"/>
                </svg>
              </button>
              <button class="btn btn-sm" :class="t.enabled ? 'btn-outline-warning' : 'btn-outline-success'" @click="toggleTracker(t)">
                {{ t.enabled ? 'Désactiver' : 'Activer' }}
              </button>
              <button class="btn btn-sm btn-outline-danger ms-auto" @click="confirmDeleteTracker(t)">
                <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                  <polyline points="3,6 5,6 21,6"/><path d="M19 6l-1 14H6L5 6"/><path d="M10 11v6M14 11v6"/><path d="M9 6V4h6v2"/>
                </svg>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ========== WEBHOOK MODAL ========== -->
    <div v-if="showWebhookModal" class="modal modal-blur show d-block" style="background:rgba(0,0,0,.5)">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ editingWebhook ? 'Modifier le webhook' : 'Nouveau webhook Git' }}</h5>
            <button type="button" class="btn-close" @click="closeWebhookModal"></button>
          </div>
          <div class="modal-body">
            <div v-if="modalError" class="alert alert-danger">{{ modalError }}</div>
            <div class="row g-3">
              <div class="col-12">
                <label class="form-label required">Nom</label>
                <input type="text" class="form-control" v-model="webhookForm.name" placeholder="ex: Deploy mon-app">
              </div>
              <div class="col-md-6">
                <label class="form-label required">Provider</label>
                <select class="form-select" v-model="webhookForm.provider">
                  <option value="github">GitHub</option>
                  <option value="gitlab">GitLab</option>
                  <option value="gitea">Gitea</option>
                  <option value="forgejo">Forgejo</option>
                  <option value="custom">Custom</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label">Événement</label>
                <select class="form-select" v-model="webhookForm.event_filter">
                  <option value="push">push</option>
                  <option value="tag">tag / create</option>
                  <option value="release">release</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label">Filtre repo <span class="text-muted">(optionnel)</span></label>
                <input type="text" class="form-control" v-model="webhookForm.repo_filter" placeholder="ex: monorg/mon-app">
              </div>
              <div class="col-md-6">
                <label class="form-label">Filtre branche <span class="text-muted">(optionnel)</span></label>
                <input type="text" class="form-control" v-model="webhookForm.branch_filter" placeholder="ex: main">
              </div>
              <div class="col-md-6">
                <label class="form-label required">VM cible</label>
                <select class="form-select" v-model="webhookForm.host_id">
                  <option value="">-- Sélectionner un hôte --</option>
                  <option v-for="h in hosts" :key="h.id" :value="h.id">{{ h.name }}</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label required">ID Tâche (tasks.yaml)</label>
                <input type="text" class="form-control" v-model="webhookForm.custom_task_id" placeholder="ex: deploy-mon-app">
                <div class="form-hint">Correspond à l'<code>id</code> dans le fichier <code>tasks.yaml</code> de l'agent.</div>
              </div>
              <div class="col-12">
                <label class="form-label">Notifications</label>
                <div class="d-flex flex-wrap gap-3 mt-1">
                  <label class="form-check">
                    <input class="form-check-input" type="checkbox" v-model="webhookForm.notify_on_success">
                    <span class="form-check-label">En cas de succès</span>
                  </label>
                  <label class="form-check">
                    <input class="form-check-input" type="checkbox" v-model="webhookForm.notify_on_failure">
                    <span class="form-check-label">En cas d'échec</span>
                  </label>
                </div>
                <div class="d-flex flex-wrap gap-3 mt-2">
                  <label v-for="ch in ['smtp','ntfy','browser']" :key="ch" class="form-check">
                    <input class="form-check-input" type="checkbox" :value="ch" v-model="webhookForm.notify_channels">
                    <span class="form-check-label">{{ ch }}</span>
                  </label>
                </div>
              </div>
              <div class="col-12">
                <label class="form-check form-switch">
                  <input class="form-check-input" type="checkbox" v-model="webhookForm.enabled">
                  <span class="form-check-label">Activé</span>
                </label>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button class="btn btn-secondary" @click="closeWebhookModal">Annuler</button>
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

    <!-- ========== TRACKER MODAL ========== -->
    <div v-if="showTrackerModal" class="modal modal-blur show d-block" style="background:rgba(0,0,0,.5)">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ editingTracker ? 'Modifier le tracker' : 'Nouveau tracker de release' }}</h5>
            <button type="button" class="btn-close" @click="closeTrackerModal"></button>
          </div>
          <div class="modal-body">
            <div v-if="modalError" class="alert alert-danger">{{ modalError }}</div>
            <div class="alert alert-info small mb-3">
              Le tracker surveille automatiquement les nouvelles releases d'un dépôt externe et déclenche un script sur une VM dès qu'une nouvelle version est publiée.
            </div>
            <div class="row g-3">
              <div class="col-12">
                <label class="form-label required">Nom</label>
                <input type="text" class="form-control" v-model="trackerForm.name" placeholder="ex: Mise à jour Home Assistant">
              </div>
              <div class="col-md-4">
                <label class="form-label required">Provider</label>
                <select class="form-select" v-model="trackerForm.provider">
                  <option value="github">GitHub</option>
                  <option value="gitlab">GitLab</option>
                  <option value="gitea">Gitea (Codeberg)</option>
                </select>
              </div>
              <div class="col-md-4">
                <label class="form-label required">Owner / Org</label>
                <input type="text" class="form-control" v-model="trackerForm.repo_owner" placeholder="ex: home-assistant">
              </div>
              <div class="col-md-4">
                <label class="form-label required">Dépôt</label>
                <input type="text" class="form-control" v-model="trackerForm.repo_name" placeholder="ex: core">
              </div>
              <div class="col-md-6">
                <label class="form-label required">VM cible</label>
                <select class="form-select" v-model="trackerForm.host_id">
                  <option value="">-- Sélectionner un hôte --</option>
                  <option v-for="h in hosts" :key="h.id" :value="h.id">{{ h.name }}</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label required">ID Tâche (tasks.yaml)</label>
                <input type="text" class="form-control" v-model="trackerForm.custom_task_id" placeholder="ex: update-home-assistant">
                <div class="form-hint">Correspond à l'<code>id</code> dans le fichier <code>tasks.yaml</code> de l'agent.</div>
              </div>
              <div class="col-12">
                <label class="form-label">Notifications</label>
                <div class="d-flex flex-wrap gap-3 mt-1">
                  <label class="form-check">
                    <input class="form-check-input" type="checkbox" v-model="trackerForm.notify_on_release">
                    <span class="form-check-label">Notifier à chaque nouvelle release</span>
                  </label>
                </div>
                <div class="d-flex flex-wrap gap-3 mt-2">
                  <label v-for="ch in ['smtp','ntfy','browser']" :key="ch" class="form-check">
                    <input class="form-check-input" type="checkbox" :value="ch" v-model="trackerForm.notify_channels">
                    <span class="form-check-label">{{ ch }}</span>
                  </label>
                </div>
              </div>
              <div class="col-12">
                <label class="form-check form-switch">
                  <input class="form-check-input" type="checkbox" v-model="trackerForm.enabled">
                  <span class="form-check-label">Activé</span>
                </label>
              </div>
            </div>

            <!-- Variables disponibles -->
            <div class="mt-3 pt-3 border-top">
              <div class="text-muted small mb-2">Variables injectées dans votre script :</div>
              <div class="table-responsive">
                <table class="table table-sm mb-0">
                  <tbody>
                    <tr v-for="v in trackerEnvVars" :key="v.name">
                      <td class="py-1"><code class="small">{{ v.name }}</code></td>
                      <td class="py-1 text-muted small">{{ v.desc }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button class="btn btn-secondary" @click="closeTrackerModal">Annuler</button>
            <button class="btn btn-primary" @click="saveTracker" :disabled="saving">
              {{ saving ? 'Enregistrement...' : (editingTracker ? 'Mettre à jour' : 'Créer') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import api from '../api'
import RelativeTime from '../components/RelativeTime.vue'
import WebhookUrlCard from '../components/WebhookUrlCard.vue'

const dialog = useConfirmDialog()

const activeTab = ref('webhooks')

// Shared
const hosts = ref([])
const error = ref('')
const saving = ref(false)
const modalError = ref('')

// ========== Webhooks ==========
const webhooks = ref([])
const loadingWebhooks = ref(false)
const showWebhookModal = ref(false)
const editingWebhook = ref(null)
const newWebhookSecret = ref('')
const newWebhookId = ref('')

const defaultWebhookForm = () => ({
  name: '', provider: 'github', event_filter: 'push',
  repo_filter: '', branch_filter: '', host_id: '',
  custom_task_id: '', notify_channels: [],
  notify_on_success: false, notify_on_failure: true, enabled: true,
})
const webhookForm = ref(defaultWebhookForm())

// ========== Trackers ==========
const trackers = ref([])
const loadingTrackers = ref(false)
const showTrackerModal = ref(false)
const editingTracker = ref(null)

const trackerEnvVars = [
  { name: 'SS_REPO_NAME',    desc: 'owner/repo (ex: home-assistant/core)' },
  { name: 'SS_TAG_NAME',     desc: 'Tag de la nouvelle release (ex: v1.2.3)' },
  { name: 'SS_RELEASE_URL',  desc: 'URL de la release sur le provider' },
  { name: 'SS_RELEASE_NAME', desc: 'Titre de la release' },
  { name: 'SS_TRACKER_NAME', desc: 'Nom du tracker dans ServerSupervisor' },
]

const defaultTrackerForm = () => ({
  name: '', provider: 'github', repo_owner: '', repo_name: '',
  host_id: '', custom_task_id: '',
  notify_channels: [], notify_on_release: true, enabled: true,
})
const trackerForm = ref(defaultTrackerForm())

// ========== Data loading ==========

async function loadAll() {
  await Promise.all([loadWebhooks(), loadTrackers(), loadHosts()])
}

async function loadWebhooks() {
  loadingWebhooks.value = true
  try {
    const res = await api.getGitWebhooks()
    webhooks.value = res.data.webhooks || []
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur lors du chargement des webhooks'
  } finally {
    loadingWebhooks.value = false
  }
}

async function loadTrackers() {
  loadingTrackers.value = true
  try {
    const res = await api.getReleaseTrackers()
    trackers.value = res.data.trackers || []
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur lors du chargement des trackers'
  } finally {
    loadingTrackers.value = false
  }
}

async function loadHosts() {
  try {
    const res = await api.getHosts()
    hosts.value = res.data || []
  } catch { /* ignore */ }
}

// ========== Webhook actions ==========

function openCreateWebhook() {
  editingWebhook.value = null
  webhookForm.value = defaultWebhookForm()
  modalError.value = ''
  showWebhookModal.value = true
}

function openEditWebhook(wh) {
  editingWebhook.value = wh
  webhookForm.value = {
    name: wh.name, provider: wh.provider, event_filter: wh.event_filter,
    repo_filter: wh.repo_filter, branch_filter: wh.branch_filter,
    host_id: wh.host_id, custom_task_id: wh.custom_task_id,
    notify_channels: [...(wh.notify_channels || [])],
    notify_on_success: wh.notify_on_success,
    notify_on_failure: wh.notify_on_failure,
    enabled: wh.enabled,
  }
  modalError.value = ''
  showWebhookModal.value = true
}

function closeWebhookModal() {
  showWebhookModal.value = false
  editingWebhook.value = null
}

async function saveWebhook() {
  if (!webhookForm.value.name || !webhookForm.value.host_id || !webhookForm.value.custom_task_id) {
    modalError.value = 'Nom, VM cible et ID de tâche sont obligatoires.'
    return
  }
  saving.value = true
  modalError.value = ''
  try {
    if (editingWebhook.value) {
      await api.updateGitWebhook(editingWebhook.value.id, webhookForm.value)
    } else {
      const res = await api.createGitWebhook(webhookForm.value)
      const created = res.data.webhook
      if (created?.secret) {
        newWebhookId.value = created.id
        newWebhookSecret.value = created.secret
      }
    }
    closeWebhookModal()
    await loadWebhooks()
  } catch (e) {
    modalError.value = e.response?.data?.error || 'Erreur'
  } finally {
    saving.value = false
  }
}

async function toggleWebhook(wh) {
  try {
    await api.updateGitWebhook(wh.id, { ...wh, enabled: !wh.enabled })
    await loadWebhooks()
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur'
  }
}

async function confirmDeleteWebhook(wh) {
  const ok = await dialog.confirm({
    title: `Supprimer le webhook "${wh.name}" ?`,
    message: 'Toutes les exécutions associées seront également supprimées.',
    variant: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteGitWebhook(wh.id)
    await loadWebhooks()
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur lors de la suppression'
  }
}

function closeSecretModal() {
  newWebhookSecret.value = ''
  newWebhookId.value = ''
}

// ========== Tracker actions ==========

function openCreateTracker() {
  editingTracker.value = null
  trackerForm.value = defaultTrackerForm()
  modalError.value = ''
  showTrackerModal.value = true
}

function openEditTracker(t) {
  editingTracker.value = t
  trackerForm.value = {
    name: t.name, provider: t.provider,
    repo_owner: t.repo_owner, repo_name: t.repo_name,
    host_id: t.host_id, custom_task_id: t.custom_task_id,
    notify_channels: [...(t.notify_channels || [])],
    notify_on_release: t.notify_on_release,
    enabled: t.enabled,
  }
  modalError.value = ''
  showTrackerModal.value = true
}

function closeTrackerModal() {
  showTrackerModal.value = false
  editingTracker.value = null
}

async function saveTracker() {
  if (!trackerForm.value.name || !trackerForm.value.repo_owner || !trackerForm.value.repo_name) {
    modalError.value = 'Nom, owner et dépôt sont obligatoires.'
    return
  }
  if (!trackerForm.value.host_id || !trackerForm.value.custom_task_id) {
    modalError.value = 'VM cible et ID de tâche sont obligatoires.'
    return
  }
  saving.value = true
  modalError.value = ''
  try {
    if (editingTracker.value) {
      await api.updateReleaseTracker(editingTracker.value.id, trackerForm.value)
    } else {
      await api.createReleaseTracker(trackerForm.value)
    }
    closeTrackerModal()
    await loadTrackers()
  } catch (e) {
    modalError.value = e.response?.data?.error || 'Erreur'
  } finally {
    saving.value = false
  }
}

async function toggleTracker(t) {
  try {
    await api.updateReleaseTracker(t.id, { ...t, enabled: !t.enabled })
    await loadTrackers()
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur'
  }
}

async function checkNow(t) {
  try {
    await api.checkReleaseTrackerNow(t.id)
    setTimeout(() => loadTrackers(), 2000)
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur'
  }
}

async function confirmDeleteTracker(t) {
  const ok = await dialog.confirm({
    title: `Supprimer le tracker "${t.name}" ?`,
    message: 'Toutes les exécutions associées seront également supprimées.',
    variant: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteReleaseTracker(t.id)
    await loadTrackers()
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur lors de la suppression'
  }
}

// ========== Helpers ==========

function repoURL(t) {
  switch (t.provider) {
    case 'gitlab': return `https://gitlab.com/${t.repo_owner}/${t.repo_name}`
    case 'gitea': return `https://codeberg.org/${t.repo_owner}/${t.repo_name}`
    default: return `https://github.com/${t.repo_owner}/${t.repo_name}`
  }
}

function providerBadge(provider) {
  const map = {
    github: 'bg-dark', gitlab: 'bg-warning text-dark',
    gitea: 'bg-success', forgejo: 'bg-info text-dark', custom: 'bg-secondary',
  }
  return map[provider] || 'bg-secondary'
}

function execStatusBadge(status) {
  const map = {
    pending: 'bg-yellow text-dark', running: 'bg-blue',
    completed: 'bg-success', failed: 'bg-danger', skipped: 'bg-secondary',
  }
  return map[status] || 'bg-secondary'
}

onMounted(loadAll)
</script>
