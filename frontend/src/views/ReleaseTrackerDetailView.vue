<template>
  <div>
    <div class="page-header mb-3">
      <div>
        <div class="page-pretitle">
          <router-link
            to="/git-webhooks?tab=trackers"
            class="text-decoration-none"
          >
            Suivi de versions
          </router-link>
          <span class="text-muted mx-1">/</span>
          <span>{{ tracker?.name || id }}</span>
        </div>
        <h2 class="page-title d-flex align-items-center gap-2">
          {{ tracker?.name }}
          <template v-if="tracker">
            <span
              v-if="tracker.tracker_type === 'docker'"
              class="badge bg-cyan-lt text-cyan"
            >docker</span>
            <span
              v-else
              class="badge"
              :class="providerBadge(tracker.provider)"
            >{{ tracker.provider }}</span>
          </template>
          <span
            v-if="tracker && !tracker.enabled"
            class="badge bg-secondary"
          >Désactivé</span>
          <span
            v-if="tracker && cooldownActive"
            class="badge bg-yellow-lt text-yellow"
            :title="`Déploiement prévu: ${cooldownEtaText}`"
          >Cooldown actif · reste {{ cooldownRemainingText }}</span>
        </h2>
      </div>
    </div>

    <div
      v-if="error"
      class="alert alert-danger mb-3"
    >
      {{ error }}
    </div>

    <div
      v-if="loading"
      class="text-center py-5"
    >
      <div
        class="spinner-border text-primary"
        role="status"
      />
    </div>

    <div
      v-else-if="tracker"
      class="row g-3"
    >
      <!-- Left column: config -->
      <div class="col-lg-5">
        <div class="card">
          <div class="card-header d-flex align-items-center justify-content-between">
            <h3 class="card-title">
              Configuration
            </h3>
            <div class="d-flex gap-2">
              <button
                class="btn btn-sm btn-ghost-secondary"
                :disabled="checking"
                @click="triggerCheck"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="14"
                  height="14"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  viewBox="0 0 24 24"
                  class="me-1"
                >
                  <polyline points="23 4 23 10 17 10" /><path d="M20.49 15a9 9 0 11-2.12-9.36L23 10" />
                </svg>
                {{ checking ? 'Vérification...' : 'Vérifier maintenant' }}
              </button>
              <button
                class="btn btn-sm btn-primary"
                :disabled="running || !canRunManually"
                :title="runDisabledReason"
                @click="runManually"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="14"
                  height="14"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  viewBox="0 0 24 24"
                  class="me-1"
                >
                  <polygon points="5 3 19 12 5 21 5 3" />
                </svg>
                {{ running ? 'Déclenchement...' : 'Exécuter' }}
              </button>
              <button
                class="btn btn-sm btn-ghost-secondary"
                @click="openEdit"
              >
                Modifier
              </button>
            </div>
          </div>
          <div class="card-body">
            <dl class="row mb-0 small">
              <dt class="col-5 text-muted">
                Type
              </dt>
              <dd class="col-7">
                <span
                  v-if="tracker.tracker_type === 'docker'"
                  class="badge bg-cyan-lt text-cyan"
                >Image Docker</span>
                <span
                  v-else
                  class="badge bg-blue-lt text-blue"
                >Release Git</span>
              </dd>

              <!-- Git-specific -->
              <template v-if="tracker.tracker_type !== 'docker'">
                <dt class="col-5 text-muted">
                  Provider
                </dt>
                <dd class="col-7">
                  {{ tracker.provider }}
                </dd>
                <dt class="col-5 text-muted">
                  Dépôt
                </dt>
                <dd class="col-7">
                  <a
                    :href="repoURL"
                    target="_blank"
                    class="link-primary"
                  >
                    {{ tracker.repo_owner }}/{{ tracker.repo_name }}
                  </a>
                </dd>
                <dt class="col-5 text-muted">
                  Dernière release
                </dt>
                <dd class="col-7">
                  <span
                    v-if="tracker.last_release_tag"
                    class="badge bg-green-lt text-green"
                  >{{ tracker.last_release_tag }}</span>
                  <span
                    v-else
                    class="text-muted"
                  >En attente...</span>
                </dd>
              </template>

              <!-- Docker-specific -->
              <template v-else>
                <dt class="col-5 text-muted">
                  Image
                </dt>
                <dd class="col-7">
                  <code>{{ tracker.docker_image }}</code>
                </dd>
                <dt class="col-5 text-muted">
                  Tag surveillé
                </dt>
                <dd class="col-7">
                  <code>{{ tracker.docker_tag || 'latest' }}</code>
                </dd>
                <template v-if="tracker.latest_image_digest">
                  <dt class="col-5 text-muted">
                    Dernier digest
                  </dt>
                  <dd class="col-7">
                    <code
                      class="small text-muted"
                      :title="tracker.latest_image_digest"
                    >
                      {{ tracker.latest_image_digest.slice(0, 19) }}…
                    </code>
                  </dd>
                </template>
                <dt class="col-5 text-muted">
                  Dernier check
                </dt>
                <dd class="col-7">
                  <span v-if="tracker.last_checked_at"><RelativeTime :date="tracker.last_checked_at" /></span>
                  <span
                    v-else
                    class="text-muted"
                  >Jamais</span>
                </dd>
              </template>

              <!-- Common fields -->
              <template v-if="tracker.host_id && tracker.custom_task_id">
                <dt class="col-5 text-muted">
                  VM cible
                </dt>
                <dd class="col-7">
                  {{ tracker.host_name || tracker.host_id }}
                </dd>
                <dt class="col-5 text-muted">
                  Tâche
                </dt>
                <dd class="col-7">
                  <code>{{ tracker.custom_task_id }}</code>
                </dd>
              </template>
              <template v-else-if="!tracker.host_id || !tracker.custom_task_id">
                <dt class="col-5 text-muted">
                  Mode
                </dt>
                <dd class="col-7">
                  <span class="badge bg-blue-lt text-blue">Surveillance seule</span>
                </dd>
              </template>
              <dt
                v-if="tracker.tracker_type !== 'docker' && tracker.last_checked_at"
                class="col-5 text-muted"
              >
                Dernier check
              </dt>
              <dd
                v-if="tracker.tracker_type !== 'docker' && tracker.last_checked_at"
                class="col-7"
              >
                <RelativeTime :date="tracker.last_checked_at" />
              </dd>
              <template v-if="tracker.last_error">
                <dt class="col-5 text-muted">
                  Erreur
                </dt>
                <dd class="col-7 text-danger small">
                  {{ tracker.last_error }}
                </dd>
              </template>
              <dt
                v-if="tracker.last_triggered_at"
                class="col-5 text-muted"
              >
                Dernier déclench.
              </dt>
              <dd
                v-if="tracker.last_triggered_at"
                class="col-7"
              >
                <RelativeTime :date="tracker.last_triggered_at" />
              </dd>
              <dt
                v-if="tracker.notify_channels?.length"
                class="col-5 text-muted"
              >
                Notifications
              </dt>
              <dd
                v-if="tracker.notify_channels?.length"
                class="col-7"
              >
                <span
                  v-for="ch in tracker.notify_channels"
                  :key="ch"
                  class="badge me-1"
                  :class="channelBadge(ch)"
                >{{ ch }}</span>
              </dd>
              <dt class="col-5 text-muted">
                Créé le
              </dt>
              <dd class="col-7">
                {{ formatDateTime(tracker.created_at) }}
              </dd>
              <dt class="col-5 text-muted">
                Cooldown
              </dt>
              <dd class="col-7">
                {{ tracker.cooldown_hours ? `${tracker.cooldown_hours}h` : 'Aucun (immédiat)' }}
              </dd>
              <template v-if="cooldownActive">
                <dt class="col-5 text-muted">
                  Déploiement prévu
                </dt>
                <dd class="col-7">
                  {{ cooldownEtaText }}
                </dd>
              </template>
            </dl>
          </div>
        </div>

        <!-- Env vars card -->
        <div class="card mt-3">
          <div class="card-header">
            <h3 class="card-title">
              Variables disponibles dans le script
            </h3>
          </div>
          <div class="card-body p-0">
            <div class="table-responsive">
              <table class="table table-sm table-vcenter mb-0">
                <tbody>
                  <tr
                    v-for="v in envVars"
                    :key="v.name"
                  >
                    <td><code class="small">{{ v.name }}</code></td>
                    <td class="text-muted small">
                      {{ v.desc }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>

      <div class="col-lg-7">
        <div class="card mb-3">
          <div class="card-header d-flex align-items-center justify-content-between">
            <h3 class="card-title mb-0">
              Historique des versions
            </h3>
            <small class="text-muted">Publication / détection</small>
          </div>
          <div class="card-body p-0">
            <div
              v-if="historyLoading"
              class="p-3 text-center text-muted"
            >
              Chargement...
            </div>
            <div
              v-else-if="!versionHistory.length"
              class="p-3 text-muted"
            >
              Aucune version disponible.
            </div>
            <div
              v-else
              class="table-responsive"
            >
              <table class="table table-sm table-vcenter mb-0">
                <thead>
                  <tr>
                    <th>Version</th>
                    <th>Détails</th>
                    <th class="text-end">
                      Date de publication
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="entry in versionHistory"
                    :key="`${entry.version}-${entry.published_at || 'n/a'}`"
                  >
                    <td>
                      <span class="badge bg-green-lt text-green">{{ entry.version }}</span>
                    </td>
                    <td>
                      <a
                        v-if="entry.release_url"
                        :href="entry.release_url"
                        target="_blank"
                        class="link-primary"
                      >
                        {{ entry.name || entry.release_url }}
                      </a>
                      <span v-else>{{ entry.name || '-' }}</span>
                    </td>
                    <td class="text-end text-muted">
                      {{ entry.published_at ? formatDateTime(entry.published_at) : 'N/A' }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <WebhookExecutionList
          :executions="executions"
          kind="tracker"
          title="Historique des exécutions"
          empty-text="Aucune exécution enregistrée."
          :show-refresh="true"
          logs-mode="inline"
          @refresh="loadExecutions"
          @open-logs="openExecutionLogs"
        />

        <div class="mt-3">
          <CommandLogPanel
            :command="selectedCmd"
            :show="showConsole"
            title="Console live"
            empty-text="Sélectionnez 'Logs' dans l'historique des exécutions"
            @close="clearExecutionLogs"
            @open="showConsole = true"
          />
        </div>
      </div>
    </div>

    <WebhookModal
      :visible="showModal"
      mode="tracker"
      :item="tracker"
      :hosts="hosts"
      :saving="saving"
      :error="modalError"
      @close="closeEdit"
      @submit="saveEdit"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api'
import { formatDateTime } from '../utils/formatters'
import RelativeTime from '../components/RelativeTime.vue'
import WebhookExecutionList from '../components/webhooks/WebhookExecutionList.vue'
import WebhookModal from '../components/webhooks/WebhookModal.vue'
import CommandLogPanel from '../components/CommandLogPanel.vue'
import { useCommandStream } from '../composables/useCommandStream'

const route = useRoute()
const auth = useAuthStore()
const id = route.params.id

const tracker = ref(null)
const executions = ref([])
const versionHistory = ref([])
const hosts = ref([])
const loading = ref(false)
const error = ref('')
const historyLoading = ref(false)
const checking = ref(false)
const running = ref(false)
const selectedCmd = ref(null)
const showConsole = ref(false)
const nowTick = ref(Date.now())
let cooldownTimer = null

const showModal = ref(false)
const saving = ref(false)
const modalError = ref('')

const { openCommandStream, closeStream } = useCommandStream({ token: () => auth.token })

const gitEnvVars = [
  { name: 'SS_REPO_NAME',    desc: 'owner/repo (ex: home-assistant/core)' },
  { name: 'SS_TAG_NAME',     desc: 'Tag de la nouvelle release (ex: v1.2.3)' },
  { name: 'SS_RELEASE_URL',  desc: 'URL de la release sur le provider' },
  { name: 'SS_RELEASE_NAME', desc: 'Titre de la release' },
  { name: 'SS_TRACKER_NAME', desc: 'Nom du tracker dans ServerSupervisor' },
]

const dockerEnvVars = [
  { name: 'SS_IMAGE_NAME',   desc: 'image:tag surveille (ex: nginx:latest)' },
  { name: 'SS_IMAGE_TAG',    desc: 'Tag surveille (ex: latest)' },
  { name: 'SS_OLD_DIGEST',   desc: 'Digest manifest SHA256 precedent' },
  { name: 'SS_NEW_DIGEST',   desc: 'Nouveau digest manifest SHA256' },
  { name: 'SS_TRACKER_NAME', desc: 'Nom du tracker dans ServerSupervisor' },
]

const envVars = computed(() =>
  tracker.value?.tracker_type === 'docker' ? dockerEnvVars : gitEnvVars
)

const canRunManually = computed(() => {
  if (!tracker.value) return false
  // Any tracker can be in monitor-only mode (no host/task dispatch configured).
  if (!tracker.value.host_id || !tracker.value.custom_task_id) {
    return false
  }
  return true
})

const runDisabledReason = computed(() => {
  if (!tracker.value) return ''
  if (!tracker.value.host_id || !tracker.value.custom_task_id) {
    return 'Mode surveillance seule: configurez une VM cible et une tâche pour autoriser l\'exécution manuelle.'
  }
  return ''
})

const cooldownRemainingMs = computed(() => {
  const t = tracker.value
  if (!t) return 0
  const hours = Number(t.cooldown_hours || 0)
  if (!hours || hours <= 0 || !t.last_release_detected_at) return 0

  const detectedAt = new Date(t.last_release_detected_at).getTime()
  if (!Number.isFinite(detectedAt)) return 0

  if (t.last_triggered_at) {
    const triggeredAt = new Date(t.last_triggered_at).getTime()
    if (Number.isFinite(triggeredAt) && triggeredAt >= detectedAt) return 0
  }

  const endsAt = detectedAt + (hours * 60 * 60 * 1000)
  return Math.max(0, endsAt - nowTick.value)
})

const cooldownActive = computed(() => cooldownRemainingMs.value > 0)

const cooldownRemainingText = computed(() => {
  const ms = cooldownRemainingMs.value
  if (ms <= 0) return '0m'
  const totalMinutes = Math.ceil(ms / 60000)
  const days = Math.floor(totalMinutes / (24 * 60))
  const hours = Math.floor((totalMinutes % (24 * 60)) / 60)
  const minutes = totalMinutes % 60
  if (days > 0) return `${days}j ${hours}h`
  if (hours > 0) return `${hours}h ${minutes}m`
  return `${minutes}m`
})

const cooldownEtaText = computed(() => {
  const t = tracker.value
  if (!t) return '-'
  const hours = Number(t.cooldown_hours || 0)
  if (!hours || hours <= 0 || !t.last_release_detected_at) return '-'

  const detectedAt = new Date(t.last_release_detected_at).getTime()
  if (!Number.isFinite(detectedAt)) return '-'

  return formatDateTime(new Date(detectedAt + (hours * 60 * 60 * 1000)).toISOString())
})

const repoURL = computed(() => {
  if (!tracker.value || tracker.value.tracker_type === 'docker') return '#'
  switch (tracker.value.provider) {
    case 'gitlab': return `https://gitlab.com/${tracker.value.repo_owner}/${tracker.value.repo_name}`
    case 'gitea':  return `https://codeberg.org/${tracker.value.repo_owner}/${tracker.value.repo_name}`
    default:       return `https://github.com/${tracker.value.repo_owner}/${tracker.value.repo_name}`
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

  await loadVersionHistory()
}

async function loadVersionHistory() {
  historyLoading.value = true
  try {
    const res = await api.getReleaseTrackerVersionHistory(id)
    versionHistory.value = res.data.history || []
  } catch {
    versionHistory.value = []
  } finally {
    historyLoading.value = false
  }
}

async function loadExecutions() {
  try {
    const res = await api.getReleaseTrackerExecutions(id)
    executions.value = res.data.executions || []
  } catch { /* ignore */ }
}

function clearExecutionLogs() {
  closeStream()
  selectedCmd.value = null
  showConsole.value = false
}

function connectExecutionStream(commandId) {
  openCommandStream(commandId, {
    onInit(payload) {
      if (!selectedCmd.value || selectedCmd.value.id !== commandId) return
      selectedCmd.value = {
        ...selectedCmd.value,
        status: payload.status || selectedCmd.value.status,
        output: payload.output ?? selectedCmd.value.output,
      }
    },
    onChunk(payload) {
      if (!selectedCmd.value || selectedCmd.value.id !== commandId) return
      selectedCmd.value = {
        ...selectedCmd.value,
        output: (selectedCmd.value.output || '') + (payload.chunk || ''),
      }
    },
    onStatus(payload) {
      if (!selectedCmd.value || selectedCmd.value.id !== commandId) return
      selectedCmd.value = {
        ...selectedCmd.value,
        status: payload.status || selectedCmd.value.status,
        output: payload.output ?? selectedCmd.value.output,
      }

      const idx = executions.value.findIndex((e) => e.command_id === commandId)
      if (idx !== -1) {
        const next = [...executions.value]
        next[idx] = { ...next[idx], status: payload.status || next[idx].status }
        executions.value = next
      }
    },
  })
}

async function openExecutionLogs(commandId) {
  closeStream()
  try {
    const res = await api.getCommandStatus(commandId)
    const cmd = res.data
    selectedCmd.value = cmd
    showConsole.value = true
    if (cmd?.status === 'pending' || cmd?.status === 'running') {
      connectExecutionStream(commandId)
    }
  } catch {
    error.value = 'Impossible de charger les logs de la commande.'
  }
}

async function runManually() {
  if (!canRunManually.value) {
    error.value = runDisabledReason.value || 'Exécution manuelle non disponible dans ce mode.'
    return
  }
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
  modalError.value = ''
  showModal.value = true
}

async function saveEdit(payload) {
  saving.value = true
  modalError.value = ''
  try {
    await api.updateReleaseTracker(id, payload)
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

onMounted(() => {
  load()
  cooldownTimer = window.setInterval(() => {
    nowTick.value = Date.now()
  }, 60000)
})
onUnmounted(() => {
  if (cooldownTimer !== null) {
    window.clearInterval(cooldownTimer)
    cooldownTimer = null
  }
  closeStream()
})
</script>

