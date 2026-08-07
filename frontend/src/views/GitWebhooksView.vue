<template>
  <div>
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <h2 class="page-title">
            Git / Automatisation
          </h2>
          <div class="text-muted">
            Webhooks entrants et suivi de releases pour déclencher des scripts sur vos VMs.
          </div>
        </div>
        <div class="btn-list">
          <button
            v-if="activeTab === 'trackers'"
            type="button"
            class="btn btn-outline-primary btn-sm"
            @click="showDiscoverModal = true"
          >
            <IconSearch
              :size="14"
              class="icon me-1"
            />
            Découvrir
          </button>
          <button
            type="button"
            class="btn btn-primary btn-sm"
            @click="activeTab === 'webhooks' ? openCreateWebhook() : openCreateTracker()"
          >
            <IconPlus
              :size="14"
              class="icon me-1"
            />
            {{ activeTab === 'webhooks' ? 'Nouveau webhook' : 'Nouveau tracker' }}
          </button>
        </div>
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
          <IconGitBranch
            :size="16"
            class="icon me-1"
          />
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
          <IconActivity
            :size="16"
            class="icon me-1"
          />
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
      <LoadingSkeleton
        v-if="loadingWebhooks"
        variant="card"
        :lines="3"
      />

      <div
        v-else-if="webhooks.length === 0"
        class="card"
      >
        <div class="card-body">
          <EmptyState
            :icon="IconGitBranch"
            title="Aucun webhook configuré."
            subtitle="Recevez des événements depuis GitHub, GitLab, Gitea ou Forgejo pour déclencher des scripts sur vos VMs."
            cta-label="Créer le premier webhook"
            @cta="openCreateWebhook"
          />
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
                    class="badge bg-secondary-lt text-secondary"
                  >Désactivé</span>
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
                    <router-link
                      v-if="webhook.host_id"
                      :to="`/hosts/${webhook.host_id}`"
                      class="text-truncate text-decoration-none"
                    >
                      {{ webhook.host_name || webhook.host_id }}
                    </router-link>
                    <span
                      v-else
                      class="text-truncate"
                    >{{ webhook.host_name || webhook.host_id }}</span>
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
                    :class="execStatusBadge(webhook.last_execution.status || '')"
                  >{{ commandStatusLabel(webhook.last_execution.status) }}</span>
                  <span class="ms-1 text-muted">{{ formatRelative(webhook.last_execution.triggered_at || '') }}</span>
                </div>
                <div
                  v-else
                  class="mt-2 pt-2 border-top small text-muted"
                >
                  Jamais déclenché
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
                  type="button"
                  class="btn btn-sm btn-outline-secondary"
                  @click="openEditWebhook(webhook)"
                >
                  Modifier
                </button>
                <button
                  type="button"
                  class="btn btn-sm"
                  :class="webhook.enabled ? 'btn-outline-warning' : 'btn-outline-success'"
                  @click="toggleWebhook(webhook)"
                >
                  {{ webhook.enabled ? 'Désactiver' : 'Activer' }}
                </button>
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-danger ms-auto"
                  @click="confirmDeleteWebhook(webhook)"
                >
                  <IconTrash :size="14" />
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
      <LoadingSkeleton
        v-if="loadingTrackers"
        variant="card"
        :lines="3"
      />

      <div
        v-else-if="trackers.length === 0"
        class="card"
      >
        <div class="card-body">
          <EmptyState
            :icon="IconActivity"
            title="Aucun tracker configuré."
            subtitle="Surveillez les releases Git ou les images Docker et déclenchez automatiquement un script sur une VM lors d'une mise à jour."
            cta-label="Créer le premier tracker"
            @cta="openCreateTracker"
          />
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
                    class="badge bg-secondary-lt text-secondary"
                  >Désactivé</span>
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
                  <template v-if="tracker.host_id && tracker.update_action === 'compose' && tracker.compose_project">
                    <div class="d-flex gap-2 mb-1">
                      <span
                        class="text-muted"
                        style="min-width:60px"
                      >VM</span>
                      <router-link
                        :to="`/hosts/${tracker.host_id}`"
                        class="text-truncate text-decoration-none"
                      >
                        {{ tracker.host_name || tracker.host_id }}
                      </router-link>
                    </div>
                    <div class="d-flex gap-2 mb-1">
                      <span
                        class="text-muted"
                        style="min-width:60px"
                      >Compose</span>
                      <code class="small text-truncate">{{ tracker.compose_project }}{{ tracker.compose_service ? ' / ' + tracker.compose_service : '' }}</code>
                    </div>
                  </template>
                  <template v-else-if="tracker.host_id && tracker.custom_task_id">
                    <div class="d-flex gap-2 mb-1">
                      <span
                        class="text-muted"
                        style="min-width:60px"
                      >VM</span>
                      <router-link
                        :to="`/hosts/${tracker.host_id}`"
                        class="text-truncate text-decoration-none"
                      >
                        {{ tracker.host_name || tracker.host_id }}
                      </router-link>
                    </div>
                    <div class="d-flex gap-2 mb-1">
                      <span
                        class="text-muted"
                        style="min-width:60px"
                      >Tache</span>
                      <code class="small text-truncate">{{ tracker.custom_task_id }}</code>
                    </div>
                  </template>
                  <template v-else>
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
                    >Dernière</span>
                    <span class="badge bg-success-lt text-success">{{ tracker.last_release_tag }}</span>
                  </div>
                  <div class="d-flex gap-2 mb-1">
                    <span
                      class="text-muted"
                      style="min-width:60px"
                    >Vérifiée</span>
                    <span>{{ formatDateOnly(tracker.last_checked_at || tracker.last_triggered_at || tracker.last_execution?.triggered_at) }}</span>
                  </div>
                  <div
                    v-if="Number(tracker.cooldown_hours || 0) > 0"
                    class="d-flex gap-2 mb-1"
                  >
                    <span
                      class="text-muted"
                      style="min-width:60px"
                    >Cooldown</span>
                    <span>{{ `${tracker.cooldown_hours}h` }}</span>
                  </div>
                </div>
                <div class="mt-2 pt-2 border-top small">
                  <div
                    v-if="isCooldownActive(tracker)"
                    class="mb-2"
                  >
                    <span
                      class="badge bg-warning-lt text-warning"
                      :title="`Déploiement prévu: ${cooldownEtaLabel(tracker)}`"
                    >Cooldown actif · reste {{ cooldownRemainingLabel(tracker) }}</span>
                  </div>
                  <template v-if="tracker.last_execution">
                    <span class="text-muted">Dernière exécution :</span>
                    <span
                      class="ms-1 badge"
                      :class="execStatusBadge(tracker.last_execution.status || '')"
                    >{{ commandStatusLabel(tracker.last_execution.status) }}</span>
                    <span class="ms-1 text-muted">{{ formatRelative(tracker.last_execution.triggered_at || '') }}</span>
                  </template>
                  <template v-else-if="tracker.last_checked_at">
                    <span class="text-muted">Dernière vérif : {{ formatRelative(tracker.last_checked_at) }}</span>
                    <span
                      v-if="tracker.last_error"
                      class="ms-1 badge bg-danger-lt text-danger"
                      :title="(tracker.last_error as string)"
                    >erreur</span>
                    <span
                      v-else-if="!tracker.last_release_tag && tracker.tracker_type !== 'docker'"
                      class="ms-1 badge bg-warning-lt text-warning"
                    >aucune release trouvée</span>
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
                  type="button"
                  class="btn btn-sm btn-outline-secondary"
                  @click="openEditTracker(tracker)"
                >
                  Modifier
                </button>
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-secondary"
                  title="Verifier maintenant"
                  @click="checkNow(tracker)"
                >
                  <IconRefresh :size="14" />
                </button>
                <button
                  type="button"
                  class="btn btn-sm"
                  :class="tracker.enabled ? 'btn-outline-warning' : 'btn-outline-success'"
                  @click="toggleTracker(tracker)"
                >
                  {{ tracker.enabled ? 'Désactiver' : 'Activer' }}
                </button>
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-danger ms-auto"
                  @click="confirmDeleteTracker(tracker)"
                >
                  <IconTrash :size="14" />
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
      :item="(editingWebhook as any)"
      :hosts="(hosts as any)"
      :saving="saving"
      :error="modalError"
      @close="closeWebhookModal"
      @submit="(saveWebhook as any)"
    />

    <WebhookModal
      :visible="showTrackerModal"
      mode="tracker"
      :item="(editingTracker as any)"
      :hosts="(hosts as any)"
      :saving="saving"
      :error="modalError"
      :prefill-docker-image="prefillDockerImage"
      :prefill-docker-tag="prefillDockerTag"
      :prefill-compose-project="prefillComposeProject"
      @close="closeTrackerModal"
      @submit="(saveTracker as any)"
    />

    <TrackableContainersModal
      :visible="showDiscoverModal"
      @close="showDiscoverModal = false"
      @created="onBulkCreated"
    />

    <template v-if="newWebhookSecret">
      <div
        ref="secretModalRef"
        class="modal modal-blur fade show d-block"
      >
        <div class="modal-dialog modal-md">
          <div class="modal-content">
            <div class="modal-header">
              <h5 class="modal-title">
                Webhook créé
              </h5>
            </div>
            <div class="modal-body">
              <div class="alert alert-warning">
                Copiez ce secret maintenant, il ne sera plus affiché en clair.
              </div>
              <WebhookUrlCard
                :webhook-id="newWebhookId"
                :secret="newWebhookSecret"
                :initial-secret="true"
              />
            </div>
            <div class="modal-footer">
              <button
                type="button"
                class="btn btn-primary"
                @click="closeSecretModal"
              >
                J'ai copié le secret
              </button>
            </div>
          </div>
        </div>
      </div>
      <div class="modal-backdrop fade show" />
    </template>
  </div>
