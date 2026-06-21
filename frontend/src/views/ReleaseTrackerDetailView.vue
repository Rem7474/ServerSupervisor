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
      <!-- Latest release info card -->
      <div
        v-if="tracker?.last_release_tag"
        class="col-lg-12"
      >
        <div class="card bg-info-lt border-info">
          <div class="card-body">
            <h4 class="card-title">
              Dernière version détectée
            </h4>
            <dl class="row mb-0 small">
              <dt class="col-sm-3 text-muted">
                Version
              </dt>
              <dd class="col-sm-9">
                <code class="fs-6">{{ tracker.last_release_tag }}</code>
              </dd>
              <template v-if="tracker.docker_image">
                <dt class="col-sm-3 text-muted">
                  Image &amp; tag
                </dt>
                <dd class="col-sm-9">
                  <code>{{ tracker.docker_image }}:{{ tracker.last_release_tag }}</code>
                </dd>
              </template>
              <template v-if="tracker.release_url">
                <dt class="col-sm-3 text-muted">
                  Release
                </dt>
                <dd class="col-sm-9">
                  <a
                    :href="tracker.release_url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="link-primary"
                  >
                    → Voir sur GitHub
                  </a>
                </dd>
              </template>
            </dl>
          </div>
        </div>
      </div>

      <!-- Left column: config -->
      <div class="col-lg-5">
        <TrackerConfigCard
          :tracker="tracker"
          :checking="checking"
          :running="running"
          :can-run-manually="canRunManually"
          :run-disabled-reason="runDisabledReason"
          :cooldown-active="cooldownActive"
          :cooldown-eta-text="cooldownEtaText"
          @check="triggerCheck"
          @run="runManually"
          @edit="openEdit"
        />

        <!-- Alert: No task configured -->
        <div
          v-if="tracker && !tracker.custom_task_id && tracker.host_id"
          class="alert alert-warning mt-3 mb-3"
        >
          <h4 class="alert-title">
            Aucune tâche configurée
          </h4>
          <p class="mb-2">
            Cette image Docker est surveillée, mais aucune tâche de déploiement n'a été configurée.
          </p>
          <p class="mb-2 small text-muted">
            Créer une tâche pour automatiser les mises à jour lorsqu'une nouvelle version est détectée.
          </p>
          <router-link
            :to="`/hosts/${tracker.host_id}`"
            class="btn btn-sm btn-warning"
          >
            Créer une tâche
          </router-link>
        </div>

        <TrackerScriptHelpCard
          :tracker="tracker"
          :compose-projects="composeProjects"
          :tasks-yaml="tasksYaml"
          :loading-snippet="loadingSnippet"
        />
      </div>

      <div class="col-lg-7">
        <TrackerVersionHistoryCard
          :history="versionHistory"
          :loading="historyLoading"
        />

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

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api'
import { formatDateTime } from '../utils/formatters'
import WebhookExecutionList from '../components/webhooks/WebhookExecutionList.vue'
import WebhookModal from '../components/webhooks/WebhookModal.vue'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'
import TrackerConfigCard from '../components/webhooks/TrackerConfigCard.vue'
import TrackerScriptHelpCard from '../components/webhooks/TrackerScriptHelpCard.vue'
import TrackerVersionHistoryCard from '../components/webhooks/TrackerVersionHistoryCard.vue'
import { useCommandStream } from '../composables/useCommandStream'
import { getApiErrorMessage, isApiAbort } from '../api/client'
import { useAbortSignal } from '../composables/useAbortSignal'
import type { ReleaseTracker, ReleaseTrackerExecution, ReleaseTrackerRequest, ReleaseVersionHistoryItem } from '../types/tracker'
import type { ComposeProject } from '../types/docker'
import type { Host } from '../types/host'
import type { WebhookFormData } from '../composables/useWebhookForm'

interface CmdRow { id: string; status?: string; output?: string; [key: string]: unknown }
// The API enriches the tracker with the resolved release URL (not in the Go model).
type TrackerView = ReleaseTracker & { release_url?: string }

const route = useRoute()
const id = route.params.id as string
const signal = useAbortSignal()

