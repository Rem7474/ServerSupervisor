<template>
  <div
    v-if="visible"
    ref="modalRef"
    class="modal modal-blur show d-block"
    style="background:rgba(0,0,0,.5)"
    role="dialog"
    aria-modal="true"
  >
    <div class="modal-dialog modal-lg">
      <div class="modal-content">
        <div class="modal-header">
          <h5 class="modal-title">
            {{ title }}
          </h5>
          <button
            type="button"
            class="btn-close"
            @click="close"
          />
        </div>
        <div class="modal-body">
          <div
            v-if="errorMessage"
            class="alert alert-danger"
          >
            {{ errorMessage }}
          </div>

          <div class="row g-3">
            <div class="col-12">
              <label
                for="webhook-name"
                class="form-label required"
              >{{ t('webhooks.nameLabel') }}</label>
              <input
                id="webhook-name"
                v-model="form.name"
                type="text"
                class="form-control"
                :placeholder="mode === 'webhook' ? t('webhooks.namePlaceholderWebhook') : t('webhooks.namePlaceholderTracker')"
              >
            </div>

            <!-- ===== WEBHOOK FIELDS ===== -->
            <template v-if="mode === 'webhook'">
              <div class="col-md-6">
                <label
                  for="webhook-provider"
                  class="form-label required"
                >Provider</label>
                <select
                  id="webhook-provider"
                  v-model="form.provider"
                  class="form-select"
                >
                  <option value="github">
                    GitHub
                  </option>
                  <option value="gitlab">
                    GitLab
                  </option>
                  <option value="gitea">
                    Gitea
                  </option>
                  <option value="forgejo">
                    Forgejo
                  </option>
                  <option value="custom">
                    Custom
                  </option>
                </select>
              </div>
              <div class="col-md-6">
                <label
                  for="webhook-event-filter"
                  class="form-label"
                >{{ t('webhooks.eventLabel') }}</label>
                <select
                  id="webhook-event-filter"
                  v-model="form.event_filter"
                  class="form-select"
                >
                  <option value="push">
                    push
                  </option>
                  <option value="tag">
                    tag / create
                  </option>
                  <option value="release">
                    release
                  </option>
                </select>
              </div>
              <div class="col-md-6">
                <label
                  for="webhook-repo-filter"
                  class="form-label"
                >{{ t('webhooks.repoFilterLabel') }} <span class="text-muted">{{ t('webhooks.optionalLabel') }}</span></label>
                <input
                  id="webhook-repo-filter"
                  v-model="form.repo_filter"
                  type="text"
                  class="form-control"
                  placeholder="ex: monorg/mon-app"
                >
              </div>
              <div class="col-md-6">
                <label
                  for="webhook-branch-filter"
                  class="form-label"
                >{{ t('webhooks.branchFilterLabel') }} <span class="text-muted">{{ t('webhooks.optionalLabel') }}</span></label>
                <input
                  id="webhook-branch-filter"
                  v-model="form.branch_filter"
                  type="text"
                  class="form-control"
                  placeholder="ex: main"
                >
              </div>
            </template>

            <!-- ===== TRACKER FIELDS ===== -->
            <template v-else>
              <WebhookTrackerFields
                v-model:container-source-host-id="containerSourceHostId"
                v-model:form="form"
                :registry-credentials="registryCredentials"
                :container-hosts="containerHosts"
                :containers-for-host="containersForHost"
                :container-key="containerKey"
                :selected-container-key="selectedContainerKey"
                :selected-container-missing="selectedContainerMissing"
                @select-container="selectContainer"
              />
            </template>

            <!-- VM + Task -->
            <!-- Trackers: optional dispatch toggle -->
            <div
              v-if="mode === 'tracker'"
              class="col-12"
            >
              <label class="form-check form-switch">
                <input
                  v-model="form.dispatch_task"
                  class="form-check-input"
                  type="checkbox"
                >
                <span class="form-check-label fw-medium">{{ t('webhooks.dispatchTaskSwitchLabel') }}</span>
              </label>
              <div
                id="dispatch-task-hint"
                class="form-hint text-muted"
              >
                {{ t('webhooks.monitorOnlyHint') }}
              </div>
            </div>

            <div
              v-if="mode === 'tracker' && form.dispatch_task"
              class="col-md-4"
            >
              <label
                for="webhook-cooldown-hours"
                class="form-label"
              >{{ t('webhooks.cooldownHoursLabel') }}</label>
              <input
                id="webhook-cooldown-hours"
                v-model.number="form.cooldown_hours"
                type="number"
                min="0"
                max="168"
                class="form-control"
                placeholder="0"
              >
              <div class="form-hint">
                {{ t('webhooks.cooldownDelayHint') }}
              </div>
            </div>

            <!-- Docker tracker: choose deployment mode -->
            <div
              v-if="mode === 'tracker' && form.dispatch_task && form.tracker_type === 'docker'"
              class="col-12"
            >
              <div class="form-label">
                {{ t('webhooks.updateModeLabel') }}
              </div>
              <div class="row g-2">
                <div class="col-6">
                  <label
                    class="tracker-type-card"
                    :class="form.update_action === 'compose' ? 'tracker-type-card--active' : 'tracker-type-card--idle'"
                  >
                    <input
                      v-model="form.update_action"
                      class="tracker-type-input"
                      type="radio"
                      value="compose"
                    >
                    <span>
                      <span class="fw-semibold d-block">{{ t('webhooks.composeNativeLabel') }}</span>
                      <span class="text-muted small">{{ t('webhooks.composeNativeDescription') }}</span>
                    </span>
                  </label>
                </div>
                <div class="col-6">
                  <label
                    class="tracker-type-card"
                    :class="form.update_action !== 'compose' ? 'tracker-type-card--active' : 'tracker-type-card--idle'"
                  >
                    <input
                      v-model="form.update_action"
                      class="tracker-type-input"
                      type="radio"
                      value="custom"
                    >
                    <span>
                      <span class="fw-semibold d-block">{{ t('webhooks.customTaskModeLabel') }}</span>
                      <span class="text-muted small">{{ t('webhooks.customTaskModeDescription') }}</span>
                    </span>
                  </label>
                </div>
              </div>
            </div>

            <template v-if="mode === 'webhook' || (mode === 'tracker' && form.dispatch_task)">
              <div class="col-md-6">
                <label
                  for="webhook-host-id"
                  class="form-label"
                  :class="(mode === 'webhook' || (mode === 'tracker' && form.dispatch_task)) ? 'required' : ''"
                >{{ t('webhooks.targetVmSelectLabel') }}</label>
                <select
                  id="webhook-host-id"
                  v-model="form.host_id"
                  class="form-select"
                >
                  <option value="">
                    {{ t('webhooks.selectHostOption') }}
                  </option>
                  <option
                    v-for="host in hosts"
                    :key="host.id"
                    :value="host.id"
                  >
                    {{ host.name }}
                  </option>
                </select>
              </div>

              <!-- Compose mode: project + service -->
              <template v-if="isComposeMode">
                <div class="col-md-6">
                  <label
                    for="webhook-compose-project"
                    class="form-label required"
                  >{{ t('webhooks.composeProjectLabel') }}</label>
                  <input
                    id="webhook-compose-project"
                    v-model="form.compose_project"
                    type="text"
                    class="form-control"
                    placeholder="ex: mon-app"
                    aria-describedby="compose-project-hint"
                  >
                  <div
                    id="compose-project-hint"
                    class="form-hint"
                  >
                    {{ t('webhooks.composeProjectHint', { label: 'com.docker.compose.project' }) }}
                  </div>
                </div>
                <div class="col-md-6">
                  <label
                    for="webhook-compose-service"
                    class="form-label"
                  >{{ t('webhooks.serviceLabel') }} <span class="text-muted">{{ t('webhooks.optionalLabel') }}</span></label>
                  <input
                    id="webhook-compose-service"
                    v-model="form.compose_service"
                    type="text"
                    class="form-control"
                    :placeholder="t('webhooks.leaveEmptyWholeProjectPlaceholder')"
                  >
                </div>
                <div class="col-md-4">
                  <label
                    for="webhook-healthcheck-timeout"
                    class="form-label"
                  >{{ t('webhooks.healthcheckSecondsLabel') }}</label>
                  <input
                    id="webhook-healthcheck-timeout"
                    v-model.number="form.healthcheck_timeout_sec"
                    type="number"
                    min="0"
                    max="3600"
                    class="form-control"
                    placeholder="0"
                  >
                  <div class="form-hint">
                    {{ t('webhooks.healthcheckHint') }}
                  </div>
                </div>
                <div class="col-md-8 d-flex align-items-end">
                  <div class="d-flex flex-wrap gap-3 pb-2">
                    <label class="form-check">
                      <input
                        v-model="form.rollback_on_failure"
                        class="form-check-input"
                        type="checkbox"
                      >
                      <span class="form-check-label">{{ t('webhooks.rollbackUnhealthyLabel') }}</span>
                    </label>
                    <label class="form-check">
                      <input
                        v-model="form.cleanup_after_update"
                        class="form-check-input"
                        type="checkbox"
                      >
                      <span class="form-check-label">{{ t('webhooks.cleanupOrphanImagesShortLabel') }}</span>
                    </label>
                    <label class="form-check">
                      <input
                        v-model="form.reconcile_drift"
                        class="form-check-input"
                        type="checkbox"
                      >
                      <span class="form-check-label">{{ t('webhooks.reconcileDriftLabel') }}</span>
                    </label>
                  </div>
                </div>
                <div
                  v-if="form.update_action === 'compose'"
                  class="col-12"
                >
                  <div class="form-hint mt-0">
                    {{ t('webhooks.driftExplanationHint') }}
                  </div>
                </div>
                <div class="col-md-6">
                  <label
                    for="webhook-pre-update-task-id"
                    class="form-label"
                  >{{ t('webhooks.preUpdateHookLabel') }} <span class="text-muted">{{ t('webhooks.optionalLabel') }}</span></label>
                  <select
                    v-if="customTasks.length"
                    id="webhook-pre-update-task-id"
                    v-model="form.pre_update_task_id"
                    class="form-select"
                  >
                    <option value="">
                      {{ t('webhooks.noneOption') }}
                    </option>
                    <option
                      v-for="task in customTasks"
                      :key="task.id"
                      :value="task.id"
                    >
                      {{ task.name }} ({{ task.id }})
                    </option>
                  </select>
                  <input
                    v-else
                    id="webhook-pre-update-task-id"
                    v-model="form.pre_update_task_id"
                    type="text"
                    class="form-control"
                    placeholder="ex: backup-postgres"
                  >
                  <div class="form-hint">
                    {{ t('webhooks.preUpdateHookHint', { code: 'tasks.yaml' }) }}
                  </div>
                </div>
                <div class="col-md-6">
                  <label
                    for="webhook-post-update-task-id"
                    class="form-label"
                  >{{ t('webhooks.postUpdateHookLabel') }} <span class="text-muted">{{ t('webhooks.optionalLabel') }}</span></label>
                  <select
                    v-if="customTasks.length"
                    id="webhook-post-update-task-id"
                    v-model="form.post_update_task_id"
                    class="form-select"
                  >
                    <option value="">
                      {{ t('webhooks.noneOption') }}
                    </option>
                    <option
                      v-for="task in customTasks"
                      :key="task.id"
                      :value="task.id"
                    >
                      {{ task.name }} ({{ task.id }})
                    </option>
                  </select>
                  <input
                    v-else
                    id="webhook-post-update-task-id"
                    v-model="form.post_update_task_id"
                    type="text"
                    class="form-control"
                    placeholder="ex: verify-health"
                  >
                </div>
              </template>

              <!-- Custom / webhook mode: tasks.yaml task -->
              <div
                v-else
                class="col-md-6"
              >
                <label
                  for="webhook-custom-task-id"
                  class="form-label"
                  :class="(mode === 'webhook' || (mode === 'tracker' && form.dispatch_task)) ? 'required' : ''"
                >{{ t('webhooks.customTaskLabel') }}</label>
                <select
                  v-if="customTasks.length"
                  id="webhook-custom-task-id"
                  v-model="form.custom_task_id"
                  class="form-select"
                >
                  <option
                    value=""
                    disabled
                  >
                    {{ t('webhooks.selectTaskOption') }}
                  </option>
                  <option
                    v-for="task in customTasks"
                    :key="task.id"
                    :value="task.id"
                  >
                    {{ task.name }} ({{ task.id }})
                  </option>
                </select>
                <input
                  v-else
                  id="webhook-custom-task-id"
                  v-model="form.custom_task_id"
                  type="text"
                  class="form-control"
                  :placeholder="mode === 'webhook' ? 'ex: deploy-mon-app' : 'ex: update-home-assistant'"
                  aria-describedby="task-id-hint"
                >
                <div
                  id="task-id-hint"
                  class="form-hint"
                >
                  {{ t('webhooks.taskIdHint', { id: 'id', tasksYaml: 'tasks.yaml' }) }}
                </div>
              </div>
            </template>

            <!-- Notifications -->
            <div class="col-12">
              <div class="form-label">
                {{ t('webhooks.notificationsLabel') }}
              </div>
              <div class="d-flex flex-wrap gap-3 mt-1">
                <label
                  v-if="mode === 'webhook'"
                  class="form-check"
                >
                  <input
                    v-model="form.notify_on_success"
                    class="form-check-input"
                    type="checkbox"
                  >
                  <span class="form-check-label">{{ t('webhooks.onSuccessLabel') }}</span>
                </label>
                <label
                  v-if="mode === 'webhook'"
                  class="form-check"
                >
                  <input
                    v-model="form.notify_on_failure"
                    class="form-check-input"
                    type="checkbox"
                  >
                  <span class="form-check-label">{{ t('webhooks.onFailureLabel') }}</span>
                </label>
                <label
                  v-if="mode === 'tracker'"
                  class="form-check"
                >
                  <input
                    v-model="form.notify_on_release"
                    class="form-check-input"
                    type="checkbox"
                    :disabled="!form.notify_channels.length"
                  >
                  <span class="form-check-label">{{ t('webhooks.notifyOnEveryUpdateLabel') }}</span>
                </label>
              </div>
              <div class="d-flex flex-wrap gap-3 mt-2">
                <label
                  v-for="channel in ['smtp', 'ntfy', 'browser']"
                  :key="channel"
                  class="form-check"
                >
                  <input
                    v-model="form.notify_channels"
                    class="form-check-input"
                    type="checkbox"
                    :value="channel"
                  >
                  <span class="form-check-label">{{ channel }}</span>
                </label>
              </div>
              <div
                v-if="mode === 'tracker'"
                class="form-hint mt-2"
              >
                {{ t('webhooks.enableChannelHint') }}
              </div>
            </div>

            <div class="col-12 border-top pt-3">
              <label class="form-check form-switch mb-0">
                <input
                  v-model="form.enabled"
                  class="form-check-input"
                  type="checkbox"
                >
                <span class="form-check-label fw-medium">{{ mode === 'tracker' ? t('webhooks.enableThisTrackerLabel') : t('webhooks.enableThisWebhookLabel') }}</span>
              </label>
            </div>
          </div>

          <!-- Env vars table for trackers -->
          <WebhookEnvVarsCard
            v-if="mode === 'tracker'"
            :env-vars="currentEnvVars"
          />
        </div>
        <div class="modal-footer">
          <button
            type="button"
            class="btn btn-outline-secondary"
            @click="close"
          >
            {{ t('webhooks.cancelButton') }}
          </button>
          <button
            type="button"
            class="btn btn-primary"
            :disabled="saving"
            @click="submit"
          >
            {{ saving ? t('webhooks.savingLabel') : submitLabel }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useModalChrome } from '../../composables/useModalChrome'