</template>

<script setup lang="ts">
import { useGitWebhooksPage } from '../composables/useGitWebhooksPage'
import { commandStatusLabel } from '../utils/commandStatus'
import { IconActivity, IconGitBranch, IconPlus, IconRefresh, IconSearch, IconTrash } from '@tabler/icons-vue'
import EmptyState from '../components/EmptyState.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import WebhookUrlCard from '../components/webhooks/WebhookUrlCard.vue'
import WebhookExecutionList from '../components/webhooks/WebhookExecutionList.vue'
import WebhookModal from '../components/webhooks/WebhookModal.vue'
import TrackableContainersModal from '../components/webhooks/TrackableContainersModal.vue'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'
import { ref } from 'vue'
import { useModalChrome } from '../composables/useModalChrome'
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
  prefillComposeProject,
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
  loadTrackers,
  repoURL,
  providerBadge,
  execStatusBadge,
  formatRelative,
  formatDateOnly,
  isCooldownActive,
  cooldownRemainingLabel,
  cooldownEtaLabel,
  selectedTrackerCmd,
  showTrackerConsole,
  closeTrackerLogs,
  openTrackerLogs,
} = useGitWebhooksPage()

// No dismiss affordance by design — the secret is shown once, and the
// only way out is the "J'ai copié le secret" button, so ESC/backdrop
// close must stay disabled (persistent: true).
const secretModalRef = ref<HTMLElement | null>(null)
useModalChrome(secretModalRef, () => !!newWebhookSecret.value, { persistent: true })

const showDiscoverModal = ref(false)

async function onBulkCreated(): Promise<void> {
  await loadTrackers()
}
</script>

