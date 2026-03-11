<template>
  <div>
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <h2 class="page-title">Git / Automatisation</h2>
          <div class="text-muted">Webhooks entrants et suivi de releases pour declencher des scripts sur vos VMs.</div>
        </div>
        <button class="btn btn-primary" @click="activeTab === 'webhooks' ? openCreateWebhook() : openCreateTracker()">
          <svg xmlns="http://www.w3.org/2000/svg" class="icon me-1" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M12 5v14M5 12h14"/>
          </svg>
          {{ activeTab === 'webhooks' ? 'Nouveau webhook' : 'Nouveau tracker' }}
        </button>
      </div>
    </div>

    <ul class="nav nav-tabs mb-3">
      <li class="nav-item">
        <a class="nav-link" :class="{ active: activeTab === 'webhooks' }" href="#" @click.prevent="activeTab = 'webhooks'">
          <svg xmlns="http://www.w3.org/2000/svg" class="icon me-1" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <circle cx="12" cy="5" r="3"/><circle cx="5" cy="19" r="3"/><circle cx="19" cy="19" r="3"/>
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v3m0 0l-4.5 5.5M12 11l4.5 5.5"/>
          </svg>
          Webhooks entrants
          <span v-if="webhooks.length" class="badge bg-azure-lt text-azure ms-1">{{ webhooks.length }}</span>
        </a>
      </li>
      <li class="nav-item">
        <a class="nav-link" :class="{ active: activeTab === 'trackers' }" href="#" @click.prevent="activeTab = 'trackers'">
          <svg xmlns="http://www.w3.org/2000/svg" class="icon me-1" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path d="M22 12h-4l-3 9L9 3l-3 9H2"/>
          </svg>
          Suivi de releases
          <span v-if="trackers.length" class="badge bg-azure-lt text-azure ms-1">{{ trackers.length }}</span>
        </a>
      </li>
    </ul>

    <div v-if="error" class="alert alert-danger mb-3">{{ error }}</div>

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
          <p class="mb-2">Aucun webhook configure.</p>
          <p class="text-muted small">Recevez des evenements depuis GitHub, GitLab, Gitea ou Forgejo pour declencher des scripts sur vos VMs.</p>
          <button class="btn btn-sm btn-primary" @click="openCreateWebhook">Creer le premier webhook</button>
        </div>
      </div>

      <template v-else>
        <div class="row row-cards">
          <div v-for="webhook in webhooks" :key="webhook.id" class="col-md-6 col-xl-4">
            <div class="card h-100" :class="{ 'opacity-50': !webhook.enabled }">
              <div class="card-header">
                <div class="d-flex align-items-center gap-2 flex-grow-1 min-w-0">
                  <span class="badge" :class="providerBadge(webhook.provider)">{{ webhook.provider }}</span>
                  <span class="fw-medium text-truncate">{{ webhook.name }}</span>
                </div>
                <div class="ms-auto d-flex gap-1">
                  <span v-if="!webhook.enabled" class="badge bg-secondary">Desactive</span>
                </div>
              </div>
              <div class="card-body">
                <div class="mb-2 small">
                  <div class="d-flex gap-2 mb-1">
                    <span class="text-muted" style="min-width:60px">Repo</span>
                    <span class="text-truncate">{{ webhook.repo_filter || '<tous>' }}</span>
                  </div>
                  <div class="d-flex gap-2 mb-1">
                    <span class="text-muted" style="min-width:60px">Branche</span>
                    <span>{{ webhook.branch_filter || '<toutes>' }}</span>
                  </div>
                  <div class="d-flex gap-2 mb-1">
                    <span class="text-muted" style="min-width:60px">VM</span>
                    <span class="text-truncate">{{ webhook.host_name || webhook.host_id }}</span>
                  </div>
                  <div class="d-flex gap-2 mb-1">
                    <span class="text-muted" style="min-width:60px">Tache</span>
                    <code class="small text-truncate">{{ webhook.custom_task_id }}</code>
                  </div>
                </div>
                <div v-if="webhook.last_execution" class="mt-2 pt-2 border-top small">
                  <span class="text-muted">Derniere execution :</span>
                  <span class="ms-1 badge" :class="execStatusBadge(webhook.last_execution.status)">{{ webhook.last_execution.status }}</span>
                  <span class="ms-1 text-muted">{{ formatRelative(webhook.last_execution.triggered_at) }}</span>
                </div>
                <div v-else class="mt-2 pt-2 border-top small text-muted">Jamais declenche</div>
              </div>
              <div class="card-footer d-flex gap-2">
                <router-link :to="`/git-webhooks/${webhook.id}`" class="btn btn-sm btn-outline-primary">Details</router-link>
                <button class="btn btn-sm btn-outline-secondary" @click="openEditWebhook(webhook)">Modifier</button>
                <button class="btn btn-sm" :class="webhook.enabled ? 'btn-outline-warning' : 'btn-outline-success'" @click="toggleWebhook(webhook)">
                  {{ webhook.enabled ? 'Desactiver' : 'Activer' }}
                </button>
                <button class="btn btn-sm btn-outline-danger ms-auto" @click="confirmDeleteWebhook(webhook)">
                  <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                    <polyline points="3,6 5,6 21,6"/><path d="M19 6l-1 14H6L5 6"/><path d="M10 11v6M14 11v6"/><path d="M9 6V4h6v2"/>
                  </svg>
                </button>
              </div>
            </div>
          </div>
        </div>

        <WebhookExecutionList
          class="mt-4"
          :executions="recentWebhookExecutions"
          kind="webhook"
          title="Dernieres executions des webhooks"
          empty-text="Aucune execution connue."
        />
      </template>
    </div>

    <div v-show="activeTab === 'trackers'">
      <div v-if="loadingTrackers" class="text-center py-5">
        <div class="spinner-border text-primary" role="status"></div>
      </div>

      <div v-else-if="trackers.length === 0" class="card">
        <div class="card-body text-center py-5 text-muted">
          <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24" class="mb-3 d-block mx-auto opacity-50">
            <path d="M22 12h-4l-3 9L9 3l-3 9H2"/>
          </svg>
          <p class="mb-2">Aucun tracker de release configure.</p>
          <p class="text-muted small">Surveillez les releases de depots externes et declenchez automatiquement un script sur une VM lors d'une nouvelle version.</p>
          <button class="btn btn-sm btn-primary" @click="openCreateTracker">Creer le premier tracker</button>
        </div>
      </div>

      <template v-else>
        <div class="row row-cards">
          <div v-for="tracker in trackers" :key="tracker.id" class="col-md-6 col-xl-4">
            <div class="card h-100" :class="{ 'opacity-50': !tracker.enabled }">
              <div class="card-header">
                <div class="d-flex align-items-center gap-2 flex-grow-1 min-w-0">
                  <span class="badge" :class="providerBadge(tracker.provider)">{{ tracker.provider }}</span>
                  <span class="fw-medium text-truncate">{{ tracker.name }}</span>
                </div>
                <div class="ms-auto">
                  <span v-if="!tracker.enabled" class="badge bg-secondary">Desactive</span>
                </div>
              </div>
              <div class="card-body">
                <div class="mb-2 small">
                  <div class="d-flex gap-2 mb-1">
                    <span class="text-muted" style="min-width:60px">Repo</span>
                    <a :href="repoURL(tracker)" target="_blank" class="link-primary text-truncate">{{ tracker.repo_owner }}/{{ tracker.repo_name }}</a>
                  </div>
                  <div class="d-flex gap-2 mb-1">
                    <span class="text-muted" style="min-width:60px">VM</span>
                    <span class="text-truncate">{{ tracker.host_name || tracker.host_id }}</span>
                  </div>
                  <div class="d-flex gap-2 mb-1">
                    <span class="text-muted" style="min-width:60px">Tache</span>
                    <code class="small text-truncate">{{ tracker.custom_task_id }}</code>
                  </div>
                  <div v-if="tracker.last_release_tag" class="d-flex gap-2 mb-1">
                    <span class="text-muted" style="min-width:60px">Derniere</span>
                    <span class="badge bg-green-lt text-green">{{ tracker.last_release_tag }}</span>
                  </div>
                </div>
                <div class="mt-2 pt-2 border-top small">
                  <template v-if="tracker.last_execution">
                    <span class="text-muted">Derniere execution :</span>
                    <span class="ms-1 badge" :class="execStatusBadge(tracker.last_execution.status)">{{ tracker.last_execution.status }}</span>
                    <span class="ms-1 text-muted">{{ formatRelative(tracker.last_execution.triggered_at) }}</span>
                  </template>
                  <template v-else-if="tracker.last_checked_at">
                    <span class="text-muted">Derniere verif : {{ formatRelative(tracker.last_checked_at) }}</span>
                    <span v-if="tracker.last_error" class="ms-1 badge bg-danger-lt text-danger" :title="tracker.last_error">erreur</span>
                    <span v-else-if="!tracker.last_release_tag" class="ms-1 badge bg-warning-lt text-warning">aucune release trouvee</span>
                  </template>
                  <template v-else>
                    <span class="text-muted">En attente du premier check...</span>
                  </template>
                </div>
              </div>
              <div class="card-footer d-flex gap-2">
                <router-link :to="`/release-trackers/${tracker.id}`" class="btn btn-sm btn-outline-primary">Details</router-link>
                <button class="btn btn-sm btn-outline-secondary" @click="openEditTracker(tracker)">Modifier</button>
                <button class="btn btn-sm btn-outline-info" @click="checkNow(tracker)" title="Verifier maintenant">
                  <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                    <polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 11-2.12-9.36L23 10"/>
                  </svg>
                </button>
                <button class="btn btn-sm" :class="tracker.enabled ? 'btn-outline-warning' : 'btn-outline-success'" @click="toggleTracker(tracker)">
                  {{ tracker.enabled ? 'Desactiver' : 'Activer' }}
                </button>
                <button class="btn btn-sm btn-outline-danger ms-auto" @click="confirmDeleteTracker(tracker)">
                  <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                    <polyline points="3,6 5,6 21,6"/><path d="M19 6l-1 14H6L5 6"/><path d="M10 11v6M14 11v6"/><path d="M9 6V4h6v2"/>
                  </svg>
                </button>
              </div>
            </div>
          </div>
        </div>

        <WebhookExecutionList
          class="mt-4"
          :executions="recentTrackerExecutions"
          kind="tracker"
          title="Dernieres executions des trackers"
          empty-text="Aucune execution connue."
        />
      </template>
    </div>

    <WebhookModal
      :visible="showWebhookModal"
      mode="webhook"
      :item="editingWebhook"
      :hosts="hosts"
      :saving="saving"
      :error="modalError"
      @close="closeWebhookModal"
      @submit="saveWebhook"
    />

    <WebhookModal
      :visible="showTrackerModal"
      mode="tracker"
      :item="editingTracker"
      :hosts="hosts"
      :saving="saving"
      :error="modalError"
      @close="closeTrackerModal"
      @submit="saveTracker"
    />

    <div v-if="newWebhookSecret" class="modal modal-blur show d-block" style="background:rgba(0,0,0,.7)">
      <div class="modal-dialog modal-md">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">Webhook cree</h5>
          </div>
          <div class="modal-body">
            <div class="alert alert-warning">Copiez ce secret maintenant, il ne sera plus affiche en clair.</div>
            <WebhookUrlCard :webhook-id="newWebhookId" :secret="newWebhookSecret" :initial-secret="true" />
          </div>
          <div class="modal-footer">
            <button class="btn btn-primary" @click="closeSecretModal">J'ai copie le secret</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import api from '../api'
