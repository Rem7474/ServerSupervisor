<template>
  <div>
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <h2 class="page-title">
            Git / Automatisation
          </h2>
          <div class="text-muted">
            Webhooks entrants et suivi de releases pour declencher des scripts sur vos VMs.
          </div>
        </div>
        <button
          class="btn btn-primary"
          @click="activeTab === 'webhooks' ? openCreateWebhook() : openCreateTracker()"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="icon me-1"
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <path d="M12 5v14M5 12h14" />
          </svg>
          {{ activeTab === 'webhooks' ? 'Nouveau webhook' : 'Nouveau tracker' }}
        </button>
      </div>
    </div>

    <ul class="nav nav-tabs mb-3">
      <li class="nav-item">
        <a
          class="nav-link"
          :class="{ active: activeTab === 'webhooks' }"
          href="#"
          @click.prevent="activeTab = 'webhooks'"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="icon me-1"
            width="16"
            height="16"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            viewBox="0 0 24 24"
          >
            <circle
              cx="12"
              cy="5"
              r="3"
            /><circle
              cx="5"
              cy="19"
              r="3"
            /><circle
              cx="19"
              cy="19"
              r="3"
            />
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M12 8v3m0 0l-4.5 5.5M12 11l4.5 5.5"
            />
          </svg>
          Webhooks entrants
          <span
            v-if="webhooks.length"
            class="badge bg-azure-lt text-azure ms-1"
          >{{ webhooks.length }}</span>
        </a>
      </li>
      <li class="nav-item">
        <a
          class="nav-link"
          :class="{ active: activeTab === 'trackers' }"
          href="#"
          @click.prevent="activeTab = 'trackers'"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="icon me-1"
            width="16"
            height="16"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            viewBox="0 0 24 24"
          >
            <path d="M22 12h-4l-3 9L9 3l-3 9H2" />
          </svg>
          Suivi de versions
          <span
            v-if="trackers.length"
            class="badge bg-azure-lt text-azure ms-1"
          >{{ trackers.length }}</span>
        </a>
      </li>
    </ul>

    <div
      v-if="error"
      class="alert alert-danger mb-3"
    >
      {{ error }}
    </div>

    <div v-show="activeTab === 'webhooks'">
      <div
        v-if="loadingWebhooks"
        class="text-center py-5"
      >
        <div
          class="spinner-border text-primary"
          role="status"
        />
      </div>

      <div
        v-else-if="webhooks.length === 0"
        class="card"
      >
        <div class="card-body text-center py-5 text-muted">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="48"
            height="48"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            viewBox="0 0 24 24"
            class="mb-3 d-block mx-auto opacity-50"
          >
            <circle
              cx="12"
              cy="5"
              r="3"
            /><circle
              cx="5"
              cy="19"
              r="3"
            /><circle
              cx="19"
              cy="19"
              r="3"
            />
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M12 8v3m0 0l-4.5 5.5M12 11l4.5 5.5"
            />
          </svg>
          <p class="mb-2">
            Aucun webhook configure.
          </p>
          <p class="text-muted small">
            Recevez des evenements depuis GitHub, GitLab, Gitea ou Forgejo pour declencher des scripts sur vos VMs.
          </p>
          <button
            class="btn btn-sm btn-primary"
            @click="openCreateWebhook"
          >
            Creer le premier webhook
          </button>
        </div>
      </div>

      <template v-else>
        <div class="row row-cards">
          <div
            v-for="webhook in webhooks"
            :key="webhook.id"
            class="col-md-6 col-xl-4"
          >
            <div
              class="card h-100"
              :class="{ 'opacity-50': !webhook.enabled }"
            >
              <div class="card-header">
                <div class="d-flex align-items-center gap-2 flex-grow-1 min-w-0">
                  <span
                    class="badge"
                    :class="providerBadge(webhook.provider)"
                  >{{ webhook.provider }}</span>
                  <span class="fw-medium text-truncate">{{ webhook.name }}</span>
                </div>
                <div class="ms-auto d-flex gap-1">
                  <span
                    v-if="!webhook.enabled"
                    class="badge bg-secondary"
                  >Desactive</span>
                </div>
              </div>
              <div class="card-body">
                <div class="mb-2 small">
                  <div class="d-flex gap-2 mb-1">
                    <span
                      class="text-muted"
                      style="min-width:60px"
                    >Repo</span>
                    <span class="text-truncate">{{ webhook.repo_filter || '<tous>' }}</span>
                  </div>
                  <div class="d-flex gap-2 mb-1">
                    <span
                      class="text-muted"
                      style="min-width:60px"
                    >Branche</span>
                    <span>{{ webhook.branch_filter || '<toutes>' }}</span>
                  </div>
                  <div class="d-flex gap-2 mb-1">
                    <span
                      class="text-muted"
                      style="min-width:60px"
                    >VM</span>
                    <span class="text-truncate">{{ webhook.host_name || webhook.host_id }}</span>
                  </div>
                  <div class="d-flex gap-2 mb-1">
                    <span
                      class="text-muted"
                      style="min-width:60px"
                    >Tache</span>
                    <code class="small text-truncate">{{ webhook.custom_task_id }}</code>
                  </div>
                </div>
                <div
                  v-if="webhook.last_execution"
                  class="mt-2 pt-2 border-top small"
                >
                  <span class="text-muted">Dernière exécution :</span>
                  <span
                    class="ms-1 badge"
                    :class="execStatusBadge(webhook.last_execution.status)"
                  >{{ webhook.last_execution.status }}</span>
                  <span class="ms-1 text-muted">{{ formatRelative(webhook.last_execution.triggered_at) }}</span>
                </div>
                <div
                  v-else
                  class="mt-2 pt-2 border-top small text-muted"
                >
                  Jamais declenche
                </div>
              </div>
              <div class="card-footer d-flex gap-2">
                <router-link
                  :to="`/git-webhooks/${webhook.id}`"
                  class="btn btn-sm btn-outline-primary"
                >
                  Détails
                </router-link>
                <button
                  class="btn btn-sm btn-outline-secondary"
                  @click="openEditWebhook(webhook)"
                >
                  Modifier
                </button>
                <button
                  class="btn btn-sm"
                  :class="webhook.enabled ? 'btn-outline-warning' : 'btn-outline-success'"
                  @click="toggleWebhook(webhook)"
                >
                  {{ webhook.enabled ? 'Desactiver' : 'Activer' }}
                </button>
                <button
                  class="btn btn-sm btn-outline-danger ms-auto"
                  @click="confirmDeleteWebhook(webhook)"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    width="14"
                    height="14"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    viewBox="0 0 24 24"
                  >
                    <polyline points="3,6 5,6 21,6" /><path d="M19 6l-1 14H6L5 6" /><path d="M10 11v6M14 11v6" /><path d="M9 6V4h6v2" />
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
          title="Dernières exécutions des webhooks"
          empty-text="Aucune exécution connue."
        />
      </template>
    </div>

    <div v-show="activeTab === 'trackers'">
      <div
        v-if="loadingTrackers"
        class="text-center py-5"
      >
        <div
          class="spinner-border text-primary"
          role="status"
        />
      </div>

      <div
        v-else-if="trackers.length === 0"
        class="card"
      >
        <div class="card-body text-center py-5 text-muted">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="48"
            height="48"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            viewBox="0 0 24 24"
            class="mb-3 d-block mx-auto opacity-50"
          >
            <path d="M22 12h-4l-3 9L9 3l-3 9H2" />
          </svg>
          <p class="mb-2">
            Aucun tracker configure.
          </p>
          <p class="text-muted small">
            Surveillez les releases Git ou les images Docker et declenchez automatiquement un script sur une VM lors d'une mise a jour.
          </p>
          <button
            class="btn btn-sm btn-primary"
            @click="openCreateTracker"
          >
            Creer le premier tracker
          </button>
        </div>
      </div>

      <template v-else>
        <div class="row row-cards">
          <div
            v-for="tracker in trackers"
            :key="tracker.id"
            class="col-md-6 col-xl-4"
          >
            <div
              class="card h-100"
              :class="{ 'opacity-50': !tracker.enabled }"
            >
              <div class="card-header">
                <div class="d-flex align-items-center gap-2 flex-grow-1 min-w-0">
                  <!-- Type badge -->
                  <span
                    v-if="tracker.tracker_type === 'docker'"
                    class="badge bg-cyan-lt text-cyan"
                  >docker</span>
                  <span
                    v-else
                    class="badge"
                    :class="providerBadge(tracker.provider)"
                  >{{ tracker.provider }}</span>
                  <span class="fw-medium text-truncate">{{ tracker.name }}</span>
                </div>
                <div class="ms-auto">
                  <span
                    v-if="!tracker.enabled"
                    class="badge bg-secondary"
                  >Desactive</span>
                </div>
              </div>
              <div class="card-body">
                <div class="mb-2 small">
                  <!-- Docker tracker info -->
                  <template v-if="tracker.tracker_type === 'docker'">
                    <div class="d-flex gap-2 mb-1">
                      <span
                        class="text-muted"
                        style="min-width:60px"
                      >Image</span>
                      <code class="text-truncate">{{ tracker.docker_image }}:{{ tracker.docker_tag || 'latest' }}</code>
                    </div>
                    <div
                      v-if="tracker.repo_owner && tracker.repo_name"
                      class="d-flex gap-2 mb-1"
                    >
                      <span
                        class="text-muted"
                        style="min-width:60px"
                      >Repo</span>
                      <a
                        :href="repoURL(tracker)"
                        target="_blank"
                        class="link-primary text-truncate"
                      >{{ tracker.repo_owner }}/{{ tracker.repo_name }}</a>
                    </div>
                  </template>
                  <!-- Git tracker info -->
                  <template v-else>
                    <div class="d-flex gap-2 mb-1">
                      <span
                        class="text-muted"
                        style="min-width:60px"
                      >Repo</span>
                      <a
                        :href="repoURL(tracker)"
                        target="_blank"
                        class="link-primary text-truncate"
                      >{{ tracker.repo_owner }}/{{ tracker.repo_name }}</a>
                    </div>
                  </template>
                  <template v-if="tracker.host_id && tracker.custom_task_id">
                    <div class="d-flex gap-2 mb-1">
                      <span
                        class="text-muted"
                        style="min-width:60px"
                      >VM</span>
                      <span class="text-truncate">{{ tracker.host_name || tracker.host_id }}</span>
                    </div>
                    <div class="d-flex gap-2 mb-1">
                      <span
                        class="text-muted"
                        style="min-width:60px"
                      >Tache</span>
                      <code class="small text-truncate">{{ tracker.custom_task_id }}</code>
                    </div>
                  </template>
                  <template v-else-if="!tracker.host_id || !tracker.custom_task_id">
                    <div class="d-flex gap-2 mb-1">
                      <span
                        class="text-muted"
                        style="min-width:60px"
                      >Mode</span>
                      <span class="badge bg-blue-lt text-blue">Surveillance seule</span>
                    </div>
                  </template>
                  <div
                    v-if="tracker.last_release_tag"
                    class="d-flex gap-2 mb-1"
                  >
                    <span
                      class="text-muted"
                      style="min-width:60px"
                    >Derniere</span>
                    <span class="badge bg-green-lt text-green">{{ tracker.last_release_tag }}</span>
                  </div>
                  <div class="d-flex gap-2 mb-1">
                    <span
                      class="text-muted"
                      style="min-width:60px"
                    >Maj</span>
                    <span>{{ formatDateOnly(tracker.last_checked_at || tracker.last_triggered_at || tracker.last_execution?.triggered_at) }}</span>
                  </div>
                  <div class="d-flex gap-2 mb-1">
                    <span
                      class="text-muted"
                      style="min-width:60px"
                    >Cooldown</span>
                    <span>{{ tracker.cooldown_hours ? `${tracker.cooldown_hours}h` : 'Aucun' }}</span>
                  </div>
                </div>
                <div class="mt-2 pt-2 border-top small">
                  <div
                    v-if="isCooldownActive(tracker)"
                    class="mb-2"
                  >
                    <span
                      class="badge bg-yellow-lt text-yellow"
                      :title="`Déploiement prévu: ${cooldownEtaLabel(tracker)}`"
                    >Cooldown actif · reste {{ cooldownRemainingLabel(tracker) }}</span>
                  </div>
                  <template v-if="tracker.last_execution">
                    <span class="text-muted">Dernière exécution :</span>
                    <span
                      class="ms-1 badge"
                      :class="execStatusBadge(tracker.last_execution.status)"
                    >{{ tracker.last_execution.status }}</span>
                    <span class="ms-1 text-muted">{{ formatRelative(tracker.last_execution.triggered_at) }}</span>
                  </template>
                  <template v-else-if="tracker.last_checked_at">
                    <span class="text-muted">Derniere verif : {{ formatRelative(tracker.last_checked_at) }}</span>
                    <span
                      v-if="tracker.last_error"
                      class="ms-1 badge bg-danger-lt text-danger"
                      :title="tracker.last_error"
                    >erreur</span>
                    <span
                      v-else-if="!tracker.last_release_tag && tracker.tracker_type !== 'docker'"
                      class="ms-1 badge bg-warning-lt text-warning"
                    >aucune release trouvee</span>
                  </template>
                  <template v-else>
                    <span class="text-muted">En attente du premier check...</span>
                  </template>
                </div>
              </div>
              <div class="card-footer d-flex gap-2">
                <router-link
                  :to="`/release-trackers/${tracker.id}`"
                  class="btn btn-sm btn-outline-primary"
                >
                  Détails
                </router-link>
                <button
                  class="btn btn-sm btn-outline-secondary"
                  @click="openEditTracker(tracker)"
                >
                  Modifier
                </button>
                <button
                  class="btn btn-sm btn-outline-info"
                  title="Verifier maintenant"
                  @click="checkNow(tracker)"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    width="14"
                    height="14"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    viewBox="0 0 24 24"
                  >
                    <polyline points="23 4 23 10 17 10" /><path d="M20.49 15a9 9 0 11-2.12-9.36L23 10" />
                  </svg>
                </button>
                <button
                  class="btn btn-sm"
                  :class="tracker.enabled ? 'btn-outline-warning' : 'btn-outline-success'"
                  @click="toggleTracker(tracker)"
                >
                  {{ tracker.enabled ? 'Desactiver' : 'Activer' }}
                </button>
                <button
                  class="btn btn-sm btn-outline-danger ms-auto"
                  @click="confirmDeleteTracker(tracker)"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    width="14"
                    height="14"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    viewBox="0 0 24 24"
                  >
                    <polyline points="3,6 5,6 21,6" /><path d="M19 6l-1 14H6L5 6" /><path d="M10 11v6M14 11v6" /><path d="M9 6V4h6v2" />
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
          title="Dernières exécutions des trackers"
          empty-text="Aucune exécution connue."
          logs-mode="inline"
          @open-logs="openTrackerLogs"
        />

        <div class="mt-3">
          <CommandLogPanel
            :command="selectedTrackerCmd"
            :show="showTrackerConsole"
            title="Console live"
            empty-text="Sélectionnez 'Logs' dans les dernières exécutions"
            @close="closeTrackerLogs"
            @open="showTrackerConsole = true"
          />
        </div>
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
      :prefill-docker-image="prefillDockerImage"
      :prefill-docker-tag="prefillDockerTag"
      @close="closeTrackerModal"
      @submit="saveTracker"
    />

    <div
      v-if="newWebhookSecret"
      class="modal modal-blur show d-block"
      style="background:rgba(0,0,0,.7)"
    >
      <div class="modal-dialog modal-md">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">
              Webhook cree
            </h5>
          </div>
          <div class="modal-body">
            <div class="alert alert-warning">
              Copiez ce secret maintenant, il ne sera plus affiche en clair.
            </div>
            <WebhookUrlCard
              :webhook-id="newWebhookId"
              :secret="newWebhookSecret"
              :initial-secret="true"
            />
          </div>
          <div class="modal-footer">
            <button
              class="btn btn-primary"
              @click="closeSecretModal"
            >
              J'ai copie le secret
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { useGitWebhooksPage } from '../composables/useGitWebhooksPage'
import WebhookUrlCard from '../components/WebhookUrlCard.vue'
import WebhookExecutionList from '../components/webhooks/WebhookExecutionList.vue'
import WebhookModal from '../components/webhooks/WebhookModal.vue'
import CommandLogPanel from '../components/CommandLogPanel.vue'
import { ref } from 'vue'
import api from '../api'
import { useAuthStore } from '../stores/auth'
import { useCommandStream } from '../composables/useCommandStream'
const {
  activeTab,
  hosts,
  error,
  saving,
  modalError,
  webhooks,
  loadingWebhooks,
  showWebhookModal,
  editingWebhook,
  newWebhookSecret,
  newWebhookId,
  trackers,
  loadingTrackers,
  showTrackerModal,
  editingTracker,
  prefillDockerImage,
  prefillDockerTag,
  recentWebhookExecutions,
  recentTrackerExecutions,
  openCreateWebhook,
  openEditWebhook,
  closeWebhookModal,
  saveWebhook,
  toggleWebhook,
  confirmDeleteWebhook,
  closeSecretModal,
  openCreateTracker,
  openEditTracker,
  closeTrackerModal,
  saveTracker,
  toggleTracker,
  checkNow,
  confirmDeleteTracker,
  repoURL,
  providerBadge,
  execStatusBadge,
  formatRelative,
  formatDateOnly,
  isCooldownActive,
  cooldownRemainingLabel,
  cooldownEtaLabel,
} = useGitWebhooksPage()

