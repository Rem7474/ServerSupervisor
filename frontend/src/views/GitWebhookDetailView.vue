<template>
  <div>
    <div class="page-header mb-3">
      <div>
        <div class="page-pretitle">
          <router-link
            to="/git-webhooks"
            class="text-decoration-none"
          >
            {{ t('webhooks.gitWebhooksBreadcrumb') }}
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
          >{{ t('webhooks.disabledBadge') }}</span>
        </h2>
      </div>
    </div>

    <div
      v-if="error"
      class="alert alert-danger mb-3"
    >
      {{ error }}
    </div>

    <LoadingSkeleton
      v-if="loading"
      variant="card"
      :lines="6"
    />

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
              {{ t('webhooks.configurationTitle') }}
            </h3>
            <router-link
              to="/git-webhooks"
              class="btn btn-sm btn-ghost-secondary"
              @click.prevent="openEdit"
            >
              {{ t('webhooks.editButton') }}
            </router-link>
          </div>
          <div class="card-body">
            <dl class="row mb-0 small">
              <dt class="col-5 text-muted">
                {{ t('webhooks.eventLabel') }}
              </dt>
              <dd class="col-7">
                {{ webhook.event_filter }}
              </dd>
              <dt class="col-5 text-muted">
                {{ t('webhooks.repoFilterLabel') }}
              </dt>
              <dd class="col-7">
                {{ webhook.repo_filter || t('webhooks.allReposPlaceholder') }}
              </dd>
              <dt class="col-5 text-muted">
                {{ t('webhooks.branchFilterLabel') }}
              </dt>
              <dd class="col-7">
                {{ webhook.branch_filter || t('webhooks.allBranchesPlaceholder') }}
              </dd>
              <dt class="col-5 text-muted">
                {{ t('webhooks.targetVmLabel') }}
              </dt>
              <dd class="col-7">
                {{ webhook.host_name || webhook.host_id }}
              </dd>
              <dt class="col-5 text-muted">
                {{ t('webhooks.taskLabel') }}
              </dt>
              <dd class="col-7">
                <code>{{ webhook.custom_task_id }}</code>
              </dd>
              <dt
                v-if="webhook.notify_channels?.length"
                class="col-5 text-muted"
              >
                {{ t('webhooks.notificationsLabel') }}
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
                <span class="text-muted">({{ [webhook.notify_on_success && t('webhooks.onSuccessWord'), webhook.notify_on_failure && t('webhooks.onFailureWord')].filter(Boolean).join(', ') || t('webhooks.noneWord') }})</span>
              </dd>
              <dt class="col-5 text-muted">
                {{ t('webhooks.createdOnLabel') }}
              </dt>
              <dd class="col-7">
                {{ formatDateTime(webhook.created_at) }}
              </dd>
              <dt
                v-if="webhook.last_triggered_at"
                class="col-5 text-muted"
              >
                {{ t('webhooks.lastTriggeredLabel') }}
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
              {{ t('webhooks.availableVarsInScriptTitle') }}
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
          :title="t('webhooks.executionHistoryTitle')"
          :empty-text="t('webhooks.noExecutionRecordedTitle')"
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
            :title="t('webhooks.consoleLiveTitle')"
            :empty-text="t('webhooks.selectLogsHint')"
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
import { useI18n } from 'vue-i18n'
import RelativeTime from '../components/RelativeTime.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import WebhookUrlCard from '../components/webhooks/WebhookUrlCard.vue'
import WebhookExecutionList from '../components/webhooks/WebhookExecutionList.vue'
import WebhookModal from '../components/webhooks/WebhookModal.vue'
import PayloadViewerModal from '../components/webhooks/PayloadViewerModal.vue'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'
import { useGitWebhookDetail } from '../composables/useGitWebhookDetail'

const { t } = useI18n()

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