import { useWebhookForm, type WebhookItem, type Host, type WebhookFormData } from '../../composables/useWebhookForm'
import WebhookTrackerFields from './WebhookTrackerFields.vue'
import WebhookEnvVarsCard from './WebhookEnvVarsCard.vue'

const props = withDefaults(defineProps<{
  visible?: boolean
  mode?: string
  item?: WebhookItem | null
  hosts?: Host[]
  saving?: boolean
  error?: string
  prefillDockerImage?: string
  prefillDockerTag?: string
  prefillComposeProject?: string
}>(), {
  visible: false,
  mode: 'webhook',
  item: null,
  hosts: () => [],
  saving: false,
  error: '',
  prefillDockerImage: '',
  prefillDockerTag: '',
  prefillComposeProject: '',
})

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', payload: WebhookFormData): void
}>()

const { t } = useI18n()

const modalRef = ref<HTMLElement | null>(null)
useModalChrome(modalRef, () => props.visible, { onClose: close })

const {
  form,
  customTasks,
  registryCredentials,
  containerSourceHostId,
  containerHosts,
  containersForHost,
  containerKey,
  selectedContainerKey,
  selectedContainerMissing,
  selectContainer,
  currentEnvVars,
  isComposeMode,
  title,
  submitLabel,
  errorMessage,
  submit,
  clearError,
} = useWebhookForm(props, (payload) => emit('submit', payload))

function close(): void {
  clearError()
  emit('close')
}
</script>

<style scoped>
.tracker-type-card {
  display: block;
  width: 100%;
  padding: 1rem;
  border-radius: 0.5rem;
  border: 1px solid var(--tblr-border-color);
  cursor: pointer;
  transition: border-color 0.18s ease, background-color 0.18s ease;
}

.tracker-type-card--active {
  border-color: var(--tblr-primary);
  background: var(--tblr-primary-lt);
}

.tracker-type-card--idle {
  border-color: var(--tblr-border-color);
  background: transparent;
}

.tracker-type-input {
  position: absolute;
  opacity: 0;
  width: 0;
  height: 0;
  pointer-events: none;
}
</style>