const auth = useAuthStore()
const { openCommandStream, closeStream } = useCommandStream({ token: () => auth.token })
const selectedTrackerCmd = ref(null)
const showTrackerConsole = ref(false)

function closeTrackerLogs() {
  closeStream()
  selectedTrackerCmd.value = null
  showTrackerConsole.value = false
}

function connectTrackerStream(commandId) {
  openCommandStream(commandId, {
    onInit(payload) {
      if (!selectedTrackerCmd.value || selectedTrackerCmd.value.id !== commandId) return
      selectedTrackerCmd.value = {
        ...selectedTrackerCmd.value,
        status: payload.status || selectedTrackerCmd.value.status,
        output: payload.output ?? selectedTrackerCmd.value.output,
      }
    },
    onChunk(payload) {
      if (!selectedTrackerCmd.value || selectedTrackerCmd.value.id !== commandId) return
      selectedTrackerCmd.value = {
        ...selectedTrackerCmd.value,
        output: (selectedTrackerCmd.value.output || '') + (payload.chunk || ''),
      }
    },
    onStatus(payload) {
      if (!selectedTrackerCmd.value || selectedTrackerCmd.value.id !== commandId) return
      selectedTrackerCmd.value = {
        ...selectedTrackerCmd.value,
        status: payload.status || selectedTrackerCmd.value.status,
        output: payload.output ?? selectedTrackerCmd.value.output,
      }
    },
  })
}

async function openTrackerLogs(commandId) {
  closeStream()
  try {
    const res = await api.getCommandStatus(commandId)
    selectedTrackerCmd.value = res.data
    showTrackerConsole.value = true
    if (res.data?.status === 'pending' || res.data?.status === 'running') {
      connectTrackerStream(commandId)
    }
  } catch {
    // Keep page usable even if command history entry vanished.
  }
}
</script>

