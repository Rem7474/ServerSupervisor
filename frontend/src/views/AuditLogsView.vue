<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
        <div class="page-pretitle">
          <router-link
            to="/"
            class="text-decoration-none"
          >
            Dashboard
          </router-link>
          <span class="text-muted mx-1">/</span>
          <span>Audit</span>
        </div>
        <h2 class="page-title">
          Audit
        </h2>
        <div class="text-secondary">
          Historique des actions, connexions et commandes
        </div>
      </div>
      <div class="d-flex align-items-center gap-2">
        <button
          type="button"
          class="btn btn-outline-secondary"
          :disabled="connexionsLoading || cmdsLoading"
          @click="refresh"
        >
          Actualiser
        </button>
      </div>
    </div>

    <!-- Tab navigation -->
    <ul class="nav nav-tabs mb-4">
      <li
        v-if="canViewCommands"
        class="nav-item"
      >
        <a
          class="nav-link"
          :class="{ active: activeTab === 'commandes' }"
          href="#"
          @click.prevent="switchToCommandes"
        >
          Commandes
        </a>
      </li>
      <li
        v-if="auth.role === 'admin'"
        class="nav-item"
      >
        <a
          class="nav-link"
          :class="{ active: activeTab === 'connexions' }"
          href="#"
          @click.prevent="switchToConnexions"
        >
          Connexions
        </a>
      </li>
    </ul>

    <!-- ── Commandes tab ────────────────────────────────────────────────────── -->
    <div
      v-show="activeTab === 'commandes'"
      class="side-layout"
    >
      <div class="side-main">
        <DataToolbar
          searchable
          :search="cmdSearch"
          search-placeholder="Rechercher hôte, commande, utilisateur..."
          @update:search="onSearchUpdate"
        >
          <template #bottom>
            <div class="row g-2">
              <div class="col-6 col-md-4">
                <select
                  v-model="cmdStatusFilter"
                  class="form-select form-select-sm"
                  @change="onFilterChange"
                >
                  <option value="">
                    Tous les états
                  </option>
                  <option value="pending">
                    En attente
                  </option>
                  <option value="running">
                    En cours
                  </option>
                  <option value="completed">
                    Terminé
                  </option>
                  <option value="failed">
                    Échoué
                  </option>
                </select>
              </div>
              <div class="col-6 col-md-4">
                <select
                  v-model="cmdModuleFilter"
                  class="form-select form-select-sm"
                  @change="onFilterChange"
                >
                  <option value="">
                    Tous les modules
                  </option>
                  <option value="apt">
                    APT
                  </option>
                  <option value="docker">
                    Docker
                  </option>
                  <option value="systemd">
                    Systemd
                  </option>
                  <option value="journal">
                    Journal
                  </option>
                  <option value="processes">
                    Processus
                  </option>
                  <option value="custom">
                    Custom
                  </option>
                </select>
              </div>
            </div>
          </template>
        </DataToolbar>

        <div class="card">
          <div class="table-responsive">
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th>
                    <SortableHeader
                      label="Date"
                      :active="cmdSortBy === 'created_at'"
                      :direction="cmdSortDir"
                      @toggle="toggleCmdSort('created_at')"
                    />
                  </th>
                  <th>
                    <SortableHeader
                      label="Hôte"
                      :active="cmdSortBy === 'host_name'"
                      :direction="cmdSortDir"
                      @toggle="toggleCmdSort('host_name')"
                    />
                  </th>
                  <th>
                    <SortableHeader
                      label="Type"
                      :active="cmdSortBy === 'module'"
                      :direction="cmdSortDir"
                      @toggle="toggleCmdSort('module')"
                    />
                  </th>
                  <th>
                    <SortableHeader
                      label="Commande"
                      :active="cmdSortBy === 'command'"
                      :direction="cmdSortDir"
                      @toggle="toggleCmdSort('command')"
                    />
                  </th>
                  <th>
                    <SortableHeader
                      label="Utilisateur"
                      :active="cmdSortBy === 'triggered_by'"
                      :direction="cmdSortDir"
                      @toggle="toggleCmdSort('triggered_by')"
                    />
                  </th>
                  <th>
                    <SortableHeader
                      label="Statut"
                      :active="cmdSortBy === 'status'"
                      :direction="cmdSortDir"
                      @toggle="toggleCmdSort('status')"
                    />
                  </th>
                  <th>Durée</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                <tr v-if="cmdsLoading">
                  <td
                    colspan="8"
                    class="py-2"
                  >
                    <LoadingSkeleton
                      variant="table"
                      :lines="5"
                    />
                  </td>
                </tr>
                <tr v-else-if="!sortedCmds.length">
                  <td
                    colspan="8"
                    class="text-center text-secondary py-4"
                  >
                    Aucune commande enregistrée
                  </td>
                </tr>
                <tr
                  v-for="cmd in sortedCmds"
                  :key="cmd.id"
                  :class="{ 'table-active': selectedCmd?.id === cmd.id }"
                >
                  <td class="text-secondary small">
                    {{ formatDate(cmd.created_at) }}
                  </td>
                  <td>
                    <router-link
                      :to="`/hosts/${cmd.host_id}`"
                      class="text-decoration-none fw-semibold"
                    >
                      {{ cmd.host_name || cmd.host_id }}
                    </router-link>
                  </td>
                  <td>
                    <span :class="moduleClass(cmd.module)">{{ moduleLabel(cmd.module) }}</span>
                  </td>
                  <td>
                    <code class="small">{{ cmdLabel(cmd) }}</code>
                  </td>
                  <td class="text-secondary small">
                    {{ cmd.triggered_by || '—' }}
                  </td>
                  <td>
                    <span :class="statusClass(cmd.status)">{{ statusLabel(cmd.status) }}</span>
                  </td>
                  <td class="text-secondary small">
                    {{ formatDuration(cmd.started_at, cmd.ended_at) }}
                  </td>
                  <td>
                    <button
                      type="button"
                      class="btn btn-sm btn-ghost-secondary"
                      :disabled="!cmd.output && cmd.status === 'pending'"
                      title="Voir les logs"
                      @click="openLogViewer(cmd)"
                    >
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        class="icon icon-sm"
                        width="16"
                        height="16"
                        viewBox="0 0 24 24"
                        stroke-width="2"
                        stroke="currentColor"
                        fill="none"
                      ><path
                        stroke="none"
                        d="M0 0h24v24H0z"
                        fill="none"
                      /><path d="M4 6l16 0" /><path d="M4 12l16 0" /><path d="M4 18l12 0" /></svg>
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="card-footer d-flex align-items-center justify-content-between">
            <div class="text-secondary small">
              {{ cmdsTotal }} commande{{ cmdsTotal !== 1 ? 's' : '' }} — page {{ cmdsPage }} / {{ totalCmdsPages }}
            </div>
            <PaginationNav
              :current-page="cmdsPage"
              :total-pages="totalCmdsPages"
              @select="selectCmdsPage"
            />
          </div>
        </div>
      </div>

      <CommandLogPanel
        :command="selectedCmd"
        :show="showLogViewer"
        wrapper-class="side-panel"
        title="Logs"
        empty-text="Aucun log sélectionné"
        @close="closeLogViewer"
        @open="showLogViewer = true"
      />
    </div>

    <!-- ── Connexions tab (admin only) ────────────────────────────────────── -->
    <div v-show="activeTab === 'connexions'">
      <!-- Period selector + stats cards -->
      <div class="d-flex align-items-center justify-content-between mb-3">
        <div class="btn-group btn-group-sm">
          <button
            v-for="p in secPeriodOptions"
            :key="p.hours"
            type="button"
            class="btn"
            :class="securityPeriod === p.hours ? 'btn-primary' : 'btn-outline-secondary'"
            @click="setSecurityPeriod(p.hours)"
          >
            {{ p.label }}
          </button>
        </div>
      </div>
      <div class="row row-cards mb-4">
        <div class="col-sm-4">
          <div class="card card-sm h-100">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">
                Connexions ({{ securityPeriodLabel }})
              </div>
              <div class="h2 mb-0">
                {{ security.stats?.total ?? '—' }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-sm-4">
          <div class="card card-sm h-100">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">
                Échecs ({{ securityPeriodLabel }})
              </div>
              <div class="h2 mb-0 text-danger">
                {{ security.stats?.failures ?? '—' }}
              </div>
            </div>
          </div>
        </div>
        <div class="col-sm-4">
          <div class="card card-sm h-100">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">
                IPs uniques ({{ securityPeriodLabel }})
              </div>
              <div class="h2 mb-0 text-azure">
                {{ security.stats?.unique_ips ?? '—' }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- IPs bloquées + Top failed IPs -->
      <div class="row row-cards mb-4">
        <div class="col-lg-5">
          <div class="card h-100">
            <div class="card-header">
              <h3 class="card-title">
                IPs bloquées
              </h3>
            </div>
            <div class="card-body p-0">
              <div
                v-if="!security.blocked_ips?.length"
                class="text-center py-4 text-secondary small"
              >
                Aucune IP bloquée
              </div>
              <div v-else>
                <div
                  v-for="ip in security.blocked_ips"
                  :key="ip"
                  class="d-flex align-items-center justify-content-between px-3 py-2 border-bottom"
                >
                  <div class="d-flex align-items-center gap-2">
                    <span class="badge bg-red-lt text-red">Bloquée</span>
                    <span class="font-monospace small">{{ ip }}</span>
                  </div>
                  <button
                    type="button"
                    class="btn btn-sm btn-outline-success"
                    :disabled="unblockingIP === ip"
                    @click="unblockIP(ip)"
                  >
                    {{ unblockingIP === ip ? '…' : 'Débloquer' }}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div class="col-lg-7">
          <div class="card h-100">
            <div class="card-header">
              <h3 class="card-title">
                Top IPs — échecs de connexion ({{ securityPeriodLabel }})
              </h3>
            </div>
            <div class="card-body p-0">
              <div
                v-if="!security.top_failed_ips?.length"
                class="text-center py-4 text-secondary small"
              >
                Aucun échec enregistré sur cette période
              </div>
              <div v-else>
                <div
                  v-for="item in security.top_failed_ips"
                  :key="item.ip_address"
                  class="px-3 py-2 border-bottom"
                >
                  <div class="d-flex align-items-center justify-content-between mb-1">
                    <span class="font-monospace small">{{ item.ip_address }}</span>
                    <span class="badge bg-red-lt text-red">{{ item.fail_count }} échecs</span>
                  </div>
                  <div
                    class="progress"
                    style="height: 4px;"
                  >
                    <div
                      class="progress-bar bg-danger"
                      :style="{ width: progressWidth(item.fail_count) + '%' }"
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- All login events -->
      <div class="card">
        <div class="card-header">
          <h3 class="card-title">
            Toutes les connexions
          </h3>
        </div>
        <div class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>Date / Heure</th>
                <th>Utilisateur</th>
                <th>IP</th>
                <th>Navigateur</th>
                <th>OS</th>
                <th>Statut</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="connexionsLoading">
                <td
                  colspan="6"
                  class="py-2"
                >
                  <LoadingSkeleton
                    variant="table"
                    :lines="4"
                  />
                </td>
              </tr>
              <tr v-else-if="!connexions.length">
                <td
                  colspan="6"
                  class="text-center text-secondary py-4"
                >
                  Aucune connexion enregistrée
                </td>
              </tr>
              <tr
                v-for="ev in connexions"
                :key="ev.id"
              >
                <td class="text-secondary small">
                  {{ formatDate(ev.created_at) }}
                </td>
                <td class="fw-semibold">
                  {{ ev.username }}
                </td>
                <td class="text-secondary small font-monospace">
                  {{ ev.ip_address }}
                </td>
                <td class="text-secondary small">
                  {{ parseUA(ev.user_agent).browser }}
                </td>
                <td class="text-secondary small">
                  {{ parseUA(ev.user_agent).os }}
                </td>
                <td>
                  <span
                    class="badge"
                    :class="ev.success ? 'bg-green-lt text-green' : 'bg-red-lt text-red'"
                  >
                    {{ ev.success ? 'Succès' : 'Échec' }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="card-footer d-flex align-items-center justify-content-between">
          <div class="text-secondary small">
            Page {{ connexionsPage }} / {{ totalConnexionsPages }}
          </div>
          <PaginationNav
            :current-page="connexionsPage"
            :total-pages="totalConnexionsPages"
            @select="selectConnexionsPage"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import apiClient from '../api'
import { useDateFormatter } from '../composables/useDateFormatter'
import { useStatusBadge } from '../composables/useStatusBadge'
import { useCommandStream } from '../composables/useCommandStream'
import PaginationNav from '../components/PaginationNav.vue'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'
import DataToolbar from '../components/common/DataToolbar.vue'
import type { RemoteCommandWithHost } from '../types/audit'
import SortableHeader from '../components/common/SortableHeader.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'

const { formatLocaleDateTime: formatDate } = useDateFormatter()
const { getStatusBadgeClass } = useStatusBadge()

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const canViewCommands = computed(() => auth.role === 'admin' || auth.role === 'operator')

const activeTab = ref((route.query.tab as string) || 'commandes')

watch(activeTab, (tab) => {
  router.replace({ query: { ...route.query, tab } })
})

// ── Commands history ─────────────────────────────────────────────────────────
const cmds = ref<RemoteCommandWithHost[]>([])
const cmdsPage = ref(1)
const cmdsLimit = 50
const cmdsTotal = ref(0)
const cmdsLoading = ref(false)
const cmdsLoaded = ref(false)

const totalCmdsPages = computed(() => Math.max(1, Math.ceil(cmdsTotal.value / cmdsLimit)))
const cmdSearch = ref('')
const cmdStatusFilter = ref('')
const cmdModuleFilter = ref('')
const cmdSortBy = ref('created_at')
const cmdSortDir = ref('desc')

// Client-side sort only (server already returns filtered+paginated, just re-sort current page)
const sortedCmds = computed(() => {
  const arr = [...cmds.value]
  const dir = cmdSortDir.value === 'asc' ? 1 : -1
  arr.sort((a, b) => {
    const key = cmdSortBy.value
    if (key === 'created_at') {
      const av = new Date(a.created_at || 0).getTime()
      const bv = new Date(b.created_at || 0).getTime()
      return (av < bv ? -1 : av > bv ? 1 : 0) * dir
    }
    const av = key === 'command' ? cmdLabel(a) : ((a as Record<string, unknown>)[key] || '')
    const bv = key === 'command' ? cmdLabel(b) : ((b as Record<string, unknown>)[key] || '')
    return String(av).toLowerCase().localeCompare(String(bv).toLowerCase()) * dir
  })
  return arr
})

const hasActiveCommands = computed(() =>
  cmds.value.some((c) => c.status === 'pending' || c.status === 'running')
)

function toggleCmdSort(key: string): void {
  if (cmdSortBy.value === key) {
    cmdSortDir.value = cmdSortDir.value === 'asc' ? 'desc' : 'asc'
    return
  }
  cmdSortBy.value = key
  cmdSortDir.value = key === 'created_at' ? 'desc' : 'asc'
}

let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null
function onSearchUpdate(val: string): void {
  cmdSearch.value = val
  if (searchDebounceTimer) clearTimeout(searchDebounceTimer)
  searchDebounceTimer = setTimeout(() => {
    cmdsPage.value = 1
    fetchCmds()
  }, 350)
}

function onFilterChange(): void {
  cmdsPage.value = 1
  fetchCmds()
}

const selectedCmd = ref<any>(null)
const showLogViewer = ref(false)
let auditPollTimer: ReturnType<typeof setInterval> | null = null

const { openCommandStream, closeStream } = useCommandStream()

// ── Connexions (admin) ───────────────────────────────────────────────────────
const connexions = ref<any[]>([])
const connexionsPage = ref(1)
const connexionsLimit = 50
const connexionsTotal = ref(0)
const connexionsLoading = ref(false)
const connexionsLoaded = ref(false)
const security = ref<{ stats: any; blocked_ips: string[]; top_failed_ips: any[] }>({ stats: null, blocked_ips: [], top_failed_ips: [] })
const lastCmdFetchAt = ref(0)
const lastConnFetchAt = ref(0)

const dialog = useConfirmDialog()
const secPeriodOptions = [
  { hours: 24, label: '24h' },
  { hours: 168, label: '7j' },
  { hours: 720, label: '30j' },
]
const securityPeriod = ref(24)
const securityPeriodLabel = computed(() => secPeriodOptions.find((p) => p.hours === securityPeriod.value)?.label ?? '24h')
const unblockingIP = ref('')

const totalConnexionsPages = computed(() =>
  Math.max(1, Math.ceil(connexionsTotal.value / connexionsLimit))
)

// ── Module display helpers ────────────────────────────────────────────────────
const MODULE_META: Record<string, { label: string; cls: string }> = {
  apt:       { label: 'APT',        cls: 'badge bg-azure-lt text-azure' },
  docker:    { label: 'Docker',     cls: 'badge bg-blue-lt text-blue' },
  systemd:   { label: 'Systemd',    cls: 'badge bg-green-lt text-green' },
  journal:   { label: 'Journal',    cls: 'badge bg-purple-lt text-purple' },
  processes: { label: 'Processus',  cls: 'badge bg-orange-lt text-orange' },
  custom:    { label: 'Custom',     cls: 'badge bg-teal-lt text-teal' },
}

const STATUS_LABELS: Record<string, string> = {
  pending:   'En attente',
  running:   'En cours',
  completed: 'Terminé',
  failed:    'Échoué',
}

function moduleLabel(module: string): string {
  return MODULE_META[module]?.label ?? module
}

function moduleClass(module: string): string {
  return MODULE_META[module]?.cls ?? 'badge bg-secondary-lt text-secondary'
}

function statusLabel(status: string): string {
  return STATUS_LABELS[status] ?? status
}

function cmdLabel(cmd: any): string {
  const parts = [cmd.action]
  if (cmd.target) parts.push(cmd.target)
  return parts.filter(Boolean).join(' ')
}

function formatDuration(startedAt: string | null | undefined, endedAt: string | null | undefined): string {
  if (!startedAt || !endedAt) return '—'
  const diff = Math.max(0, Math.round((new Date(endedAt).getTime() - new Date(startedAt).getTime()) / 1000))
  if (diff < 60) return `${diff}s`
  const m = Math.floor(diff / 60), s = diff % 60
  return s > 0 ? `${m}m ${s}s` : `${m}m`
}

function statusClass(status: string | undefined): string {
  return getStatusBadgeClass(status, 'badge bg-yellow-lt text-yellow')
}

function parseUA(ua: string | undefined): { browser: string; os: string } {
  if (!ua) return { browser: '—', os: '—' }
  const browser = ua.includes('Firefox/') ? 'Firefox'
    : ua.includes('Edg/') ? 'Edge'
    : ua.includes('Chrome/') ? 'Chrome'
    : ua.includes('Safari/') ? 'Safari' : 'Other'
  const os = ua.includes('Windows') ? 'Windows'
    : ua.includes('Mac OS X') ? 'macOS'
    : ua.includes('Android') ? 'Android'
    : (ua.includes('iPhone') || ua.includes('iPad')) ? 'iOS'
    : ua.includes('Linux') ? 'Linux' : 'Other'
  return { browser, os }
}

function progressWidth(failCount: number): number {
  const max = Math.max(...(security.value.top_failed_ips?.map((i: any) => i.fail_count) || [1]))
  return max > 0 ? Math.round((failCount / max) * 100) : 0
}

function openLogViewer(cmd: any): void {
  if (selectedCmd.value?.id === cmd.id) {
    showLogViewer.value = true
    return
  }
  closeLogViewer()
  selectedCmd.value = { ...cmd }
  showLogViewer.value = true

  if (cmd.status === 'running' || cmd.status === 'pending') {
    connectStream(cmd.id)
  }
}

function closeLogViewer(): void {
  closeStream()
  selectedCmd.value = null
  showLogViewer.value = false
}

function connectStream(commandId: string): void {
  const syncCmdInList = (patch: any): void => {
    const idx = cmds.value.findIndex((c: any) => c.id === commandId)
    if (idx === -1) return
    const next = [...cmds.value]
    next[idx] = { ...next[idx], ...patch }
    cmds.value = next
  }

  openCommandStream(commandId, {
    onInit(p: any) {
      if (selectedCmd.value) { selectedCmd.value.status = p.status; selectedCmd.value.output = p.output || '' }
      syncCmdInList({ status: p.status, output: p.output || '' })
    },
    onChunk(p: any) {
      if (selectedCmd.value) selectedCmd.value.output = (selectedCmd.value.output || '') + p.chunk
    },
    onStatus(p: any) {
      if (selectedCmd.value) { selectedCmd.value.status = p.status; if (p.output) selectedCmd.value.output = p.output }
      syncCmdInList({ status: p.status, ...(p.output ? { output: p.output } : {}) })
    },
  })
}

// ── Data fetching ─────────────────────────────────────────────────────────────
async function fetchCmds(): Promise<void> {
  if (cmdsLoading.value) return
  cmdsLoading.value = true
  try {
    const filters = {
      search: cmdSearch.value.trim() || undefined,
      module: cmdModuleFilter.value || undefined,
      status: cmdStatusFilter.value || undefined,
    }
    const res = await apiClient.getCommandsHistory(cmdsPage.value, cmdsLimit, filters)
    const nextCmds = res.data?.commands || []
    cmds.value = nextCmds
    await reconcileCommandStatuses(nextCmds)
    cmdsTotal.value = res.data?.total || 0
    cmdsLoaded.value = true
    lastCmdFetchAt.value = Date.now()
  } catch { cmds.value = [] } finally { cmdsLoading.value = false }
}

async function reconcileCommandStatuses(list: any[]): Promise<void> {
  const ids: string[] = []
  for (const c of list) {
    if (c.status === 'pending' || c.status === 'running') {
      ids.push(c.id)
    }
  }
  if (selectedCmd.value?.id && !ids.includes(selectedCmd.value.id)) {
    ids.push(selectedCmd.value.id)
  }
  if (!ids.length) return

  const snapshots = await Promise.allSettled(ids.map((id) => apiClient.getCommandStatus(id)))
  if (!snapshots.length) return

  const patchById: Record<string, any> = {}
  snapshots.forEach((result: any, idx: number) => {
    if (result.status !== 'fulfilled') return
    const cmd = result.value?.data
    if (!cmd?.id) return
    patchById[ids[idx]] = cmd
  })

  if (!Object.keys(patchById).length) return

  cmds.value = list.map((c: any) => {
    const snap = patchById[c.id]
    if (!snap) return c
    const isActive = snap.status === 'running' || snap.status === 'pending'
    return {
      ...c,
      status: snap.status || c.status,
      // Don't overwrite streamed output with empty DB value while command is active
      output: isActive ? c.output : (snap.output ?? c.output),
      started_at: snap.started_at || c.started_at,
      ended_at: snap.ended_at || c.ended_at,
    }
  })

  if (selectedCmd.value?.id && patchById[selectedCmd.value.id]) {
    const snap = patchById[selectedCmd.value.id]
    const isActive = snap.status === 'running' || snap.status === 'pending'
    selectedCmd.value = {
      ...selectedCmd.value,
      ...snap,
      // Preserve streamed output accumulated from WebSocket while command is still active
      output: isActive ? selectedCmd.value.output : (snap.output ?? selectedCmd.value.output),
    }
  }
}

async function fetchConnexions(): Promise<void> {
  if (connexionsLoading.value) return
  connexionsLoading.value = true
  try {
    const [evRes, secRes] = await Promise.allSettled([
      apiClient.getLoginEventsAdmin(connexionsPage.value, connexionsLimit),
      apiClient.getSecuritySummary(securityPeriod.value),
    ])

    if (evRes.status === 'fulfilled') {
      connexions.value = evRes.value.data?.events || []
      connexionsTotal.value = evRes.value.data?.total || 0
      connexionsLoaded.value = true
    } else {
      connexions.value = []
      connexionsTotal.value = 0
    }

    if (secRes.status === 'fulfilled') {
      security.value = secRes.value.data || { stats: null, blocked_ips: [], top_failed_ips: [] }
    } else {
      security.value = { stats: null, blocked_ips: [], top_failed_ips: [] }
    }
    lastConnFetchAt.value = Date.now()
  } finally { connexionsLoading.value = false }
}

async function switchToCommandes(): Promise<void> {
  activeTab.value = 'commandes'
  if (!cmdsLoaded.value) await fetchCmds()
}

async function switchToConnexions(): Promise<void> {
  activeTab.value = 'connexions'
  if (!connexionsLoaded.value) await fetchConnexions()
}

function refresh(): void {
  if (activeTab.value === 'commandes') {
    cmdsLoaded.value = false
    fetchCmds()
  } else {
    connexionsLoaded.value = false
    fetchConnexions()
  }
}

// ── Pagination ────────────────────────────────────────────────────────────────
function selectCmdsPage(page: number): void {
  if (page === cmdsPage.value) return
  cmdsPage.value = page
  closeLogViewer()
  fetchCmds()
}

function selectConnexionsPage(page: number): void {
  if (page === connexionsPage.value) return
  connexionsPage.value = page
  fetchConnexions()
}

async function setSecurityPeriod(hours: number): Promise<void> {
  securityPeriod.value = hours
  try {
    const res = await apiClient.getSecuritySummary(hours)
    security.value = res.data || { stats: null, blocked_ips: [], top_failed_ips: [] }
  } catch { /* keep stale data */ }
}

async function unblockIP(ip: string): Promise<void> {
  const ok = await dialog.confirm({
    title: 'Débloquer cette IP',
    message: `Retirer l'IP ${ip} de la liste noire ?`,
    variant: 'warning',
  })
  if (!ok) return
  unblockingIP.value = ip
  try {
    await apiClient.unblockIP(ip)
    const res = await apiClient.getSecuritySummary(securityPeriod.value)
    security.value = res.data || { stats: null, blocked_ips: [], top_failed_ips: [] }
  } catch { /* ignore */ } finally {
    unblockingIP.value = ''
  }
}

onMounted(async () => {
  if (route.query.module) cmdModuleFilter.value = String(route.query.module)
  await fetchCmds()
  const cmdId = route.query.command
  if (cmdId) {
    try {
      const res = await apiClient.getCommandStatus(String(cmdId))
      if (res.data?.id) openLogViewer(res.data)
    } catch { /* ignore — command may not exist */ }
  }
})
onMounted(() => {
  auditPollTimer = setInterval(() => {
    if (activeTab.value === 'commandes') {
      const now = Date.now()
      const refreshMs = hasActiveCommands.value ? 5000 : 30000
      if (now - lastCmdFetchAt.value >= refreshMs) {
        fetchCmds()
      }
    } else if (auth.role === 'admin') {
      const now = Date.now()
      if (now - lastConnFetchAt.value >= 30000) {
        fetchConnexions()
      }
    }
  }, 5000)
})

onUnmounted(() => {
  if (searchDebounceTimer) clearTimeout(searchDebounceTimer)
  if (auditPollTimer) {
    clearInterval(auditPollTimer)
    auditPollTimer = null
  }
})
</script>
