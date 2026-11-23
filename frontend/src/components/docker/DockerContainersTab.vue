<template>
  <DockerContainersToolbar
    v-model:host-filter="hostFilter"
    v-model:state-filter="stateFilter"
    v-model:compose-filter="composeFilter"
    :search="searchInput"
    :unique-hosts="uniqueHosts"
    @update:search="searchInput = $event"
  />

  <div
    v-if="sortedContainers.length > 0"
    class="card"
  >
    <div
      v-if="trackerFeedback"
      class="alert m-3 mb-0 py-2"
      :class="trackerFeedbackIsError ? 'alert-danger' : 'alert-success'"
      role="status"
    >
      {{ trackerFeedback }}
    </div>
    <div class="table-responsive">
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th
              v-if="canRunDocker"
              class="docker-select-col"
            >
              <label class="form-check mb-0">
                <input
                  class="form-check-input"
                  type="checkbox"
                  :checked="allVisibleSelected"
                  :aria-label="t('docker.selectAllVisibleAriaLabel')"
                  @change="toggleSelectAll(($event.target as HTMLInputElement).checked)"
                >
              </label>
            </th>
            <th>
              <SortableHeader
                :label="t('docker.nameColumn')"
                :active="sortBy === 'name'"
                :direction="sortDir"
                @toggle="toggleSort('name')"
              />
            </th>
            <th>
              <SortableHeader
                :label="t('docker.hostColumn')"
                :active="sortBy === 'hostname'"
                :direction="sortDir"
                @toggle="toggleSort('hostname')"
              />
            </th>
            <th>Compose</th>
            <th>
              <SortableHeader
                label="Image"
                :active="sortBy === 'image'"
                :direction="sortDir"
                @toggle="toggleSort('image')"
              />
            </th>
            <th>
              <SortableHeader
                :label="t('docker.stateColumn')"
                :active="sortBy === 'state'"
                :direction="sortDir"
                @toggle="toggleSort('state')"
              />
            </th>
            <th>{{ t('docker.internalPortColumn') }}</th>
            <th>{{ t('docker.exposedPortColumn') }}</th>
            <th>{{ t('docker.networkRxTxColumn') }}</th>
            <th />
          </tr>
        </thead>
        <tbody>
          <DockerContainerRow
            v-for="c in pagedContainers"
            :key="c.id"
            :container="c"
            :version="containerVersion(c)"
            :ports="normalizedPortsForContainer(c)"
            :selected="selectedIds.has(c.id)"
            :can-run-docker="canRunDocker"
            :action-loading="actionLoading[c.name]"
            :tracker-run-loading="!!trackerRunLoading[containerVersion(c)?.tracker_id || '']"
            :tracker-run-disabled="isTrackerRunDisabled(containerVersion(c))"
            :tracker-run-tooltip="trackerRunTooltip(containerVersion(c))"
            :unknown-version-title="unknownVersionTitle(containerVersion(c))"
            @toggle-select="toggleSelected(c.id, $event)"
            @action="emit('container-action', { hostId: c.host_id, name: c.name, action: $event, container: c })"
            @inspect="inspectTarget = c"
            @open-tracker="openTracker(containerVersion(c)?.tracker_id)"
            @run-tracker="runTracker(containerVersion(c), c)"
            @track-image="trackImage(c)"
            @show-info="selectedContainer = c"
          />
        </tbody>
      </table>
    </div>
    <div
      v-if="totalPages > 1"
      class="card-footer d-flex align-items-center"
    >
      <p class="m-0 text-muted small">
        {{ t('docker.paginationSummary', { from: (currentPage - 1) * PAGE_SIZE + 1, to: Math.min(currentPage * PAGE_SIZE, sortedContainers.length), total: sortedContainers.length }, sortedContainers.length) }}
      </p>
      <PaginationNav
        class="ms-auto"
        :current-page="currentPage"
        :total-pages="totalPages"
        @select="setPage"
      />
    </div>
  </div>

  <BulkActionBar
    :count="selectedIds.size"
    @clear="clearSelection"
  >
    <button
      type="button"
      class="btn btn-sm btn-success"
      :disabled="bulkActionLoading"
      @click="emitBulkAction('start')"
    >
      {{ t('docker.verbStart') }}
    </button>
    <button
      type="button"
      class="btn btn-sm btn-outline-danger"
      :disabled="bulkActionLoading"
      @click="emitBulkAction('stop')"
    >
      {{ t('docker.verbStop') }}
    </button>
    <button
      type="button"
      class="btn btn-sm btn-outline-warning"
      :disabled="bulkActionLoading"
      @click="emitBulkAction('restart')"
    >
      {{ t('docker.verbRestart') }}
    </button>
  </BulkActionBar>

  <EmptyState
    v-if="sortedContainers.length === 0"
    :title="search || hostFilter || stateFilter || composeFilter ? t('docker.noFilterResultsTitle') : t('docker.noContainersFoundTitle')"
    :subtitle="search || hostFilter || stateFilter || composeFilter ? t('docker.noFilterResultsSubtitle') : t('docker.noContainersSubtitle')"
    :cta-label="!search && !hostFilter && !stateFilter && !composeFilter ? t('docker.addHostCta') : ''"
    cta-to="/hosts/new"
  >
    <template #icon>
      <IconBox
        :size="48"
        class="mb-3"
        :stroke-width="1.5"
      />
    </template>
  </EmptyState>

  <DockerContainerComposeModal
    v-if="selectedContainer"
    :container="selectedContainer"
    @close="selectedContainer = null"
  />

  <DockerContainerInspectModal
    v-if="inspectTarget"
    :container="inspectTarget"
    :version="containerVersion(inspectTarget)"
    @close="inspectTarget = null"
  />
