<template>
  <tr
    :class="{ 'table-active': selected }"
  >
    <td v-if="canRunDocker">
      <label class="form-check mb-0">
        <input
          class="form-check-input"
          type="checkbox"
          :checked="selected"
          :aria-label="t('docker.selectContainerAriaLabel', { name: container.name })"
          @change="emit('toggle-select', ($event.target as HTMLInputElement).checked)"
        >
      </label>
    </td>
    <td class="fw-semibold text-nowrap">
      {{ container.name }}
    </td>
    <td>
      <router-link
        :to="`/hosts/${container.host_id}`"
        class="text-decoration-none"
      >
        {{ container.hostname }}
      </router-link>
    </td>
    <td>
      <DockerComposeBadge :labels="container.labels" />
    </td>
    <td class="small">
      <div>{{ container.image }}</div>
      <div class="mt-1 d-flex align-items-center gap-1 flex-wrap">
        <code>{{ version?.running_version || container.image_tag }}</code>
        <template v-if="version">
          <span
            v-if="version?.status === 'up_to_date'"
            class="badge bg-success-lt text-success"
          >{{ t('docker.upToDateBadge') }}</span>
          <span
            v-else-if="version?.status === 'update_available'"
            class="badge bg-warning-lt text-warning"
            :title="t('docker.latestVersionTooltip', { version: version?.latest_version })"
          >{{ t('docker.updateAvailableBadge') }}</span>
          <span
            v-else
            class="badge bg-secondary-lt text-secondary"
            :title="unknownVersionTitle"
          >{{ t('docker.unknownVersionBadge') }}</span>
        </template>
      </div>
    </td>
    <td>
      <span :class="getEntityStateClass(container.state)">{{ getEntityStateLabel(container.state) }}</span>
    </td>
    <td class="small">
      <DockerPortBadges
        :ports="ports"
        kind="internal"
      />
    </td>
    <td class="small">
      <DockerPortBadges
        :ports="ports"
        kind="exposed"
      />
    </td>
    <td class="text-secondary small font-monospace">
      <template v-if="container.state === 'running' && ((container.net_rx_bytes ?? 0) > 0 || (container.net_tx_bytes ?? 0) > 0)">
        ↓ {{ formatBytes(container.net_rx_bytes) }} / ↑ {{ formatBytes(container.net_tx_bytes) }}
      </template>
      <span
        v-else
        class="text-muted"
      >—</span>
    </td>
    <td class="text-end">
      <div class="d-flex align-items-center justify-content-end gap-1">
        <template v-if="canRunDocker">
          <button
            v-if="['exited', 'dead', 'created', 'paused'].includes(container.state)"
            type="button"
            :disabled="!!actionLoading"
            class="btn btn-icon btn-sm btn-ghost-success"
            :title="t('docker.verbStart')"
            :aria-label="t('docker.startContainerAriaLabel')"
            @click="emit('action', 'start')"
          >
            <span
              v-if="actionLoading === 'start'"
              class="spinner-border spinner-border-sm"
            />
            <IconPlayerPlay
              v-else
              :size="16"
              class="icon icon-sm"
            />
          </button>
          <button
            v-if="container.state === 'running'"
            type="button"
            :disabled="!!actionLoading"
            class="btn btn-icon btn-sm btn-ghost-danger"
            :title="t('docker.verbStop')"
            :aria-label="t('docker.stopContainerTitle')"
            @click="emit('action', 'stop')"
          >
            <span
              v-if="actionLoading === 'stop'"
              class="spinner-border spinner-border-sm"
            />
            <IconPlayerStop
              v-else
              :size="16"
              class="icon icon-sm"
            />
          </button>
          <button
            v-if="container.state === 'running'"
            type="button"
            :disabled="!!actionLoading"
            class="btn btn-icon btn-sm btn-ghost-warning"
            :title="t('docker.verbRestart')"
            :aria-label="t('docker.restartContainerTitle')"
            @click="emit('action', 'restart')"
          >
            <span
              v-if="actionLoading === 'restart'"
              class="spinner-border spinner-border-sm"
            />
            <IconRefresh
              v-else
              :size="16"
              class="icon icon-sm"
            />
          </button>
          <button
            type="button"
            :disabled="!!actionLoading"
            class="btn btn-icon btn-sm btn-ghost-secondary"
            :title="t('docker.viewProjectLogsTooltip')"
            :aria-label="t('docker.viewContainerLogsAriaLabel')"
            @click="emit('action', 'logs')"
          >
            <span
              v-if="actionLoading === 'logs'"
              class="spinner-border spinner-border-sm"
            />
            <IconList
              v-else
              :size="16"
              class="icon icon-sm"
            />
          </button>
        </template>
        <button
          type="button"
          class="btn btn-icon btn-sm btn-ghost-secondary"
          :title="t('docker.inspectTooltip')"
          :aria-label="t('docker.inspectContainerAriaLabel')"
          @click="emit('inspect')"
        >
          <IconSearch
            :size="16"
            class="icon icon-sm"
          />
        </button>
        <button
          v-if="version?.tracker_id"
          type="button"
          class="btn btn-icon btn-sm btn-ghost-secondary"
          :title="t('docker.viewVersionTrackingTooltip')"
          :aria-label="t('docker.viewVersionTrackingTooltip')"
          @click="emit('open-tracker')"
        >
          <IconChevronRight
            :size="16"
            class="icon icon-sm"
          />
        </button>
        <button
          v-if="version?.tracker_id"
          type="button"
          :disabled="trackerRunDisabled"
          class="btn btn-icon btn-sm btn-ghost-success"
          :title="trackerRunTooltip"
          :aria-label="t('docker.triggerTracker')"
          @click="emit('run-tracker')"
        >
          <span
            v-if="trackerRunLoading"
            class="spinner-border spinner-border-sm"
          />
          <IconPlayerPlay
            v-else
            :size="16"
            class="icon icon-sm"
          />
        </button>
        <button
          type="button"
          class="btn btn-icon btn-sm btn-ghost-secondary"
          :title="t('docker.trackUpdatesTooltip')"
          :aria-label="t('docker.createTrackerAriaLabel')"
          @click="emit('track-image')"
        >
          <IconActivity
            :size="16"
            class="icon icon-sm"
          />
        </button>
        <button
          v-if="composeInfo.project || Object.keys(container.labels || {}).length > 0"
          type="button"
          class="btn btn-sm btn-ghost-secondary"
          :title="composeInfo.project ? t('docker.composeInfoLabelsTooltip') : t('docker.labelsOnlyTooltip')"
          @click="emit('show-info')"
        >
          <IconClipboard
            :size="16"
            class="icon icon-sm"
          />
        </button>
      </div>
    </td>
  </tr>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconActivity, IconChevronRight, IconClipboard, IconList, IconPlayerPlay, IconRefresh, IconSearch, IconPlayerStop } from '@tabler/icons-vue'