import WebhookUrlCard from '../components/WebhookUrlCard.vue'
import WebhookExecutionList from '../components/webhooks/WebhookExecutionList.vue'
import WebhookModal from '../components/webhooks/WebhookModal.vue'
import { useConfirmDialog } from '../composables/useConfirmDialog'

const dialog = useConfirmDialog()

const activeTab = ref('webhooks')
const hosts = ref([])
const error = ref('')
const saving = ref(false)
const modalError = ref('')
const webhooks = ref([])
const loadingWebhooks = ref(false)
const showWebhookModal = ref(false)
const editingWebhook = ref(null)
const newWebhookSecret = ref('')
const newWebhookId = ref('')
const trackers = ref([])
const loadingTrackers = ref(false)
const showTrackerModal = ref(false)
const editingTracker = ref(null)

const recentWebhookExecutions = computed(() =>
  webhooks.value
    .filter((webhook) => webhook.last_execution)
    .map((webhook) => ({
      ...webhook.last_execution,
      sourceId: webhook.id,
      sourceName: webhook.name,
      repo_name: webhook.last_execution.repo_name || webhook.repo_filter || webhook.name,
      branch: webhook.last_execution.branch || webhook.branch_filter || '',
    }))
    .sort((left, right) => new Date(right.triggered_at) - new Date(left.triggered_at))
)