</template>

<script setup lang="ts">
import { ref, toRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconBox } from '@tabler/icons-vue'
import { useRouter } from 'vue-router'
import SortableHeader from '../common/SortableHeader.vue'
import EmptyState from '../EmptyState.vue'
import PaginationNav from '../PaginationNav.vue'
import BulkActionBar from '../BulkActionBar.vue'
import DockerContainersToolbar from './DockerContainersToolbar.vue'
import DockerContainerRow from './DockerContainerRow.vue'
import DockerContainerComposeModal from './DockerContainerComposeModal.vue'
import DockerContainerInspectModal from './DockerContainerInspectModal.vue'
import { useDockerContainerPorts } from '../../composables/useDockerContainerPorts'
import { useDockerContainerVersions } from '../../composables/useDockerContainerVersions'
import { useDockerContainerList, PAGE_SIZE } from '../../composables/useDockerContainerList'
import type { Container, VersionComparison } from '../../types/dockerContainersTab'

const router = useRouter()
const { t } = useI18n()

const props = withDefaults(defineProps<{
  containers?: Container[]
  versionComparisons?: VersionComparison[]
  canRunDocker?: boolean
  actionLoading?: Record<string, string | boolean>
  bulkActionLoading?: boolean
}>(), {
  containers: () => [],
  versionComparisons: () => [],
  canRunDocker: false,
  actionLoading: () => ({}),
  bulkActionLoading: false,
})

const emit = defineEmits<{
  (e: 'container-action', ...args: unknown[]): void
  (e: 'bulk-container-action', containers: Container[], action: string): void
}>()

const { normalizedPortsForContainer } = useDockerContainerPorts(toRef(props, 'containers') as any)

const {
  containerVersion,
  trackerRunLoading,
  trackerFeedback,
  trackerFeedbackIsError,
  unknownVersionTitle,
  isTrackerRunDisabled,
  trackerRunTooltip,
  runTracker,
} = useDockerContainerVersions(toRef(props, 'versionComparisons'), () => props.canRunDocker)

const {
  searchInput,
  search,
  stateFilter,
  hostFilter,
  composeFilter,
  sortBy,
  sortDir,
  toggleSort,
  sortedContainers,
  uniqueHosts,
  currentPage,
  totalPages,
  pagedContainers,
  setPage,
  selectedIds,
  toggleSelected,
  allVisibleSelected,
  toggleSelectAll,
  clearSelection,
  selectedContainers,
} = useDockerContainerList(toRef(props, 'containers'))

const inspectTarget = ref<Container | null>(null)
const selectedContainer = ref<Container | null>(null)

function emitBulkAction(action: string): void {
  const selected = selectedContainers()
  if (!selected.length) return
  emit('bulk-container-action', selected, action)
  clearSelection()
}

function trackImage(c: Container): void {
  const image = c.image || ''
  const tag = c.image_tag || 'latest'
  const query: Record<string, string> = { tab: 'trackers', docker_image: image, docker_tag: tag }
  const project = c.labels?.['com.docker.compose.project']
  if (project) {
    query.compose_project = project
    const service = c.labels?.['com.docker.compose.service']
    if (service) query.compose_service = service
  }
  router.push({ path: '/git-webhooks', query })
}

function openTracker(trackerId: string | undefined): void {
  if (!trackerId) return
  router.push(`/release-trackers/${trackerId}`)
}
</script>

<style scoped>
.docker-select-col {
  width: 1%;
}
</style>


