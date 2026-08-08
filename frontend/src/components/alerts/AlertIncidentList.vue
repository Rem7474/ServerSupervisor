<template>
  <div class="card">
    <div class="card-header d-flex flex-column flex-lg-row align-items-start align-items-lg-center justify-content-between gap-3">
      <div>
        <h3 class="card-title mb-1">
          Historique de notifications
        </h3>
        <div class="text-muted small">
          Recherche, filtre par type ou par état, puis ouvre le détail en un clic.
        </div>
      </div>
      <div class="d-flex flex-wrap align-items-center gap-2">
        <BadgePill
          v-if="activeIncidentCount > 0"
          :text="`${activeIncidentCount} actif${activeIncidentCount > 1 ? 's' : ''}`"
          tone="danger"
          compact
        />
        <BadgePill
          :text="incidentCountLabel"
          tone="secondary"
          compact
        />
      </div>
    </div>

    <div class="card-body border-bottom py-3">
      <div class="row g-3 align-items-end">
        <div class="col-12 col-xl-4">
          <label class="form-label text-muted small mb-2">
            Recherche
          </label>
          <div class="input-icon">
            <span class="input-icon-addon">
              <IconSearch
                :size="16"
                class="icon"
              />
            </span>
            <input
              v-model="searchQuery"
              type="text"
              class="form-control"
              placeholder="Rechercher une règle, un hôte, une source…"
            >
            <button
              v-if="searchQuery"
              class="btn btn-icon btn-outline-secondary"
              type="button"
              aria-label="Effacer la recherche"
              @click="clearSearch"
            >
              <IconX
                :size="16"
                class="icon"
              />
            </button>
          </div>
        </div>
        <div class="col-12 col-xl-8">
          <div class="d-flex flex-wrap align-items-center justify-content-xl-end gap-3">
            <div class="d-flex flex-wrap align-items-center gap-2">
              <span class="text-muted small me-1 fw-semibold">Type :</span>
              <button
                v-for="opt in TYPE_FILTERS"
                :key="opt.value"
                type="button"
                class="btn btn-sm rounded-pill"
                :class="filterType === opt.value ? opt.activeClass : 'btn-ghost-secondary'"
                @click="setTypeFilter(opt.value)"
              >
                {{ opt.label }}
              </button>
            </div>
            <div
              class="vr opacity-50"
              style="height: 24px;"
            />
            <div class="d-flex flex-wrap align-items-center gap-2">
              <span class="text-muted small me-1 fw-semibold">État :</span>
              <button
                v-for="opt in STATUS_FILTERS"
                :key="opt.value"
                type="button"
                class="btn btn-sm rounded-pill"
                :class="filterStatus === opt.value ? opt.activeClass : 'btn-ghost-secondary'"
                @click="setStatusFilter(opt.value)"
              >
                {{ opt.label }}
              </button>
            </div>
          </div>
        </div>
        <div class="col-12 d-flex flex-wrap align-items-center gap-2">
          <span class="text-muted small me-1">
            Actions rapides
          </span>
          <button
            type="button"
            class="btn btn-sm btn-ghost-secondary"
            :disabled="markingRead"
            @click="$emit('mark-all-read')"
          >
            <IconCheck
              :size="16"
              class="icon me-1"
            />
            Tout marquer lu
          </button>
          <button
            type="button"
            class="btn btn-sm btn-ghost-secondary"
            :title="groupByHost ? 'Afficher en liste chronologique' : 'Regrouper par hôte'"
            @click="groupByHost = !groupByHost"
          >
            <IconStack2
              v-if="!groupByHost"
              :size="14"
              class="icon me-1"
            />
            <IconList
              v-else
              :size="14"
              class="icon me-1"
            />
            {{ groupByHost ? 'Vue chronologique' : 'Regrouper par hôte' }}
          </button>
          <button
            v-if="hasActiveFilters"
            class="btn btn-sm btn-outline-secondary"
            type="button"
            @click="resetFilters"
          >
            Réinitialiser
          </button>
          <span class="ms-auto text-secondary small text-nowrap">
            {{ incidentCountLabel }}
          </span>
        </div>
      </div>
    </div>

    <div
      v-if="loading"
      class="card-body"
    >
      <LoadingSkeleton variant="list" />
    </div>
    <div
      v-else-if="error"
      class="card-body text-center py-5 text-danger"
    >
      {{ error }}
    </div>
    <div
      v-else-if="incidents.length === 0"
      class="card-body"
    >
      <EmptyState
        :icon="IconBell"
        title="Aucune notification enregistrée"
        subtitle="Les alertes et les notifications du release tracker apparaîtront ici"
      />
    </div>
    <div
      v-else-if="filteredIncidents.length === 0"
      class="card-body"
    >
      <EmptyState
        :icon="IconBell"
        title="Aucune notification ne correspond à cette recherche."
        subtitle="Essayez un autre mot-clé ou réinitialisez les filtres."
        :cta-label="hasActiveFilters ? 'Réinitialiser' : ''"
        @cta="resetFilters"
      />
    </div>
    <div
      v-else
      class="table-responsive scroll-table"
    >
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th style="width: 90px;">
              État
            </th>
            <th>Type</th>
            <th>Élément</th>
            <th>Source</th>
            <th>Détails</th>
            <th>
              <SortableHeader
                label="Déclenché"
                :active="sortKey === 'triggered_at'"
                :direction="sortDir"
                @toggle="toggleSort('triggered_at')"
              />
            </th>
            <th>
              <SortableHeader
                label="Terminé"
                :active="sortKey === 'resolved_at'"
                :direction="sortDir"
                @toggle="toggleSort('resolved_at')"
              />
            </th>
            <th style="width: 90px;" />
          </tr>
        </thead>
        <tbody>
          <template
            v-for="row in displayRows"
            :key="row.kind === 'group-header' ? `host-${row.hostKey}` : row.item.id"
          >
            <tr v-if="row.kind === 'group-header'">
              <td
                colspan="8"
                class="bg-body-tertiary"
              >
                <button
                  type="button"
                  class="btn btn-link btn-sm text-decoration-none d-flex align-items-center gap-2 p-0 w-100 text-start"
                  @click="toggleHostGroup(row.hostKey)"
                >
                  <IconChevronRight
                    :size="16"
                    class="icon transition-transform"
                    :class="{ 'rotate-90': !isHostCollapsed(row.hostKey) }"
                  />
                  <span class="fw-medium text-body">{{ row.hostName }}</span>
                  <BadgePill
                    :text="String(row.count)"
                    tone="secondary"
                    compact
                  />
                  <BadgePill
                    v-if="row.activeCount > 0"
                    :text="`${row.activeCount} actif${row.activeCount > 1 ? 's' : ''}`"
                    tone="danger"
                    compact
                  />
                </button>
              </td>
            </tr>
            <template v-else>
              <tr v-if="!groupByHost && row.item._showSeparator">
                <td
                  colspan="8"
                  class="text-center text-muted small py-1 border-top"
                >
                  — Plus de 7 jours —
                </td>
              </tr>
              <tr :class="{ 'text-muted': row.item._isOld }">
                <td>
                  <BadgePill
                    :tone="notificationStateTone(row.item)"
                    :text="notificationStateLabel(row.item)"
                    compact
                  />
                </td>
                <td>
                  <BadgePill
                    :tone="notificationTypeTone(row.item)"
                    :text="notificationTypeLabel(row.item)"
                    compact
                  />
                </td>
                <td>
                  <div
                    class="fw-semibold text-truncate"
                    style="max-width: 220px;"
                    :title="row.item.rule_name"
                  >
                    {{ notificationTitle(row.item) }}
                  </div>
                  <div
                    v-if="row.item.metric"
                    class="text-muted small"
                  >
                    {{ metricLabel(row.item.metric) }}
                  </div>
                </td>
                <td>
                  <router-link
                    v-if="notificationRoute(row.item)"
                    :to="notificationRoute(row.item)"
                    class="text-decoration-none"
                  >
                    {{ row.item.host_name || 'Source inconnue' }}
                  </router-link>
                  <span v-else>{{ row.item.host_name || 'Source inconnue' }}</span>
                  <div
                    v-if="row.item.source_label && row.item.source_label !== row.item.host_name"
                    class="text-muted small text-truncate"
                    :title="row.item.source_label"
                    style="max-width: 260px;"
                  >
                    {{ row.item.source_label }}
                  </div>
                </td>
                <td>
                  <template v-if="isTrackerType(row.item)">
                    <div>
                      Version : <code>{{ row.item.version || '-' }}</code>
                    </div>
                    <div class="text-muted small">
                      {{ trackerStatusLabel(row.item.status) }}
                    </div>
                  </template>
                  <template v-else>
                    <code>{{ formatIncidentValue({ value: row.item.value, metric: row.item.metric, value_label: row.item.value_label }) }}</code>
                    <div
                      v-if="!isCompleted(row.item) && row.item.current_value != null"
                      class="text-muted small mt-1"
                    >
                      Actuel :
                      <span class="fw-medium">{{ formatIncidentValue({ value: row.item.current_value, metric: row.item.metric, value_label: row.item.value_label }) }}</span>
                      <span
                        v-if="resolveHint(row.item)"
                        class="ms-1"
                      >· {{ resolveHint(row.item) }}</span>
                    </div>
                    <div
                      v-if="row.item.command_status"
                      class="text-muted small mt-1"
                    >
                      Remédiation :
                      <span :class="getExecutionStateClass(row.item.command_status)">{{ commandStatusLabel(row.item.command_status) }}</span>
                    </div>
                  </template>
                </td>
                <td class="text-muted small">
                  {{ formatDate(row.item.triggered_at) }}
                </td>
                <td class="text-muted small">
                  <template v-if="row.item.resolved_at">
                    {{ formatDate(row.item.resolved_at) }}
                    <span v-if="!isTrackerType(row.item)">({{ incidentDuration(row.item) }})</span>
                  </template>
                  <span
                    v-else
                    class="text-secondary"
                  >-</span>
                </td>
                <td class="text-nowrap">
                  <button
                    v-if="isAdmin && !isCompleted(row.item) && !isTrackerType(row.item) && !isAcknowledged(row.item) && row.item.id"
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-warning"
                    :disabled="acknowledgingId === row.item.id"
                    title="Accuser réception — je m'en occupe"
                    aria-label="Accuser réception de l'incident"
                    @click="$emit('acknowledge', row.item)"
                  >
                    <span
                      v-if="acknowledgingId === row.item.id"
                      class="spinner-border spinner-border-sm"
                    />
                    <IconEye
                      v-else
                      :size="14"
                      class="icon"
                    />
                  </button>
                  <button
                    v-if="isAdmin && !isCompleted(row.item) && row.item.id"
                    type="button"
                    class="btn btn-icon btn-sm btn-ghost-success"
                    :disabled="resolvingId === row.item.id"
                    title="Clôturer manuellement"
                    @click="$emit('resolve', row.item)"
                  >
                    <span
                      v-if="resolvingId === row.item.id"
                      class="spinner-border spinner-border-sm"
                    />
                    <IconCheck
                      v-else
                      :size="14"
                      class="icon"
                    />
                  </button>
                </td>
              </tr>
            </template>
          </template>
        </tbody>
      </table>
    </div>

    <div
      v-if="!groupByHost && totalPages > 1"
      class="card-footer d-flex align-items-center justify-content-between"
    >
      <p class="m-0 text-muted">
        Page {{ currentPage }} / {{ totalPages }}
      </p>
      <PaginationNav
        :current-page="currentPage"
        :total-pages="totalPages"
        @select="currentPage = $event"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { IconBell, IconCheck, IconChevronRight, IconEye, IconList, IconSearch, IconStack2, IconX } from '@tabler/icons-vue'
