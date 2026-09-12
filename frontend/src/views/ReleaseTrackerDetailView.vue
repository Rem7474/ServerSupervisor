<template>
  <div>
    <div class="page-header mb-3">
      <div>
        <div class="page-pretitle">
          <router-link
            to="/git-webhooks?tab=trackers"
            class="text-decoration-none"
          >
            {{ t('webhooks.versionTrackingTab') }}
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
            class="badge bg-secondary-lt text-secondary"
          >{{ t('webhooks.disabledBadge') }}</span>
          <span
            v-if="tracker && cooldownActive"
            class="badge bg-warning-lt text-warning"
            :title="t('webhooks.plannedDeploymentTooltip', { eta: cooldownEtaText })"
          >{{ t('webhooks.cooldownActiveLabel', { remaining: cooldownRemainingText }) }}</span>
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
      v-else-if="tracker"
      class="row g-3"
    >
      <!-- Latest release info card -->
      <div
        v-if="tracker?.last_release_tag"
        class="col-lg-12"
      >
        <div class="card bg-primary-lt border-primary">
          <div class="card-body">
            <h4 class="card-title">
              {{ t('webhooks.latestVersionDetectedTitle') }}
            </h4>
            <dl class="row mb-0 small">
              <dt class="col-sm-3 text-muted">
                {{ t('webhooks.versionColumn') }}
              </dt>
              <dd class="col-sm-9">
                <code class="fs-6">{{ tracker.last_release_tag }}</code>
              </dd>
              <template v-if="tracker.docker_image">
                <dt class="col-sm-3 text-muted">
                  {{ t('webhooks.imageAndTagLabel') }}
                </dt>
                <dd class="col-sm-9">
                  <code>{{ tracker.docker_image }}:{{ tracker.last_release_tag }}</code>
                </dd>
              </template>
              <template v-if="tracker.release_url">
                <dt class="col-sm-3 text-muted">
                  {{ t('webhooks.releaseColumn') }}
                </dt>
                <dd class="col-sm-9">
                  <a
                    :href="tracker.release_url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="link-primary"
                  >
                    {{ t('webhooks.viewOnGithubLink') }}
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
            {{ t('webhooks.noTaskConfiguredTitle') }}
          </h4>
          <p class="mb-2">
            {{ t('webhooks.noTaskConfiguredMessage') }}
          </p>
          <p class="mb-2 small text-muted">
            {{ t('webhooks.createTaskHint') }}
          </p>
          <router-link
            :to="`/hosts/${tracker.host_id}`"
            class="btn btn-sm btn-warning"
          >
            {{ t('webhooks.createTaskButton') }}
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
          :title="t('webhooks.executionHistoryTitle')"
          :empty-text="t('webhooks.noExecutionRecordedTitle')"
          :show-refresh="true"
          logs-mode="inline"
          @refresh="loadExecutions"
          @open-logs="openExecutionLogs"
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
import { useI18n } from 'vue-i18n'
import WebhookExecutionList from '../components/webhooks/WebhookExecutionList.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import WebhookModal from '../components/webhooks/WebhookModal.vue'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'
import TrackerConfigCard from '../components/webhooks/TrackerConfigCard.vue'
import TrackerScriptHelpCard from '../components/webhooks/TrackerScriptHelpCard.vue'
import TrackerVersionHistoryCard from '../components/webhooks/TrackerVersionHistoryCard.vue'
import { useReleaseTrackerDetail } from '../composables/useReleaseTrackerDetail'

const { t } = useI18n()

const {
  id,
  tracker,
  executions,
  versionHistory,
  hosts,
  loading,
  error,
  historyLoading,
  checking,
  running,
  selectedCmd,
  showConsole,
  composeProjects,
  tasksYaml,
  loadingSnippet,
  showModal,
  saving,
  modalError,
  canRunManually,
  runDisabledReason,
  cooldownActive,
  cooldownRemainingText,
  cooldownEtaText,
  loadExecutions,
  clearExecutionLogs,
  openExecutionLogs,
  runManually,
  triggerCheck,
  openEdit,
  saveEdit,
  closeEdit,
  providerBadge,
} = useReleaseTrackerDetail()
</script>
