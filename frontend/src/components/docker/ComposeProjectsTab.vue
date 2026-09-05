<template>
  <!-- Filters -->
  <DataToolbar
    searchable
    :search="composeSearchInput"
    :search-placeholder="t('docker.searchProjectPlaceholder')"
    @update:search="composeSearchInput = $event"
  >
    <template #bottom>
      <div class="row g-3">
        <div class="col-6 col-md-6 col-lg-3">
          <select
            v-model="composeHostFilter"
            class="form-select"
          >
            <option value="">
              {{ t('docker.allHosts') }}
            </option>
            <option
              v-for="h in uniqueHosts"
              :key="h"
              :value="h"
            >
              {{ h }}
            </option>
          </select>
        </div>
        <div class="col-6 col-md-6 col-lg-3">
          <select
            v-model="composeStateFilter"
            class="form-select"
          >
            <option value="">
              {{ t('docker.allStates') }}
            </option>
            <option value="running">
              {{ t('docker.running') }}
            </option>
            <option value="stopped">
              {{ t('docker.stopped') }}
            </option>
          </select>
        </div>
      </div>
    </template>
  </DataToolbar>

  <div
    v-if="filteredComposeProjects.length > 0"
    class="card"
  >
    <div class="table-responsive scroll-table">
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>{{ t('docker.projectColumn') }}</th>
            <th>{{ t('docker.hostColumn') }}</th>
            <th>{{ t('docker.stateColumn') }}</th>
            <th>{{ t('docker.servicesColumn') }}</th>
            <th>{{ t('docker.configFileColumn') }}</th>
            <th />
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="p in filteredComposeProjects"
            :key="p.id"
          >
            <td class="fw-semibold">
              {{ p.name }}
            </td>
            <td>
              <router-link
                :to="`/hosts/${p.host_id}`"
                class="text-decoration-none"
              >
                {{ p.hostname }}
              </router-link>
            </td>
            <td>
              <span :class="getComposeStatus(p) === 'running' ? 'badge bg-success-lt text-success' : 'badge bg-secondary-lt text-secondary'">
                {{ getComposeStatus(p) === 'running' ? t('docker.running') : t('docker.stopped') }}
              </span>
              <span
                v-if="getComposeUpdates(p).length > 0"
                class="badge bg-warning-lt text-warning ms-1"
                :title="getComposeUpdates(p).map(v => t('docker.updateAvailableTooltipItem', { image: v.docker_image, version: v.latest_version })).join('\n')"
              >
                {{ t('docker.updatesCountBadge', { count: getComposeUpdates(p).length }) }}
              </span>
              <div
                v-if="getComposeUpdates(p).length > 0"
                class="d-flex flex-wrap gap-1 mt-1"
              >
                <template
                  v-for="vc in getComposeUpdates(p)"
                  :key="vc.tracker_id || `${p.id}:${vc.docker_image}`"
                >
                  <router-link
                    v-if="vc.tracker_id"
                    :to="`/release-trackers/${vc.tracker_id}`"
                    class="btn btn-sm btn-outline-secondary"
                    :title="t('docker.viewTracking')"
                  >
                    {{ vc.docker_image }}
                  </router-link>
                  <button
                    v-if="vc.tracker_id"
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-success"
                    :disabled="isTrackerRunDisabled(vc)"
                    :title="trackerRunTooltip(vc)"
                    :aria-label="t('docker.triggerTracker')"
                    @click="runTracker(vc, p)"
                  >
                    <span
                      v-if="trackerRunLoading[vc.tracker_id]"
                      class="spinner-border spinner-border-sm"
                    />
                    <IconPlayerPlay
                      v-else
                      :size="14"
                      class="icon"
                    />
                  </button>
                </template>
              </div>
            </td>
            <td>
              <div class="d-flex flex-wrap gap-1">
                <span
                  v-for="svc in p.services"
                  :key="svc"
                  class="badge bg-blue-lt text-blue"
                >{{ svc }}</span>
                <span
                  v-if="!p.services || p.services.length === 0"
                  class="text-secondary"
                >-</span>
              </div>
            </td>
            <td class="font-monospace small text-secondary">
              {{ p.config_file || p.working_dir || '-' }}
            </td>
            <td class="text-end">
              <div class="d-flex align-items-center justify-content-end gap-1">
                <template v-if="canRunDocker">
                  <button
                    v-if="getComposeStatus(p) === 'stopped'"
                    type="button"
                    :disabled="!!actionLoading[p.name]"
                    class="btn btn-icon btn-sm btn-ghost-success"
                    title="Start (up -d)"
                    :aria-label="t('docker.startProjectAriaLabel')"
                    @click="$emit('compose-action', { hostId: p.host_id, name: p.name, action: 'compose_up', workingDir: p.working_dir || '' })"
                  >
                    <span
                      v-if="actionLoading[p.name] === 'compose_up'"
                      class="spinner-border spinner-border-sm"
                    />
                    <IconPlayerPlay
                      v-else
                      :size="16"
                      class="icon icon-sm"
                    />
                  </button>
                  <template v-if="getComposeStatus(p) === 'running'">
                    <button
                      type="button"
                      :disabled="!!actionLoading[p.name]"
                      class="btn btn-icon btn-sm btn-ghost-danger"
                      title="Stop (down)"
                      :aria-label="t('docker.stopProjectTitle')"
                      @click="$emit('compose-action', { hostId: p.host_id, name: p.name, action: 'compose_down', workingDir: p.working_dir || '' })"
                    >
                      <span
                        v-if="actionLoading[p.name] === 'compose_down'"
                        class="spinner-border spinner-border-sm"
                      />
                      <IconPlayerStop
                        v-else
                        :size="16"
                        class="icon icon-sm"
                      />
                    </button>
                    <button
                      type="button"
                      :disabled="!!actionLoading[p.name]"
                      class="btn btn-icon btn-sm btn-ghost-warning"
                      :title="t('docker.verbRestart')"
                      :aria-label="t('docker.restartProjectTitle')"
                      @click="$emit('compose-action', { hostId: p.host_id, name: p.name, action: 'compose_restart', workingDir: p.working_dir || '' })"
                    >
                      <span
                        v-if="actionLoading[p.name] === 'compose_restart'"
                        class="spinner-border spinner-border-sm"
                      />
                      <IconRefresh
                        v-else
                        :size="16"
                        class="icon icon-sm"
                      />
                    </button>
                  </template>
                  <button
                    type="button"
                    :disabled="!!actionLoading[p.name]"
                    class="btn btn-icon btn-sm btn-ghost-secondary"
                    :title="t('docker.viewProjectLogsTooltip')"
                    :aria-label="t('docker.viewProjectLogsAriaLabel')"
                    @click="$emit('compose-action', { hostId: p.host_id, name: p.name, action: 'compose_logs', workingDir: p.working_dir || '' })"
                  >
                    <span
                      v-if="actionLoading[p.name] === 'compose_logs'"
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
                  :title="t('docker.configTooltip')"
                  @click="selectedProject = p"
                >
                  <IconFile
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div
      v-if="trackerFeedback"
      class="alert m-3 mt-0 py-2"
      :class="trackerFeedbackIsError ? 'alert-danger' : 'alert-success'"
      role="status"
    >
      {{ trackerFeedback }}
    </div>
  </div>

  <EmptyState
    v-if="filteredComposeProjects.length === 0"
    :title="composeSearch || composeHostFilter || composeStateFilter ? t('docker.noFilterResultsTitle') : t('docker.noComposeProjectsTitle')"
    :subtitle="composeSearch || composeHostFilter || composeStateFilter ? t('docker.noFilterResultsSubtitle') : t('docker.noComposeProjectsSubtitle')"
  />

  <!-- Modal projet compose (raw config) -->
  <div
    v-if="selectedProject"
    ref="modalRef"
    class="modal modal-blur fade show d-block"
    @click.self="selectedProject = null"
  >
    <div class="modal-dialog modal-xl modal-dialog-centered modal-dialog-scrollable">
      <div class="modal-content">
        <div class="modal-header">
          <div>
            <h5 class="modal-title">
              {{ selectedProject.name }}
            </h5>
            <div class="text-secondary small font-monospace mt-1">
              {{ selectedProject.config_file || selectedProject.working_dir || '-' }}
            </div>
          </div>
          <button
            type="button"
            class="btn-close"
            :aria-label="t('common.close')"
            @click="selectedProject = null"
          />
        </div>
        <div class="modal-body p-0">
          <div class="row g-0">
            <div class="col-md-3 border-end p-3">
              <div class="mb-3">
                <div class="text-secondary small fw-semibold text-uppercase mb-1">
                  {{ t('docker.hostColumn') }}
                </div>
                <div>{{ selectedProject.hostname }}</div>
              </div>
              <div class="mb-3">
                <div class="text-secondary small fw-semibold text-uppercase mb-1">
                  {{ t('docker.directoryLabel') }}
                </div>
                <div class="font-monospace small text-break">
                  {{ selectedProject.working_dir || '-' }}
                </div>
              </div>
              <div class="mb-3">
                <div class="text-secondary small fw-semibold text-uppercase mb-1">
                  {{ t('docker.fileLabel') }}
                </div>
                <div class="font-monospace small text-break">
                  {{ selectedProject.config_file || '-' }}
                </div>
              </div>
              <div>
                <div class="text-secondary small fw-semibold text-uppercase mb-1">
                  {{ t('docker.servicesWithCount', { count: (selectedProject.services || []).length }) }}
                </div>
                <div class="d-flex flex-wrap gap-1">
                  <span
                    v-for="svc in selectedProject.services"
                    :key="svc"
                    class="badge bg-blue-lt text-blue"
                  >{{ svc }}</span>
                  <span
                    v-if="!selectedProject.services || selectedProject.services.length === 0"
                    class="text-secondary small"
                  >-</span>
                </div>
              </div>
            </div>
            <div class="col-md-9">
              <div class="d-flex align-items-center justify-content-between px-3 pt-3 pb-2 border-bottom">
                <span class="text-secondary small fw-semibold">{{ t('docker.resolvedConfigLabel') }}</span>
                <button
                  type="button"
                  :class="['btn', 'btn-sm', copied ? 'btn-success' : 'btn-ghost-secondary']"
                  @click="copyConfig(selectedProject.raw_config)"
                >
                  {{ copied ? t('docker.copiedBadge') : t('docker.copy') }}
                </button>
              </div>
              <pre
                v-if="selectedProject.raw_config"
                class="m-0 p-3 small"
                style="max-height: 60vh; overflow-y: auto; background: var(--ss-panel-solid-darker); color: var(--ss-text-on-dark); border-radius: 0 0 4px 0;"
              >{{ selectedProject.raw_config }}</pre>
              <div
                v-else
                class="p-4 text-secondary text-center"
              >
                {{ t('docker.configNotAvailable') }}
              </div>
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <button
            type="button"
            class="btn"
            @click="selectedProject = null"
          >
            {{ t('common.close') }}
          </button>
        </div>
      </div>
    </div>
  </div>
  <div
    v-if="selectedProject"
    class="modal-backdrop fade show"
  />
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconFile, IconList, IconPlayerPlay, IconRefresh, IconPlayerStop } from '@tabler/icons-vue'
import apiClient from '../../api'
import DataToolbar from '../common/DataToolbar.vue'
import EmptyState from '../EmptyState.vue'
import { getApiErrorMessage } from '../../api/client'
import { useModalChrome } from '../../composables/useModalChrome'
import type { VersionComparisonStatus } from '../../types/docker'