import BadgePill from '../common/BadgePill.vue'
import SortableHeader from '../common/SortableHeader.vue'
import EmptyState from '../EmptyState.vue'
import LoadingSkeleton from '../LoadingSkeleton.vue'
import PaginationNav from '../PaginationNav.vue'
import { getExecutionStateClass } from '../../utils/statusClasses'
import {
  notificationStateLabel,
  notificationStateTone,
  notificationTypeLabel,
  notificationTypeTone,
} from '../../utils/notificationBadges'
import {
  formatIncidentValue,
  incidentDuration,
  isTrackerType,
  metricLabel,
  notificationAcknowledged as isAcknowledged,
  notificationResolved as isCompleted,
  notificationRoute,
  notificationTitle,
  resolveHint,
  trackerStatusLabel,
} from '../../utils/incidentFormat'
import type { NotificationItem } from '../../types/generated'

type Incident = NotificationItem

interface AnnotatedIncident extends Incident {
  _isOld: boolean
  _showSeparator: boolean
}

interface GroupHeaderRow {
  kind: 'group-header'
  hostKey: string
  hostName: string
  count: number
  activeCount: number
}
interface ItemRow {
  kind: 'item'
  item: AnnotatedIncident
}
type DisplayRow = GroupHeaderRow | ItemRow

