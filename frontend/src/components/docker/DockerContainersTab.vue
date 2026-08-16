<template>
  <!-- Filters -->
  <DataToolbar
    searchable
    :search="searchInput"
    search-placeholder="Rechercher un conteneur…"
    @update:search="searchInput = $event"
  >
    <template #bottom>
      <div class="row g-3">
        <div class="col-6 col-md-6 col-lg-4">
          <select
            v-model="hostFilter"
            class="form-select"
          >
            <option value="">
              Tous les hôtes
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
        <div class="col-6 col-md-6 col-lg-4">
          <select
            v-model="stateFilter"
            class="form-select"
          >
            <option value="">
              Tous les états
            </option>
            <option value="running">
              En cours
            </option>
            <option value="restarting">
              Redémarrage
            </option>
            <option value="paused">
              En pause
            </option>
            <option value="created">
              Créé
            </option>
            <option value="exited">
              Arrêté
            </option>
            <option value="dead">
              Mort
            </option>
          </select>
        </div>
        <div class="col-12 col-md-12 col-lg-4">
          <select
            v-model="composeFilter"
            class="form-select"
          >
            <option value="">
              Tous les conteneurs
            </option>
            <option value="compose">
              Docker Compose
            </option>
            <option value="standalone">
              Standalone
            </option>
          </select>
        </div>
      </div>
    </template>
  </DataToolbar>

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
                  aria-label="Sélectionner tous les conteneurs affichés"
                  @change="toggleSelectAll(($event.target as HTMLInputElement).checked)"
                >
              </label>
            </th>
            <th>
              <SortableHeader
                label="Nom"
                :active="sortBy === 'name'"
                :direction="sortDir"
                @toggle="toggleSort('name')"
              />
            </th>
            <th>
              <SortableHeader
                label="Hôte"
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
                label="État"
                :active="sortBy === 'state'"
                :direction="sortDir"
                @toggle="toggleSort('state')"
              />
            </th>
            <th>Port interne</th>
            <th>Port hôte exposé</th>
            <th>Réseau (Rx / Tx)</th>
            <th />
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="c in pagedContainers"
            :key="c.id"
            :class="{ 'table-active': selectedIds.has(c.id) }"
          >
            <td v-if="canRunDocker">
              <label class="form-check mb-0">
                <input
                  class="form-check-input"
                  type="checkbox"
                  :checked="selectedIds.has(c.id)"
                  :aria-label="`Sélectionner ${c.name}`"
                  @change="toggleSelected(c.id, ($event.target as HTMLInputElement).checked)"
                >
              </label>
            </td>
            <td class="fw-semibold">
              {{ c.name }}
            </td>
            <td>
              <router-link
                :to="`/hosts/${c.host_id}`"
                class="text-decoration-none"
              >
                {{ c.hostname }}
              </router-link>
            </td>
            <td>
              <DockerComposeBadge :labels="c.labels" />
            </td>
            <td class="small">
              <div>{{ c.image }}</div>
              <div class="mt-1 d-flex align-items-center gap-1 flex-wrap">
                <code>{{ containerVersion(c)?.running_version || c.image_tag }}</code>
                <template v-if="containerVersion(c)">
                  <span
                    v-if="containerVersion(c)?.status === 'up_to_date'"
                    class="badge bg-success-lt text-success"
                  >À jour</span>
                  <span
                    v-else-if="containerVersion(c)?.status === 'update_available'"
                    class="badge bg-warning-lt text-warning"
                    :title="`Dernière version : ${containerVersion(c)?.latest_version}`"
                  >Mise à jour disponible</span>
                  <span
                    v-else
                    class="badge bg-secondary-lt text-secondary"
                    :title="unknownVersionTitle(containerVersion(c))"
                  >Version inconnue</span>
                </template>
              </div>
            </td>
            <td>
              <span :class="getEntityStateClass(c.state)">{{ getEntityStateLabel(c.state) }}</span>
            </td>
            <td class="small">
              <DockerPortBadges
                :ports="normalizedPortsForContainer(c)"
                kind="internal"
              />
            </td>
            <td class="small">
              <DockerPortBadges
                :ports="normalizedPortsForContainer(c)"
                kind="exposed"
              />
            </td>
            <td class="text-secondary small font-monospace">
              <template v-if="c.state === 'running' && ((c.net_rx_bytes ?? 0) > 0 || (c.net_tx_bytes ?? 0) > 0)">
                ↓ {{ formatBytes(c.net_rx_bytes) }} / ↑ {{ formatBytes(c.net_tx_bytes) }}
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
                    v-if="['exited', 'dead', 'created', 'paused'].includes(c.state)"
                    type="button"
                    :disabled="!!actionLoading[c.name]"
                    class="btn btn-icon btn-sm btn-ghost-success"
                    title="Démarrer"
                    aria-label="Démarrer le conteneur"
                    @click="$emit('container-action', { hostId: c.host_id, name: c.name, action: 'start', container: c })"
                  >
                    <span
                      v-if="actionLoading[c.name] === 'start'"
                      class="spinner-border spinner-border-sm"
                    />
                    <IconPlayerPlay
                      v-else
                      :size="16"
                      class="icon icon-sm"
                    />
                  </button>
                  <button
                    v-if="c.state === 'running'"
                    type="button"
                    :disabled="!!actionLoading[c.name]"
                    class="btn btn-icon btn-sm btn-ghost-danger"
                    title="Arrêter"
                    aria-label="Arrêter le conteneur"
                    @click="$emit('container-action', { hostId: c.host_id, name: c.name, action: 'stop', container: c })"
                  >
                    <span
                      v-if="actionLoading[c.name] === 'stop'"
                      class="spinner-border spinner-border-sm"
                    />
                    <IconPlayerStop
                      v-else
                      :size="16"
                      class="icon icon-sm"
                    />
                  </button>
                  <button
                    v-if="c.state === 'running'"
                    type="button"
                    :disabled="!!actionLoading[c.name]"
                    class="btn btn-icon btn-sm btn-ghost-warning"
                    title="Redémarrer"
                    aria-label="Redémarrer le conteneur"
                    @click="$emit('container-action', { hostId: c.host_id, name: c.name, action: 'restart', container: c })"
                  >
                    <span
                      v-if="actionLoading[c.name] === 'restart'"
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
                    :disabled="!!actionLoading[c.name]"
                    class="btn btn-icon btn-sm btn-ghost-secondary"
                    title="Voir les logs"
                    aria-label="Voir les logs du conteneur"
                    @click="$emit('container-action', { hostId: c.host_id, name: c.name, action: 'logs', container: c })"
                  >
                    <span
                      v-if="actionLoading[c.name] === 'logs'"
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
                  title="Inspecter"
                  aria-label="Inspecter le conteneur"
                  @click="inspectTarget = c; inspectTab = 'env'"
                >
                  <IconSearch
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
                <button
                  v-if="containerVersion(c)?.tracker_id"
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-secondary"
                  title="Voir le suivi de version"
                  aria-label="Voir le suivi de version"
                  @click="openTracker(containerVersion(c)?.tracker_id)"
                >
                  <IconChevronRight
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
                <button
                  v-if="containerVersion(c)?.tracker_id"
                  type="button"
                  :disabled="isTrackerRunDisabled(containerVersion(c))"
                  class="btn btn-icon btn-sm btn-ghost-success"
                  :title="trackerRunTooltip(containerVersion(c))"
                  aria-label="Déclencher le tracker"
                  @click="runTracker(containerVersion(c), c)"
                >
                  <span
                    v-if="trackerRunLoading[containerVersion(c)?.tracker_id || '']"
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
                  title="Suivre les mises à jour de cette image"
                  aria-label="Créer un tracker de mise à jour"
                  @click="trackImage(c)"
                >
                  <IconActivity
                    :size="16"
                    class="icon icon-sm"
                  />
                </button>
                <button
                  v-if="getComposeInfo(c).project || Object.keys(c.labels || {}).length > 0"
                  type="button"
                  class="btn btn-sm btn-ghost-secondary"
                  :title="getComposeInfo(c).project ? 'Infos Compose + Labels' : 'Labels'"
                  @click="selectedContainer = c"
                >
                  <IconClipboard
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
      v-if="totalPages > 1"
      class="card-footer d-flex align-items-center"
    >
      <p class="m-0 text-muted small">
        {{ (currentPage - 1) * PAGE_SIZE + 1 }}–{{ Math.min(currentPage * PAGE_SIZE, sortedContainers.length) }} sur {{ sortedContainers.length }} conteneur{{ sortedContainers.length > 1 ? 's' : '' }}
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
    @clear="selectedIds = new Set()"
  >
    <button
      type="button"
      class="btn btn-sm btn-success"
      :disabled="bulkActionLoading"
      @click="emitBulkAction('start')"
    >
      Démarrer
    </button>
    <button
      type="button"
      class="btn btn-sm btn-outline-danger"
      :disabled="bulkActionLoading"
      @click="emitBulkAction('stop')"
    >
      Arrêter
    </button>
    <button
      type="button"
      class="btn btn-sm btn-outline-warning"
      :disabled="bulkActionLoading"
      @click="emitBulkAction('restart')"
    >
      Redémarrer
    </button>
  </BulkActionBar>

  <EmptyState
    v-if="sortedContainers.length === 0"
    :title="search || hostFilter || stateFilter || composeFilter ? 'Aucun résultat pour ces filtres' : 'Aucun conteneur trouvé'"
    :subtitle="search || hostFilter || stateFilter || composeFilter ? 'Modifiez vos critères de recherche' : 'Connectez un hôte avec l\'agent Docker activé pour voir vos conteneurs ici'"
    :cta-label="!search && !hostFilter && !stateFilter && !composeFilter ? 'Ajouter un hôte' : ''"
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

  <!-- Modal conteneur (labels/compose info) -->
  <div
    v-if="selectedContainer"
    ref="containerModalRef"
    class="modal modal-blur fade show d-block"
    @click.self="selectedContainer = null"
  >
    <div class="modal-dialog modal-lg modal-dialog-centered">
      <div class="modal-content">
        <div class="modal-header">
          <h5 class="modal-title">
            Détails Docker Compose
          </h5>
          <button
            type="button"
            class="btn-close"
            aria-label="Fermer"
            @click="selectedContainer = null"
          />
        </div>
        <div class="modal-body">
          <div class="mb-3">
            <label class="form-label fw-semibold">Conteneur</label>
            <div>{{ selectedContainer.name }}</div>
          </div>
          <div class="mb-3">
            <label class="form-label fw-semibold">Projet Compose</label>
            <div>{{ getComposeInfo(selectedContainer).project || '-' }}</div>
          </div>
          <div class="mb-3">
            <label class="form-label fw-semibold">Service</label>
            <div>{{ getComposeInfo(selectedContainer).service || '-' }}</div>
          </div>
          <div class="mb-3">
            <label class="form-label fw-semibold">Répertoire de travail</label>
            <div class="font-monospace small">
              {{ getComposeInfo(selectedContainer).workingDir || '-' }}
            </div>
          </div>
          <div class="mb-3">
            <label class="form-label fw-semibold">Fichiers de configuration</label>
            <div class="font-monospace small">
              {{ getComposeInfo(selectedContainer).configFiles || '-' }}
            </div>
          </div>
          <div
            v-if="Object.keys(selectedContainer.labels || {}).length > 0"
            class="mb-3"
          >
            <label class="form-label fw-semibold">Labels</label>
            <div
              class="border rounded p-2 bg-dark small font-monospace"
              style="max-height: 200px; overflow-y: auto;"
            >
              <div
                v-for="(value, key) in selectedContainer.labels"
                :key="key"
                class="mb-1"
              >
                <span class="text-muted">{{ key }}:</span> <span class="text-light">{{ value }}</span>
              </div>
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <button
            type="button"
            class="btn"
            @click="selectedContainer = null"
          >
            Fermer
          </button>
        </div>
      </div>
    </div>
  </div>
  <div
    v-if="selectedContainer"
    class="modal-backdrop fade show"
  />

  <!-- Modal Inspection (env vars / volumes / networks) -->
  <div
    v-if="inspectTarget"
    ref="inspectModalRef"
    class="modal modal-blur fade show d-block"
    @click.self="inspectTarget = null"
  >
    <div class="modal-dialog modal-lg modal-dialog-centered">
      <div class="modal-content">
        <div class="modal-header">
          <div>
            <h5 class="modal-title">
              {{ inspectTarget.name }}
            </h5>
            <div class="text-secondary small">
              {{ inspectTarget.image }}:<code>{{ containerVersion(inspectTarget)?.running_version || inspectTarget.image_tag }}</code>
              <span
                class="ms-2"
                :class="getEntityStateClass(inspectTarget.state)"
              >{{ getEntityStateLabel(inspectTarget.state) }}</span>
            </div>
          </div>
          <button
            type="button"
            class="btn-close"
            aria-label="Fermer"
            @click="inspectTarget = null"
          />
        </div>
        <div class="modal-body p-0">
          <div class="border-bottom px-3">
            <ul class="nav nav-tabs nav-tabs-alt">
              <li class="nav-item">
                <a
                  class="nav-link"
                  :class="{ active: inspectTab === 'env' }"
                  href="#"
                  @click.prevent="inspectTab = 'env'"
                >
                  Env Vars
                  <span class="badge bg-azure-lt text-azure ms-1">{{ Object.keys(inspectTarget.env_vars || {}).length }}</span>
                </a>
              </li>
              <li class="nav-item">
                <a
                  class="nav-link"
                  :class="{ active: inspectTab === 'volumes' }"
                  href="#"
                  @click.prevent="inspectTab = 'volumes'"
                >
                  Volumes
                  <span class="badge bg-azure-lt text-azure ms-1">{{ (inspectTarget.volumes || []).length }}</span>
                </a>
              </li>
              <li class="nav-item">
                <a
                  class="nav-link"
                  :class="{ active: inspectTab === 'networks' }"
                  href="#"
                  @click.prevent="inspectTab = 'networks'"
                >
                  Réseaux
                  <span class="badge bg-azure-lt text-azure ms-1">{{ (inspectTarget.networks || []).length }}</span>
                </a>
              </li>
            </ul>
          </div>
          <div
            class="p-3"
            style="min-height: 200px; max-height: 400px; overflow-y: auto;"
          >
            <div v-if="inspectTab === 'env'">
              <div
                v-if="Object.keys(inspectTarget.env_vars || {}).length === 0"
                class="text-secondary text-center py-3"
              >
                Aucune variable d'environnement (non sensible) disponible
              </div>
              <table
                v-else
                class="table table-sm table-vcenter"
              >
                <thead><tr><th>Variable</th><th>Valeur</th></tr></thead>
                <tbody>
                  <tr
                    v-for="(val, key) in inspectTarget.env_vars"
                    :key="key"
                  >
                    <td class="font-monospace small fw-semibold">
                      {{ key }}
                    </td>
                    <td
                      class="font-monospace small text-secondary"
                      style="word-break: break-all;"
                    >
                      {{ val }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-if="inspectTab === 'volumes'">
              <div
                v-if="!(inspectTarget.volumes || []).length"
                class="text-secondary text-center py-3"
              >
                Aucun volume monté
              </div>
              <ul
                v-else
                class="list-unstyled mb-0"
              >
                <li
                  v-for="vol in inspectTarget.volumes"
                  :key="vol"
                  class="py-1 border-bottom font-monospace small"
                >
                  {{ vol }}
                </li>
              </ul>
            </div>
            <div v-if="inspectTab === 'networks'">
              <div
                v-if="!(inspectTarget.networks || []).length"
                class="text-secondary text-center py-3"
              >
                Aucun réseau connecté
              </div>
              <div
                v-else
                class="d-flex flex-wrap gap-2 pt-1"
              >
                <span
                  v-for="net in inspectTarget.networks"
                  :key="net"
                  class="badge bg-blue-lt text-blue fs-6"
                >{{ net }}</span>
              </div>
              <div
                v-if="(inspectTarget?.net_rx_bytes ?? 0) > 0 || (inspectTarget?.net_tx_bytes ?? 0) > 0"
                class="mt-3 border-top pt-3"
              >
                <div class="text-secondary small fw-semibold mb-1">
                  I/O réseau (cumulatif)
                </div>
                <div class="row row-sm">
                  <div class="col-6">
                    <div class="text-muted small">
                      ↓ Reçu
                    </div>
                    <div class="fw-semibold text-info">
                      {{ formatBytes(inspectTarget.net_rx_bytes) }}
                    </div>
                  </div>
                  <div class="col-6">
                    <div class="text-muted small">
                      ↑ Envoyé
                    </div>
                    <div class="fw-semibold text-warning">
                      {{ formatBytes(inspectTarget.net_tx_bytes) }}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <button
            type="button"
            class="btn"
            @click="inspectTarget = null"
          >
            Fermer
          </button>
        </div>
      </div>
    </div>
  </div>
  <div
    v-if="inspectTarget"
    class="modal-backdrop fade show"
  />
</template>

<script setup lang="ts">
import { ref, computed, watch, toRef } from 'vue'
import { IconActivity, IconBox, IconChevronRight, IconClipboard, IconList, IconPlayerPlay, IconRefresh, IconSearch, IconPlayerStop } from '@tabler/icons-vue'
import { useRouter } from 'vue-router'
import apiClient from '../../api'
import DataToolbar from '../common/DataToolbar.vue'
import SortableHeader from '../common/SortableHeader.vue'
import DockerPortBadges from '../common/DockerPortBadges.vue'
import DockerComposeBadge from './DockerComposeBadge.vue'
import EmptyState from '../EmptyState.vue'
import PaginationNav from '../PaginationNav.vue'
import BulkActionBar from '../BulkActionBar.vue'
import { useModalChrome } from '../../composables/useModalChrome'
import { useDockerContainerPorts } from '../../composables/useDockerContainerPorts'
import { usePagination } from '../../composables/usePagination'
import { getApiErrorMessage } from '../../api/client'
import { getEntityStateClass, getEntityStateLabel } from '../../utils/statusClasses'
import type { VersionComparisonStatus } from '../../types/docker'
import {
  getComposeInfo as getComposeInfoFromLabels,
  isComposeContainer as isComposeContainerFromLabels,
} from '../../utils/dockerCompose'

interface Container {
  id: string
  name: string
  hostname?: string
  host_id: string
  image: string
  image_tag?: string
  state: string
  labels?: Record<string, string>
  env_vars?: Record<string, string>
  volumes?: string[]
  networks?: string[]
  net_rx_bytes?: number
  net_tx_bytes?: number
  [key: string]: any
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
  last_error?: string
}

const router = useRouter()

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

const { normalizedPortsForContainer } = useDockerContainerPorts(toRef(props, 'containers') as any)

// Two kinds of rows land here: ambient ones (one per host+image+tag group,
// carrying image_tag) and tracker ones (aggregated across tags, image_tag
// empty). Indexing both under distinct keys keeps the exact-tag ambient row
// preferred while the tracker row stays reachable as the fallback — the server
// never emits both for the same container group.
const versionMap = computed<Record<string, VersionComparison>>(() => {
  const m: Record<string, VersionComparison> = {}
  for (const vc of props.versionComparisons) {
    if (vc.image_tag) m[`${vc.host_id}|${vc.docker_image}|${vc.image_tag}`] = vc
    else m[`${vc.host_id}|${vc.docker_image}`] = vc
  }
  return m
})

function containerVersion(c: Container): VersionComparison | null {
  return versionMap.value[`${c.host_id}|${c.image}|${c.image_tag || 'latest'}`] ||
         versionMap.value[`${c.host_id}|${c.image}`] ||
         versionMap.value[`${c.host_id}|${c.image}:${c.image_tag}`] ||
         null
}

const emit = defineEmits<{
  (e: 'container-action', ...args: unknown[]): void
  (e: 'bulk-container-action', containers: Container[], action: string): void
}>()

const selectedIds = ref<Set<string>>(new Set())

function toggleSelected(id: string, checked: boolean): void {
  const next = new Set(selectedIds.value)
  if (checked) next.add(id)
  else next.delete(id)
  selectedIds.value = next
}

// Scoped to the current page, matching the checkbox's own aria-label
// ("...affichés") — selecting "all" used to silently span every page of the
// current filter, which made a bulk stop/restart much wider than what the
// screen showed.
const allVisibleSelected = computed(() =>
  pagedContainers.value.length > 0 && pagedContainers.value.every((c) => selectedIds.value.has(c.id))
)

function toggleSelectAll(checked: boolean): void {
  const next = new Set(selectedIds.value)
  for (const c of pagedContainers.value) {
    if (checked) next.add(c.id)
    else next.delete(c.id)
  }
  selectedIds.value = next
}

function emitBulkAction(action: string): void {
  const selected = sortedContainers.value.filter((c) => selectedIds.value.has(c.id))
  if (!selected.length) return
  emit('bulk-container-action', selected, action)
  selectedIds.value = new Set()
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

const searchInput = ref('')
const search = ref('')
let searchDebounce: ReturnType<typeof setTimeout> | null = null
watch(searchInput, (val) => {
  if (searchDebounce) clearTimeout(searchDebounce)
  searchDebounce = setTimeout(() => { search.value = val }, 300)
})
const stateFilter = ref('')
const hostFilter = ref('')
const composeFilter = ref('')
const sortBy = ref<keyof Container>('hostname')
const sortDir = ref<'asc' | 'desc'>('asc')
const inspectTarget = ref<Container | null>(null)
const inspectTab = ref('env')
const selectedContainer = ref<Container | null>(null)
const containerModalRef = ref<HTMLElement | null>(null)
const inspectModalRef = ref<HTMLElement | null>(null)
useModalChrome(containerModalRef, () => !!selectedContainer.value, { onClose: () => { selectedContainer.value = null } })
useModalChrome(inspectModalRef, () => !!inspectTarget.value, { onClose: () => { inspectTarget.value = null } })
const trackerRunLoading = ref<Record<string, boolean>>({})
const trackerFeedback = ref('')
const trackerFeedbackIsError = ref(false)

function toggleSort(key: keyof Container): void {
  if (sortBy.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
    return
  }
  sortBy.value = key
  sortDir.value = 'asc'
}

function getComposeInfo(container: Container) {
  return getComposeInfoFromLabels(container.labels)
}

function isComposeContainer(container: Container): boolean {
  return isComposeContainerFromLabels(container.labels)
}

// Groups states by operational severity rather than sorting alphabetically
// (which interleaved "created"/"dead"/"exited" with no meaningful order).
const STATE_RANK: Record<string, number> = {
  running: 0,
  restarting: 1,
  paused: 1,
  created: 2,
  exited: 3,
  dead: 4,
  removing: 4,
}

function stateRank(state: string | undefined): number {
  return STATE_RANK[state || ''] ?? 5
}

function formatBytes(bytes: number | undefined): string {
  if (!bytes) return '0 B'
  const k = 1024
  const sizes = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function openTracker(trackerId: string | undefined): void {
  if (!trackerId) return
  router.push(`/release-trackers/${trackerId}`)
}

// The ambient engine records why it couldn't classify an image (private
// registry with no matching credential, registry unreachable, not swept yet) —
// surface it rather than leaving "Version inconnue" unexplained.
function unknownVersionTitle(vc: VersionComparison | null | undefined): string {
  return vc?.last_error || "Aucune information de version disponible pour cette image (registre non interrogeable ou pas encore vérifié)."
}

function canRunTracker(vc: VersionComparison | null | undefined): boolean {
  return props.canRunDocker && !!vc?.tracker_id
}

function hasManualTrackerData(vc: VersionComparison | null | undefined): boolean {
  return !!(vc?.latest_version && String(vc.latest_version).trim())
}

function isTrackerRunDisabled(vc: VersionComparison | null | undefined): boolean {
  if (!vc) return true
  if (!canRunTracker(vc)) return true
  if (!hasManualTrackerData(vc)) return true
  return !!trackerRunLoading.value[vc.tracker_id!]
}

function trackerRunTooltip(vc: VersionComparison | null | undefined): string {
  if (!props.canRunDocker) return 'Action réservée admin/opérateur'
  if (!hasManualTrackerData(vc)) return 'Attendez la première vérification automatique'
  return 'Déclencher la tâche du tracker maintenant'
}

async function runTracker(vc: VersionComparison | null | undefined, container?: Container): Promise<void> {
  if (!vc || isTrackerRunDisabled(vc)) return
  const id = vc.tracker_id!
  trackerRunLoading.value = { ...trackerRunLoading.value, [id]: true }
  trackerFeedback.value = ''
  trackerFeedbackIsError.value = false
  try {
    await apiClient.runReleaseTracker(id)
    trackerFeedback.value = `Déclenchement lancé pour ${container?.image || 'le tracker'}.`
  } catch (e: unknown) {
    trackerFeedback.value = getApiErrorMessage(e, 'Échec du déclenchement manuel.')
    trackerFeedbackIsError.value = true
  } finally {
    const next = { ...trackerRunLoading.value }
    delete next[id]
    trackerRunLoading.value = next
  }
}

const filteredContainers = computed(() => {
  return props.containers.filter((c) => {
    const matchSearch = !search.value ||
      c.name?.toLowerCase().includes(search.value.toLowerCase()) ||
      c.image?.toLowerCase().includes(search.value.toLowerCase()) ||
      getComposeInfo(c).project?.toLowerCase().includes(search.value.toLowerCase())
    const matchState = !stateFilter.value || c.state === stateFilter.value
    const matchCompose = !composeFilter.value ||
      (composeFilter.value === 'compose' && isComposeContainer(c)) ||
      (composeFilter.value === 'standalone' && !isComposeContainer(c))
    const matchHost = !hostFilter.value || c.hostname === hostFilter.value
    return matchSearch && matchState && matchCompose && matchHost
  })
})

const sortedContainers = computed(() => {
  const dir = sortDir.value === 'asc' ? 1 : -1
  const sorted = [...filteredContainers.value]
  sorted.sort((a, b) => {
    if (sortBy.value === 'state') {
      return (stateRank(a.state) - stateRank(b.state)) * dir
    }

    let av: unknown = a[sortBy.value]
    let bv: unknown = b[sortBy.value]

    if (sortBy.value === 'image') {
      av = `${a.image || ''}:${a.image_tag || ''}`
      bv = `${b.image || ''}:${b.image_tag || ''}`
    }

    const aVal = String(av || '').toLowerCase()
    const bVal = String(bv || '').toLowerCase()
    return aVal.localeCompare(bVal) * dir
  })
  return sorted
})

const uniqueHosts = computed(() => {
  const seen = new Set<string>()
  return props.containers
    .filter((c) => { if (!c.hostname || seen.has(c.hostname)) return false; seen.add(c.hostname); return true })
    .map((c) => c.hostname!)
    .sort()
})

const PAGE_SIZE = 25
const { currentPage, totalPages, pagedItems: pagedContainers, resetPage, setPage } = usePagination({
  items: sortedContainers,
  pageSize: PAGE_SIZE,
})

watch([search, stateFilter, hostFilter, composeFilter], () => {
  resetPage()
  selectedIds.value = new Set()
})
</script>

<style scoped>
.docker-select-col {
  width: 1%;
}
</style>


