<template>
  <div class="card">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">
        {{ t('webhooks.configurationTitle') }}
      </h3>
      <div class="d-flex gap-2">
        <button
          type="button"
          class="btn btn-sm btn-ghost-secondary"
          :disabled="checking"
          @click="$emit('check')"
        >
          <IconRefresh
            :size="14"
            class="me-1"
          />
          {{ checking ? t('webhooks.checkingLabel') : t('webhooks.checkNowButton') }}
        </button>
        <button
          type="button"
          class="btn btn-sm btn-outline-success"
          :disabled="running || !canRunManually"
          :title="runDisabledReason"
          @click="$emit('run')"
        >
          <IconPlayerPlay
            :size="14"
            class="me-1"
          />
          {{ running ? t('webhooks.triggeringLabel') : t('webhooks.executeButton') }}
        </button>
        <button
          type="button"
          class="btn btn-sm btn-ghost-secondary"
          @click="$emit('edit')"
        >
          {{ t('webhooks.editButton') }}
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
          >{{ t('webhooks.dockerImageBadge') }}</span>
          <span
            v-else
            class="badge bg-blue-lt text-blue"
          >{{ t('webhooks.gitReleaseBadge') }}</span>
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
            {{ t('webhooks.repositoryLabel') }}
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
            {{ t('webhooks.lastReleaseLabel') }}
          </dt>
          <dd class="col-7">
            <span
              v-if="tracker.last_release_tag"
              class="badge bg-success-lt text-success"
            >{{ tracker.last_release_tag }}</span>
            <span
              v-else
              class="text-muted"
            >{{ t('webhooks.pendingEllipsisLabel') }}</span>
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
            {{ t('webhooks.watchedTagLabel') }}
          </dt>
          <dd class="col-7">
            <code>{{ tracker.docker_tag || 'latest' }}</code>
          </dd>
          <template v-if="tracker.latest_image_digest">
            <dt class="col-5 text-muted">
              {{ t('webhooks.lastDigestLabel') }}
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
            {{ t('webhooks.lastCheckLabel') }}
          </dt>
          <dd class="col-7">
            <span v-if="tracker.last_checked_at"><RelativeTime :date="tracker.last_checked_at" /></span>
            <span
              v-else
              class="text-muted"
            >{{ t('webhooks.neverLabel') }}</span>
          </dd>

          <template v-if="tracker.repo_owner && tracker.repo_name">
            <dt class="col-5 text-muted">
              {{ t('webhooks.linkedRepoLabel') }}
            </dt>
            <dd class="col-7">
              <a
                :href="repoURL"
                target="_blank"
                class="link-primary"
              >
                {{ tracker.repo_owner }}/{{ tracker.repo_name }}
              </a>
              <div class="small mt-1">
                <a
                  :href="releaseNotesURL"
                  target="_blank"
                  class="link-secondary"
                >{{ t('webhooks.viewReleaseNotesLink') }}</a>
              </div>
            </dd>
          </template>
        </template>

        <!-- Common fields -->
        <template v-if="tracker.host_id && tracker.custom_task_id">
          <dt class="col-5 text-muted">
            {{ t('webhooks.targetVmLabel') }}
          </dt>
          <dd class="col-7">
            {{ tracker.host_name || tracker.host_id }}
          </dd>
          <dt class="col-5 text-muted">
            {{ t('webhooks.taskLabel') }}
          </dt>
          <dd class="col-7">
            <code>{{ tracker.custom_task_id }}</code>
          </dd>
        </template>
        <template v-else-if="!tracker.host_id || !tracker.custom_task_id">
          <dt class="col-5 text-muted">
            {{ t('webhooks.modeLabel') }}
          </dt>
          <dd class="col-7">
            <span class="badge bg-blue-lt text-blue">{{ t('webhooks.monitoringOnlyBadge') }}</span>
          </dd>
        </template>
        <dt
          v-if="tracker.tracker_type !== 'docker' && tracker.last_checked_at"
          class="col-5 text-muted"
        >
          {{ t('webhooks.lastCheckLabel') }}
        </dt>
        <dd
          v-if="tracker.tracker_type !== 'docker' && tracker.last_checked_at"
          class="col-7"
        >
          <RelativeTime :date="tracker.last_checked_at" />
        </dd>
        <template v-if="tracker.last_error">
          <dt class="col-5 text-muted">
            {{ t('common.error') }}
          </dt>
          <dd class="col-7 text-danger small">
            {{ tracker.last_error }}
          </dd>
        </template>
        <dt
          v-if="tracker.last_triggered_at"
          class="col-5 text-muted"
        >
          {{ t('webhooks.lastTriggeredLabel') }}
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
          {{ t('webhooks.notificationsLabel') }}
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
          {{ t('webhooks.createdOnLabel') }}
        </dt>
        <dd class="col-7">
          {{ formatDateTime(tracker.created_at) }}
        </dd>
        <template v-if="Number(tracker.cooldown_hours || 0) > 0">
          <dt class="col-5 text-muted">
            {{ t('webhooks.cooldownLabel') }}
          </dt>
          <dd class="col-7">
            {{ `${tracker.cooldown_hours}h` }}
          </dd>
        </template>
        <template v-if="cooldownActive">
          <dt class="col-5 text-muted">
            {{ t('webhooks.plannedDeploymentLabel') }}
          </dt>
          <dd class="col-7">
            {{ cooldownEtaText }}
          </dd>
        </template>
      </dl>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconPlayerPlay, IconRefresh } from '@tabler/icons-vue'
import RelativeTime from '../RelativeTime.vue'
import { formatDateTime } from '../../utils/formatters'
import type { ReleaseTracker } from '../../types/tracker'

const { t } = useI18n()

const props = defineProps<{
  tracker: ReleaseTracker
  checking: boolean
  running: boolean
  canRunManually: boolean
  runDisabledReason: string
  cooldownActive: boolean
  cooldownEtaText: string
}>()

defineEmits<{
  (e: 'check'): void
  (e: 'run'): void
  (e: 'edit'): void
}>()

const repoURL = computed(() => {
  const tracker = props.tracker
  if (!tracker || !tracker.repo_owner || !tracker.repo_name) return '#'
  switch (tracker.provider) {
    case 'gitlab': return `https://gitlab.com/${tracker.repo_owner}/${tracker.repo_name}`
    case 'gitea': return `https://codeberg.org/${tracker.repo_owner}/${tracker.repo_name}`
    default: return `https://github.com/${tracker.repo_owner}/${tracker.repo_name}`
  }
})

const releaseNotesURL = computed(() => {
  const tracker = props.tracker
  if (!tracker || !tracker.repo_owner || !tracker.repo_name) return '#'
  switch (tracker.provider) {
    case 'gitlab': return `https://gitlab.com/${tracker.repo_owner}/${tracker.repo_name}/-/releases`
    case 'gitea':
    case 'forgejo':
      return `https://codeberg.org/${tracker.repo_owner}/${tracker.repo_name}/releases`
    default:
      return `https://github.com/${tracker.repo_owner}/${tracker.repo_name}/releases`
  }
})

function channelBadge(ch: string): string {
  const map: Record<string, string> = {
    smtp: 'bg-blue-lt text-blue',
    ntfy: 'bg-orange-lt text-orange',
    browser: 'bg-purple-lt text-purple',
  }
  return map[ch] || 'bg-secondary-lt text-secondary'
}
</script>