import DockerPortBadges from '../common/DockerPortBadges.vue'
import DockerComposeBadge from './DockerComposeBadge.vue'
import { getEntityStateClass, getEntityStateLabel } from '../../utils/statusClasses'
import { getComposeInfo } from '../../utils/dockerCompose'
import { formatBytes as formatBytesShared } from '../../utils/formatters'
import type { DockerPortMapping } from '../../utils/dockerPorts'
import type { Container, VersionComparison } from '../../types/dockerContainersTab'

const props = defineProps<{
  container: Container
  version: VersionComparison | null
  ports: DockerPortMapping[]
  selected: boolean
  canRunDocker: boolean
  /** In-flight lifecycle action for this container, if any. */
  actionLoading?: string | boolean | null
  trackerRunLoading: boolean
  trackerRunDisabled: boolean
  trackerRunTooltip: string
  unknownVersionTitle: string
}>()

const emit = defineEmits<{
  (e: 'toggle-select', checked: boolean): void
  (e: 'action', action: 'start' | 'stop' | 'restart' | 'logs'): void
  (e: 'inspect'): void
  (e: 'open-tracker'): void
  (e: 'run-tracker'): void
  (e: 'track-image'): void
  (e: 'show-info'): void
}>()

const { t } = useI18n()

const composeInfo = computed(() => getComposeInfo(props.container.labels))

function formatBytes(bytes: number | undefined): string {
  return formatBytesShared(bytes || 0)
}
</script>