const recentTrackerExecutions = computed(() =>
  trackers.value
    .filter((tracker) => tracker.last_execution)
    .map((tracker) => ({
      ...tracker.last_execution,
      sourceId: tracker.id,
      tag_name: tracker.last_execution.tag_name || tracker.last_release_tag,
      release_name: tracker.last_execution.release_name || tracker.name,
    }))
    .sort((left, right) => new Date(right.triggered_at) - new Date(left.triggered_at))
)

onMounted(loadAll)

async function loadAll() {
  await Promise.all([loadWebhooks(), loadTrackers(), loadHosts()])
}

async function loadWebhooks() {
  loadingWebhooks.value = true
  try {
    error.value = ''
    const response = await api.getGitWebhooks()
    webhooks.value = response.data.webhooks || []
  } catch (err) {
    error.value = err.response?.data?.error || 'Erreur lors du chargement des webhooks'
  } finally {
    loadingWebhooks.value = false
  }
}

async function loadTrackers() {
  loadingTrackers.value = true
  try {
    error.value = ''
    const response = await api.getReleaseTrackers()
    trackers.value = response.data.trackers || []
  } catch (err) {
    error.value = err.response?.data?.error || 'Erreur lors du chargement des trackers'
  } finally {
    loadingTrackers.value = false
  }
}