const PAGE_SIZE = 50
const AGE_THRESHOLD_MS = 7 * 24 * 60 * 60 * 1000

const TYPE_FILTERS = [
  { value: 'all', label: 'Tous', activeClass: 'btn-primary shadow-sm' },
  { value: 'crit', label: 'Critique', activeClass: 'btn-danger shadow-sm' },
  { value: 'warn', label: 'Avertissement', activeClass: 'btn-warning shadow-sm' },
  { value: 'tracker', label: 'Tracker', activeClass: 'btn-secondary shadow-sm' },
] as const

const STATUS_FILTERS = [
  { value: 'all', label: 'Tous états', activeClass: 'btn-primary shadow-sm' },
  { value: 'active', label: 'Actifs', activeClass: 'btn-danger shadow-sm' },
  { value: 'acknowledged', label: 'En cours', activeClass: 'btn-warning shadow-sm' },
  { value: 'resolved', label: 'Terminés', activeClass: 'btn-success shadow-sm' },
] as const

const props = withDefaults(defineProps<{
  incidents?: Incident[]
  loading?: boolean
  error?: string
  activeIncidentCount?: number
  isAdmin?: boolean
  initialSearch?: string
  markingRead?: boolean
  resolvingId?: string | number | null
  acknowledgingId?: string | number | null
}>(), {
  incidents: () => [],
  loading: false,
  error: '',
  activeIncidentCount: 0,
  isAdmin: false,
  initialSearch: '',
  markingRead: false,
  resolvingId: null,
  acknowledgingId: null,
})

