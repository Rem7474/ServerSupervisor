<template>
  <div>
    <div class="page-header mb-3">
      <div>
        <div class="page-pretitle">
          <router-link
            to="/git-webhooks"
            class="text-decoration-none"
          >
            Git Webhooks
          </router-link>
          <span class="text-muted mx-1">/</span>
          <span>{{ webhook?.name || id }}</span>
        </div>
        <h2 class="page-title d-flex align-items-center gap-2">
          {{ webhook?.name }}
          <span
            v-if="webhook"
            class="badge"
            :class="providerBadge(webhook.provider)"
          >{{ webhook.provider }}</span>
          <span
            v-if="webhook && !webhook.enabled"
            class="badge bg-secondary-lt text-secondary"
          >Désactivé</span>
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
      v-else-if="webhook"
      class="row g-3"
    >
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
            <h3 class="card-title">
              Configuration
            </h3>
            <router-link
              to="/git-webhooks"
              class="btn btn-sm btn-ghost-secondary"
              @click.prevent="openEdit"
            >
              Modifier
            </router-link>
          </div>
          <div class="card-body">
            <dl class="row mb-0 small">
              <dt class="col-5 text-muted">
                Événement
              </dt>
              <dd class="col-7">
                {{ webhook.event_filter }}
              </dd>
              <dt class="col-5 text-muted">
                Filtre repo
              </dt>
              <dd class="col-7">
                {{ webhook.repo_filter || '<tous>' }}
              </dd>
              <dt class="col-5 text-muted">
                Filtre branche
              </dt>
              <dd class="col-7">
                {{ webhook.branch_filter || '<toutes>' }}
              </dd>
              <dt class="col-5 text-muted">
                VM cible
              </dt>
              <dd class="col-7">
                {{ webhook.host_name || webhook.host_id }}
              </dd>
              <dt class="col-5 text-muted">
                Tâche
              </dt>
              <dd class="col-7">
                <code>{{ webhook.custom_task_id }}</code>
              </dd>
              <dt
                v-if="webhook.notify_channels?.length"
                class="col-5 text-muted"
              >
                Notifications
              </dt>
              <dd
                v-if="webhook.notify_channels?.length"
                class="col-7"
              >
                <span
                  v-for="ch in webhook.notify_channels"
                  :key="ch"
                  class="badge me-1"
                  :class="channelBadge(ch)"
                >{{ ch }}</span>
                <span class="text-muted">({{ [webhook.notify_on_success && 'succès', webhook.notify_on_failure && 'échec'].filter(Boolean).join(', ') || 'aucune' }})</span>
              </dd>
              <dt class="col-5 text-muted">
                Créé le
              </dt>
              <dd class="col-7">
                {{ formatDateTime(webhook.created_at) }}
              </dd>
              <dt
                v-if="webhook.last_triggered_at"
                class="col-5 text-muted"
              >
                Dernier déclench.
              </dt>
              <dd
                v-if="webhook.last_triggered_at"
                class="col-7"
              >
                <RelativeTime :date="webhook.last_triggered_at" />
              </dd>
            </dl>
          </div>
        </div>

        <!-- Variables disponibles -->
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
        <WebhookExecutionList
          :executions="executions"
          kind="webhook"
          title="Historique des exécutions"
          empty-text="Aucune exécution enregistrée."
          :show-refresh="true"
          logs-mode="inline"
          @refresh="loadExecutions"
          @open-logs="openExecutionLogs"
          @open-payload="selectedPayload = $event"
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
      mode="webhook"
      :item="webhook"
      :hosts="hosts"
      :saving="saving"
      :error="modalError"
      @close="closeEdit"
      @submit="saveEdit"
    />

    <PayloadViewerModal
      :payload="selectedPayload"
      @close="selectedPayload = null"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import RelativeTime from '../components/RelativeTime.vue'
import WebhookUrlCard from '../components/webhooks/WebhookUrlCard.vue'
import WebhookExecutionList from '../components/webhooks/WebhookExecutionList.vue'
import WebhookModal from '../components/webhooks/WebhookModal.vue'
import PayloadViewerModal from '../components/webhooks/PayloadViewerModal.vue'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'
import { useGitWebhookDetail } from '../composables/useGitWebhookDetail'

const selectedPayload = ref<string | null>(null)

const {
  id,
  webhook,
  executions,
  hosts,
  loading,
  error,
  revealedSecret,
  selectedCmd,
  showConsole,
  showModal,
  saving,
  modalError,
  envVars,
  formatDateTime,
  loadExecutions,
  clearExecutionLogs,
  openExecutionLogs,
  openEdit,
  saveEdit,
  closeEdit,
  onSecretRegenerated,
  providerBadge,
  channelBadge,
} = useGitWebhookDetail()
</script>