const tracker = ref<TrackerView | null>(null)
const executions = ref<ReleaseTrackerExecution[]>([])
const versionHistory = ref<ReleaseVersionHistoryItem[]>([])
const hosts = ref<Host[]>([])
const loading = ref(false)
const error = ref('')
const historyLoading = ref(false)
const checking = ref(false)
const running = ref(false)
const selectedCmd = ref<CmdRow | null>(null)
const showConsole = ref(false)
const nowTick = ref(Date.now())
let cooldownTimer: number | null = null

const composeProjects = ref<ComposeProject[]>([])
const tasksYaml = ref('')
const loadingSnippet = ref(false)

const showModal = ref(false)
const saving = ref(false)
const modalError = ref('')

const { openCommandStream, closeStream } = useCommandStream()

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

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const [res, hostsRes] = await Promise.all([api.getReleaseTracker(id, signal), api.getHosts(signal)])
    tracker.value = res.data.tracker
    executions.value = res.data.executions || []
    hosts.value = hostsRes.data || []
  } catch (e: unknown) {
    if (isApiAbort(e)) return
    error.value = getApiErrorMessage(e, 'Erreur lors du chargement')
  } finally {
    loading.value = false
  }

  await loadVersionHistory()

  if (tracker.value?.host_id) {
    await loadSnippetData(tracker.value.host_id)
  }
}

async function loadSnippetData(hostId: string): Promise<void> {
  loadingSnippet.value = true
  try {
    const [composeRes, yamlRes] = await Promise.all([
      api.getHostComposeProjects(hostId).catch(() => ({ data: [] })),
      api.getHostTasksYaml(hostId).catch(() => ({ data: { yaml: '' } })),
    ])
    composeProjects.value = composeRes.data || []
    tasksYaml.value = yamlRes.data?.yaml || ''
  } finally {
    loadingSnippet.value = false
  }
}

async function loadVersionHistory(): Promise<void> {
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

async function loadExecutions(): Promise<void> {
  try {
    const res = await api.getReleaseTrackerExecutions(id)
    executions.value = res.data.executions || []
  } catch { /* ignore */ }
}

function clearExecutionLogs(): void {
  closeStream()
  selectedCmd.value = null
  showConsole.value = false
}

function connectExecutionStream(commandId: string): void {
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

      const idx = executions.value.findIndex((e: ReleaseTrackerExecution) => e.command_id === commandId)
      if (idx !== -1) {
        const next = [...executions.value]
        next[idx] = { ...next[idx], status: payload.status || next[idx].status }
        executions.value = next
      }
    },
  })
}

async function openExecutionLogs(commandId: string): Promise<void> {
  closeStream()
  try {
    const res = await api.getCommandStatus(commandId)
    const cmd = res.data
    selectedCmd.value = cmd as unknown as CmdRow
    showConsole.value = true
    if (cmd?.status === 'pending' || cmd?.status === 'running') {
      connectExecutionStream(commandId)
    }
  } catch {
    error.value = 'Impossible de charger les logs de la commande.'
  }
}

async function runManually(): Promise<void> {
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
  } catch (e: unknown) {
    error.value = getApiErrorMessage(e, 'Erreur lors du déclenchement')
    running.value = false
  }
}

async function triggerCheck(): Promise<void> {
  checking.value = true
  try {
    await api.checkReleaseTrackerNow(id)
    setTimeout(async () => {
      await load()
      checking.value = false
    }, 2000)
  } catch (e: unknown) {
    error.value = getApiErrorMessage(e, 'Erreur')
    checking.value = false
  }
}

function openEdit(): void {
  modalError.value = ''
  showModal.value = true
}

async function saveEdit(payload: WebhookFormData): Promise<void> {
  saving.value = true
  modalError.value = ''
  try {
    await api.updateReleaseTracker(id, payload as unknown as ReleaseTrackerRequest)
    closeEdit()
    await load()
  } catch (e: unknown) {
    modalError.value = getApiErrorMessage(e, 'Erreur')
  } finally {
    saving.value = false
  }
}

function closeEdit(): void {
  showModal.value = false
  modalError.value = ''
}

function providerBadge(provider: string): string {
  const map: Record<string, string> = {
    github: 'bg-blue-lt text-blue',
    gitlab: 'bg-orange-lt text-orange',
    gitea: 'bg-teal-lt text-teal',
    forgejo: 'bg-purple-lt text-purple',
    custom: 'bg-secondary-lt text-secondary',
  }
  return map[provider] || 'bg-secondary-lt text-secondary'
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