defineEmits<{
  (e: 'mark-all-read'): void
  (e: 'resolve', item: Incident): void
  (e: 'acknowledge', item: Incident): void
}>()

const filterType = ref('all')
const filterStatus = ref('all')
// Seeded from the caller (e.g. HostDetailView's "?host=" deep link) so
// arriving from a specific host's incidents lands pre-filtered instead of on
// the full undifferentiated list.
const searchQuery = ref(props.initialSearch)
const currentPage = ref(1)
const sortKey = ref<'triggered_at' | 'resolved_at'>('triggered_at')
const sortDir = ref<'asc' | 'desc'>('desc')
const groupByHost = ref(false)
const collapsedHosts = ref(new Set<string>())

const filteredIncidents = computed(() => {
  const search = searchQuery.value.trim().toLowerCase()
  return props.incidents.filter((incident) => {
    if (filterType.value === 'crit') {
      if (isTrackerType(incident)) return false
      if ((incident.severity || '').toLowerCase() !== 'crit') return false
    } else if (filterType.value === 'warn') {
      if (isTrackerType(incident)) return false
      if ((incident.severity || '').toLowerCase() !== 'warn') return false
    } else if (filterType.value === 'tracker') {
      if (!isTrackerType(incident)) return false
    }

    if (filterStatus.value === 'active' && (isCompleted(incident) || isAcknowledged(incident))) return false
    if (filterStatus.value === 'acknowledged' && !isAcknowledged(incident)) return false
    if (filterStatus.value === 'resolved' && !isCompleted(incident)) return false

    if (search) {
      const haystack = [
        incident.rule_name,
        incident.host_name,
        incident.source_label,
        incident.metric,
        incident.type,
        incident.status,
        incident.version,
        incident.value,
      ]
        .filter(Boolean)
        .map((value) => String(value).toLowerCase())
        .join(' ')
      if (!haystack.includes(search)) return false
    }

    return true
  })
})

const hasActiveFilters = computed(() => filterType.value !== 'all' || filterStatus.value !== 'all' || searchQuery.value.trim().length > 0)