interface ComposeProject {
  id: string | number
  name: string
  hostname?: string
  host_id: string
  config_file?: string
  working_dir?: string
  raw_config?: string
  services?: string[]
}

interface Container {
  id?: string
  host_id: string
  state?: string
  image: string
  image_tag?: string
  labels?: Record<string, string>
}

interface VersionComparison {
  tracker_id?: string
  host_id: string
  docker_image: string
  image_tag?: string
  running_version?: string
  latest_version?: string
  status?: VersionComparisonStatus
  is_up_to_date?: boolean
  update_confirmed?: boolean
}

const props = withDefaults(defineProps<{
  composeProjects?: ComposeProject[]
  containers?: Container[]
  versionComparisons?: VersionComparison[]
  canRunDocker?: boolean
  actionLoading?: Record<string, string | boolean>
}>(), {
  composeProjects: () => [],
  containers: () => [],
  versionComparisons: () => [],
  canRunDocker: false,
  actionLoading: () => ({}),
})

defineEmits<{
  (e: 'compose-action', ...args: unknown[]): void
}>()

const { t } = useI18n()

const composeSearchInput = ref('')
const composeSearch = ref('')
let composeSearchDebounce: ReturnType<typeof setTimeout> | null = null
watch(composeSearchInput, (val) => {
  if (composeSearchDebounce) clearTimeout(composeSearchDebounce)
  composeSearchDebounce = setTimeout(() => { composeSearch.value = val }, 300)
})
const composeHostFilter = ref('')
const composeStateFilter = ref('')
const selectedProject = ref<ComposeProject | null>(null)
const modalRef = ref<HTMLElement | null>(null)
useModalChrome(modalRef, () => !!selectedProject.value, { onClose: () => { selectedProject.value = null } })
const copied = ref(false)
const trackerRunLoading = ref<Record<string, boolean>>({})
const trackerFeedback = ref('')
const trackerFeedbackIsError = ref(false)