async function loadHosts() {
  try {
    const response = await api.getHosts()
    hosts.value = response.data || []
  } catch {
    hosts.value = []
  }
}

function openCreateWebhook() {
  editingWebhook.value = null
  modalError.value = ''
  showWebhookModal.value = true
}

function openEditWebhook(webhook) {
  editingWebhook.value = webhook
  modalError.value = ''
  showWebhookModal.value = true
}

function closeWebhookModal() {
  showWebhookModal.value = false
  editingWebhook.value = null
  modalError.value = ''
}

async function saveWebhook(payload) {
  saving.value = true
  modalError.value = ''
  try {
    if (editingWebhook.value) {
      await api.updateGitWebhook(editingWebhook.value.id, payload)
    } else {
      const response = await api.createGitWebhook(payload)
      const created = response.data.webhook
      if (created?.secret) {
        newWebhookId.value = created.id
        newWebhookSecret.value = created.secret
      }
    }
    closeWebhookModal()
    await loadWebhooks()
  } catch (err) {
    modalError.value = err.response?.data?.error || 'Erreur'
  } finally {
    saving.value = false
  }
}

async function toggleWebhook(webhook) {
  try {
    await api.updateGitWebhook(webhook.id, { ...webhook, enabled: !webhook.enabled })
    await loadWebhooks()
  } catch (err) {
    error.value = err.response?.data?.error || 'Erreur'
  }
}