const incidentCountLabel = computed(() => {
  const visible = filteredIncidents.value.length
  const total = props.incidents.length
  return `${visible}${visible !== total ? `/${total}` : ''} notification${visible !== 1 ? 's' : ''}`
})

watch([filterType, filterStatus, searchQuery], () => {
  currentPage.value = 1
})

function toggleSort(key: 'triggered_at' | 'resolved_at'): void {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
    return
  }
  sortKey.value = key
  sortDir.value = 'desc'
}

const sortedIncidents = computed(() => {
  const dir = sortDir.value === 'asc' ? 1 : -1
  return [...filteredIncidents.value].sort((a, b) => {
    const av = a[sortKey.value] ? new Date(a[sortKey.value] as string).getTime() : 0
    const bv = b[sortKey.value] ? new Date(b[sortKey.value] as string).getTime() : 0
    return (av - bv) * dir
  })
})

const annotatedIncidents = computed<AnnotatedIncident[]>(() => {
  const now = Date.now()
  let separatorShown = false
  return sortedIncidents.value.map((incident) => {
    const isOld = incident.triggered_at
      ? now - new Date(incident.triggered_at).getTime() > AGE_THRESHOLD_MS
      : false
    const showSeparator = isOld && !separatorShown
    if (isOld) separatorShown = true
    return { ...incident, _isOld: isOld, _showSeparator: showSeparator }
  })
})

const totalPages = computed(() => Math.max(1, Math.ceil(annotatedIncidents.value.length / PAGE_SIZE)))

const paginatedIncidents = computed(() => {
  if (groupByHost.value) return annotatedIncidents.value
  const start = (currentPage.value - 1) * PAGE_SIZE
  return annotatedIncidents.value.slice(start, start + PAGE_SIZE)
})

function isHostCollapsed(key: string): boolean {
  return collapsedHosts.value.has(key)
}

function toggleHostGroup(key: string): void {
  const next = new Set(collapsedHosts.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  collapsedHosts.value = next
}

const displayRows = computed<DisplayRow[]>(() => {
  if (!groupByHost.value) {
    return paginatedIncidents.value.map((item): ItemRow => ({ kind: 'item', item }))
  }

  const order: string[] = []
  const map = new Map<string, AnnotatedIncident[]>()
  for (const item of annotatedIncidents.value) {
    const key = item.host_name || '__sans_hote__'
    if (!map.has(key)) {
      map.set(key, [])
      order.push(key)
    }
    map.get(key)!.push(item)
  }

  const rows: DisplayRow[] = []
  for (const key of order) {
    const list = map.get(key)!
    rows.push({
      kind: 'group-header',
      hostKey: key,
      hostName: key === '__sans_hote__' ? 'Sans hôte' : key,
      count: list.length,
      activeCount: list.filter((item) => !isCompleted(item)).length,
    })
    if (!isHostCollapsed(key)) {
      for (const item of list) rows.push({ kind: 'item', item })
    }
  }
  return rows
})

function setTypeFilter(value: string): void {
  filterType.value = value
  currentPage.value = 1
}

function setStatusFilter(value: string): void {
  filterStatus.value = value
  currentPage.value = 1
}

function clearSearch() {
  searchQuery.value = ''
}

function resetFilters() {
  filterType.value = 'all'
  filterStatus.value = 'all'
  searchQuery.value = ''
  currentPage.value = 1
}

// Describes the remote_commands row a rule's command_trigger dispatched when
// this incident fired (item.command_status, joined server-side from
// remote_commands.status) — adjectival wording ("réussie"/"échouée") to
// agree with "Remédiation :" above it, not the standalone noun forms
// commandStatusLabel in utils/commandStatus.ts uses elsewhere.
function commandStatusLabel(status: string | undefined): string {
  if (status === 'pending') return 'en attente'
  if (status === 'running') return 'en cours'
  if (status === 'completed') return 'réussie'
  if (status === 'failed') return 'échouée'
  return status || 'inconnue'
}

function formatDate(dateStr: string | undefined | null): string {
  if (!dateStr) return ''
  return new Date(dateStr).toLocaleString('fr-FR')
}
</script>

<style scoped>
.transition-transform {
  transition: transform 0.15s ease;
}
.rotate-90 {
  transform: rotate(90deg);
}
</style>