const composeProjectStatus = computed<Record<string, string>>(() => {
  const statusMap: Record<string, string> = {}
  for (const project of props.composeProjects) {
    const projectContainers = props.containers.filter(
      (c) => c.labels?.['com.docker.compose.project'] === project.name &&
           c.host_id === project.host_id
    )
    statusMap[`${project.host_id}:${project.name}`] =
      projectContainers.some((c) => c.state === 'running') ? 'running' : 'stopped'
  }
  return statusMap
})

function getComposeStatus(project: ComposeProject): string {
  return composeProjectStatus.value[`${project.host_id}:${project.name}`] || 'stopped'
}

// Same two-shaped index as the containers table: ambient rows are keyed with
// their exact tag, tracker rows (tag-agnostic) without one.
const vcByImage = computed<Record<string, VersionComparison>>(() => {
  const m: Record<string, VersionComparison> = {}
  for (const vc of props.versionComparisons) {
    if (vc.image_tag) m[`${vc.host_id}|${vc.docker_image}|${vc.image_tag}`] = vc
    else m[`${vc.host_id}|${vc.docker_image}`] = vc
  }
  return m
})

function getComposeUpdates(project: ComposeProject): VersionComparison[] {
  const projectContainers = props.containers.filter(
    (c) => c.labels?.['com.docker.compose.project'] === project.name && c.host_id === project.host_id
  )
  const updates: VersionComparison[] = []
  const seen = new Set<string>()
  for (const c of projectContainers) {
    const vc = vcByImage.value[`${c.host_id}|${c.image}|${c.image_tag || 'latest'}`] ||
               vcByImage.value[`${c.host_id}|${c.image}`] ||
               vcByImage.value[`${c.host_id}|${c.image}:${c.image_tag}`]
    if (vc && vc.status === 'update_available') {
      const key = vc.tracker_id || `${vc.host_id}|${vc.docker_image}|${vc.image_tag || ''}`
      if (!seen.has(key)) {
        seen.add(key)
        updates.push(vc)
      }
    }
  }
  return updates
}