async function confirmDeleteWebhook(webhook) {
  const ok = await dialog.confirm({
    title: `Supprimer le webhook "${webhook.name}" ?`,
    message: 'Toutes les executions associees seront egalement supprimees.',
    variant: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteGitWebhook(webhook.id)
    await loadWebhooks()
  } catch (err) {
    error.value = err.response?.data?.error || 'Erreur lors de la suppression'
  }
}

function closeSecretModal() {
  newWebhookSecret.value = ''
  newWebhookId.value = ''
}

function openCreateTracker() {
  editingTracker.value = null
  modalError.value = ''
  showTrackerModal.value = true
}

function openEditTracker(tracker) {
  editingTracker.value = tracker
  modalError.value = ''
  showTrackerModal.value = true
}

function closeTrackerModal() {
  showTrackerModal.value = false
  editingTracker.value = null
  modalError.value = ''
}

async function saveTracker(payload) {
  saving.value = true
  modalError.value = ''
  try {
    if (editingTracker.value) {
      await api.updateReleaseTracker(editingTracker.value.id, payload)
    } else {
      await api.createReleaseTracker(payload)
    }
    closeTrackerModal()
    await loadTrackers()
  } catch (err) {
    modalError.value = err.response?.data?.error || 'Erreur'
  } finally {
    saving.value = false
  }
}

async function toggleTracker(tracker) {
  try {
    await api.updateReleaseTracker(tracker.id, { ...tracker, enabled: !tracker.enabled })
    await loadTrackers()
  } catch (err) {
    error.value = err.response?.data?.error || 'Erreur'
  }
}

async function checkNow(tracker) {
  try {
    await api.checkReleaseTrackerNow(tracker.id)
    setTimeout(() => loadTrackers(), 2000)
  } catch (err) {
    error.value = err.response?.data?.error || 'Erreur'
  }
}

async function confirmDeleteTracker(tracker) {
  const ok = await dialog.confirm({
    title: `Supprimer le tracker "${tracker.name}" ?`,
    message: 'Toutes les executions associees seront egalement supprimees.',
    variant: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteReleaseTracker(tracker.id)
    await loadTrackers()
  } catch (err) {
    error.value = err.response?.data?.error || 'Erreur lors de la suppression'
  }
}

function repoURL(tracker) {
  switch (tracker.provider) {
    case 'gitlab':
      return `https://gitlab.com/${tracker.repo_owner}/${tracker.repo_name}`
    case 'gitea':
      return `https://codeberg.org/${tracker.repo_owner}/${tracker.repo_name}`
    default:
      return `https://github.com/${tracker.repo_owner}/${tracker.repo_name}`
  }
}

function providerBadge(provider) {
  const map = {
    github: 'bg-blue-lt text-blue',
    gitlab: 'bg-orange-lt text-orange',
    gitea: 'bg-teal-lt text-teal',
    forgejo: 'bg-purple-lt text-purple',
    custom: 'bg-secondary-lt text-secondary',
  }
  return map[provider] || 'bg-secondary-lt text-secondary'
}

function execStatusBadge(status) {
  const map = {
    pending: 'bg-yellow-lt text-yellow',
    running: 'bg-blue-lt text-blue',
    completed: 'bg-success-lt text-success',
    failed: 'bg-danger-lt text-danger',
    skipped: 'bg-secondary-lt text-secondary',
  }
  return map[status] || 'bg-secondary-lt text-secondary'
}

function formatRelative(dateStr) {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString('fr-FR')
}
</script>
