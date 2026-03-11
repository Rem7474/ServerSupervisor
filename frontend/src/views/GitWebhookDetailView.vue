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

      <div class="col-lg-7">
        <WebhookExecutionList
          :executions="executions"
          kind="webhook"
          title="Historique des executions"
          empty-text="Aucune execution enregistree."
          :show-refresh="true"
          @refresh="loadExecutions"
        />
      </div>
    </div>

    <WebhookModal
      :visible="showModal"
      mode="webhook"
      :item="webhook"
      :hosts="hosts"
      :saving="saving"
      :error="modalError"
      @close="closeEdit"
      @submit="saveEdit"
    />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api'
import { formatDateTime } from '../utils/formatters'
import RelativeTime from '../components/RelativeTime.vue'
import WebhookUrlCard from '../components/WebhookUrlCard.vue'
import WebhookExecutionList from '../components/webhooks/WebhookExecutionList.vue'
import WebhookModal from '../components/webhooks/WebhookModal.vue'

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
  modalError.value = ''
  showModal.value = true
}

async function saveEdit(payload) {
  saving.value = true
  modalError.value = ''
  try {
    await api.updateGitWebhook(id, payload)
    closeEdit()
    await load()
  } catch (e) {
    modalError.value = e.response?.data?.error || 'Erreur'
  } finally {
    saving.value = false
  }
}

function closeEdit() {
  showModal.value = false
  modalError.value = ''
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