const uniqueHosts = computed(() => {
  const seen = new Set<string>()
  return props.composeProjects
    .filter((p) => { if (!p.hostname || seen.has(p.hostname)) return false; seen.add(p.hostname); return true })
    .map((p) => p.hostname!)
    .sort()
})

const filteredComposeProjects = computed(() => {
  return props.composeProjects.filter((p) => {
    if (composeSearch.value) {
      const q = composeSearch.value.toLowerCase()
      const match = p.name?.toLowerCase().includes(q) ||
        p.hostname?.toLowerCase().includes(q) ||
        p.config_file?.toLowerCase().includes(q) ||
        p.working_dir?.toLowerCase().includes(q)
      if (!match) return false
    }
    if (composeHostFilter.value && p.hostname !== composeHostFilter.value) return false
    if (composeStateFilter.value && getComposeStatus(p) !== composeStateFilter.value) return false
    return true
  })
})

async function copyConfig(text: string | undefined): Promise<void> {
  if (!text) return
  await navigator.clipboard.writeText(text)
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}

function canRunTracker(vc: VersionComparison | undefined): boolean {
  return props.canRunDocker && !!vc?.tracker_id
}

function hasManualTrackerData(vc: VersionComparison | undefined): boolean {
  return !!(vc?.latest_version && String(vc.latest_version).trim())
}

function isTrackerRunDisabled(vc: VersionComparison): boolean {
  if (!canRunTracker(vc)) return true
  if (!hasManualTrackerData(vc)) return true
  return !!trackerRunLoading.value[vc.tracker_id!]
}

function trackerRunTooltip(vc: VersionComparison): string {
  if (!props.canRunDocker) return t('docker.adminOperatorOnly')
  if (!hasManualTrackerData(vc)) return t('docker.waitFirstCheck')
  return t('docker.triggerTrackerNow')
}

async function runTracker(vc: VersionComparison, project?: ComposeProject): Promise<void> {
  if (isTrackerRunDisabled(vc)) return
  const id = vc.tracker_id!
  trackerRunLoading.value = { ...trackerRunLoading.value, [id]: true }
  trackerFeedback.value = ''
  trackerFeedbackIsError.value = false
  try {
    await apiClient.runReleaseTracker(id)
    trackerFeedback.value = t('docker.triggerLaunchedFor', { name: project?.name || vc?.docker_image || t('docker.trackerFallbackName') })
  } catch (e: unknown) {
    trackerFeedback.value = getApiErrorMessage(e, t('docker.triggerFailed'))
    trackerFeedbackIsError.value = true
  } finally {
    const next = { ...trackerRunLoading.value }
    delete next[id]
    trackerRunLoading.value = next
  }
}
</script>
