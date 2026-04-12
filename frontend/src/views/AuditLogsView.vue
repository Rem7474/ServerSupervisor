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
      <!-- Left: table -->
      <div class="side-main">
        <div class="card mb-3">
          <div class="card-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-2">
            <div>
              <h3 class="card-title mb-0">
                Audit Trail
              </h3>
              <div class="text-secondary small">
                {{ commandLogs.length }} commande{{ commandLogs.length > 1 ? 's' : '' }} tracée{{ commandLogs.length > 1 ? 's' : '' }}
              </div>
            </div>
            <div
              class="btn-group btn-group-sm"
              role="group"
            >
              <button
                v-for="type in ['all', 'command', 'alert', 'config']"
                :key="type"
                class="btn"
                :class="filterType === type ? 'btn-primary' : 'btn-outline-primary'"
                @click="filterType = type"
              >
                {{ type === 'all' ? 'Tous' : type === 'command' ? 'Commandes' : type === 'alert' ? 'Alertes' : 'Config' }}
              </button>
            </div>
          </div>
          <div class="card-body p-0">
            <div
              v-if="auditLogsLoading"
              class="p-3"
            >
              <LoadingSkeleton
                variant="list"
                :lines="4"
              />
            </div>
            <div
              v-else-if="!filteredLogs.length"
              class="text-center text-secondary py-4"
            >
              Aucun événement d'audit correspondant
            </div>
            <div
              v-else
              class="timeline timeline-sm px-3 py-3"
            >
              <div
                v-for="log in filteredLogs"
                :key="log.id"
                class="timeline-item"
              >
                <div
                  class="timeline-point"
                  :class="log.type === 'command' ? 'bg-primary' : log.type === 'alert' ? 'bg-warning' : 'bg-secondary'"
                />
                <div class="timeline-content">
                  <div class="d-flex flex-column flex-md-row justify-content-between gap-2">
                    <div>
                      <div class="fw-semibold">
                        {{ log.title || log.action || log.type }}
                      </div>
                      <div class="text-secondary small">
                        {{ log.description || log.message || '—' }}
                      </div>
                      <code
                        v-if="log.type === 'command' && (log.command || log.action)"
                        class="text-xs d-inline-block mt-1"
                      >
                        {{ log.command || log.action }}
                      </code>
                    </div>
                    <div class="text-md-end">
                      <span
                        class="badge"
                        :class="auditStatusClass(log.status)"
                      >
                        {{ log.status || log.type }}
                      </span>
                      <div class="text-secondary small mt-1">
                        {{ formatRelativeTime(log.timestamp || log.created_at) }}
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <DataToolbar
          searchable
          :search="cmdSearch"
          search-placeholder="Rechercher une commande..."
          @update:search="cmdSearch = $event"
        >
          <template #bottom>
            <div class="row g-2">
              <div class="col-12 col-md-4">
                <select
                  v-model="cmdHostFilter"
                  class="form-select form-select-sm"
                >
                  <option value="">
                    Tous les hôtes
                  </option>
                  <option
                    v-for="h in cmdHosts"
                    :key="h"
                    :value="h"
                  >
                    {{ h }}
                  </option>
                </select>
              </div>
              <div class="col-6 col-md-4">
                <select
                  v-model="cmdStatusFilter"
                  class="form-select form-select-sm"
                >
                  <option value="">
                    Tous les états
                  </option>
                  <option value="pending">
                    pending
                  </option>
                  <option value="running">
                    running
                  </option>
                  <option value="completed">
                    completed
                  </option>
                  <option value="failed">
                    failed
                  </option>
                </select>
              </div>
              <div class="col-6 col-md-4">
                <select
                  v-model="cmdModuleFilter"
                  class="form-select form-select-sm"
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
                    class="text-center text-secondary py-3"
                  >
                    Chargement...
                  </td>
                </tr>
                <tr v-else-if="!displayedCmds.length">
                  <td
                    colspan="8"
                    class="text-center text-secondary py-4"
                  >
                    Aucune commande enregistrée
                  </td>
                </tr>
                <tr
                  v-for="cmd in displayedCmds"
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
                    <span :class="statusClass(cmd.status)">{{ cmd.status }}</span>
                  </td>
                  <td class="text-secondary small">
                    {{ formatDuration(cmd.started_at, cmd.ended_at) }}
                  </td>
                  <td>
                    <button
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
      <!-- Stats cards -->
      <div class="row row-cards mb-4">
        <div class="col-sm-4">
          <div class="card card-sm h-100">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">
                Connexions (24h)
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
                Échecs (24h)
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
                IPs uniques (24h)
              </div>
              <div class="h2 mb-0 text-azure">
                {{ security.stats?.unique_ips ?? '—' }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Top failed IPs -->
      <div class="card mb-4">
        <div class="card-header">
          <h3 class="card-title">
            Top IPs — échecs de connexion (24h)
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
                  class="text-center text-secondary py-3"
                >
                  Chargement...
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

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import apiClient from '../api'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import { useDateFormatter } from '../composables/useDateFormatter'
import { useStatusBadge } from '../composables/useStatusBadge'
import { useCommandStream } from '../composables/useCommandStream'
import { useAuditLogs } from '../composables/useAuditLogs'
import PaginationNav from '../components/PaginationNav.vue'
import CommandLogPanel from '../components/CommandLogPanel.vue'
import DataToolbar from '../components/common/DataToolbar.vue'
import SortableHeader from '../components/common/SortableHeader.vue'

const { formatLocaleDateTime: formatDate, formatRelativeTime } = useDateFormatter()
const { getStatusBadgeClass } = useStatusBadge()
const { auditLogs, isLoading: auditLogsLoading } = useAuditLogs()

const route = useRoute()
const auth = useAuthStore()
const canViewCommands = computed(() => auth.role === 'admin' || auth.role === 'operator')

const activeTab = ref('commandes')
const filterType = ref('all')

const filteredLogs = computed(() => {
  if (filterType.value === 'all') return auditLogs.value
  return auditLogs.value.filter((log) => log.type === filterType.value)
})

const commandLogs = computed(() => auditLogs.value.filter((log) => log.type === 'command'))

// ── Commands history ─────────────────────────────────────────────────────────
const cmds = ref([])
const cmdsPage = ref(1)
const cmdsLimit = 50
const cmdsTotal = ref(0)
const cmdsLoading = ref(false)
const cmdsLoaded = ref(false)

const totalCmdsPages = computed(() => Math.max(1, Math.ceil(cmdsTotal.value / cmdsLimit)))
const cmdSearch = ref('')
const cmdHostFilter = ref('')
const cmdStatusFilter = ref('')
const cmdModuleFilter = ref('')
const cmdSortBy = ref('created_at')
const cmdSortDir = ref('desc')

const cmdHosts = computed(() => {
  const seen = new Set()
  return cmds.value
    .map((c) => c.host_name || c.host_id || '')
    .filter((h) => {
      if (!h || seen.has(h)) return false
      seen.add(h)
      return true
    })
    .sort((a, b) => a.localeCompare(b))
})

const displayedCmds = computed(() => {
  const q = cmdSearch.value.trim().toLowerCase()
  const arr = cmds.value.filter((c) => {
    const hostName = (c.host_name || c.host_id || '').toLowerCase()
    const cmdText = cmdLabel(c).toLowerCase()
    const user = (c.triggered_by || '').toLowerCase()

    const matchSearch = !q || hostName.includes(q) || cmdText.includes(q) || user.includes(q)
    const matchHost = !cmdHostFilter.value || (c.host_name || c.host_id || '') === cmdHostFilter.value
    const matchStatus = !cmdStatusFilter.value || c.status === cmdStatusFilter.value
    const matchModule = !cmdModuleFilter.value || c.module === cmdModuleFilter.value
    return matchSearch && matchHost && matchStatus && matchModule
  })

  const dir = cmdSortDir.value === 'asc' ? 1 : -1
  arr.sort((a, b) => {
    const key = cmdSortBy.value
    let av
    let bv

    if (key === 'created_at') {
      av = new Date(a.created_at || 0).getTime()
      bv = new Date(b.created_at || 0).getTime()
      if (av < bv) return -1 * dir
      if (av > bv) return 1 * dir
      return 0
    }

    if (key === 'command') {
      av = cmdLabel(a)
      bv = cmdLabel(b)
    } else {
      av = a[key] || ''
      bv = b[key] || ''
    }

    return String(av).toLowerCase().localeCompare(String(bv).toLowerCase()) * dir
  })

  return arr
})

const hasActiveCommands = computed(() =>
  cmds.value.some((c) => c.status === 'pending' || c.status === 'running')
)

function toggleCmdSort(key) {
  if (cmdSortBy.value === key) {
    cmdSortDir.value = cmdSortDir.value === 'asc' ? 'desc' : 'asc'
    return
  }
  cmdSortBy.value = key
  cmdSortDir.value = key === 'created_at' ? 'desc' : 'asc'
}

// Log viewer
const selectedCmd = ref(null)
const showLogViewer = ref(false)
let auditPollTimer = null

const { openCommandStream, closeStream } = useCommandStream({ token: () => auth.token })

// ── Connexions (admin) ───────────────────────────────────────────────────────
const connexions = ref([])
const connexionsPage = ref(1)
const connexionsLimit = 50
const connexionsTotal = ref(0)
const connexionsLoading = ref(false)
const connexionsLoaded = ref(false)
const security = ref({ stats: null, top_failed_ips: [] })
const lastCmdFetchAt = ref(0)
const lastConnFetchAt = ref(0)

const totalConnexionsPages = computed(() =>
  Math.max(1, Math.ceil(connexionsTotal.value / connexionsLimit))
)

// ── Module display helpers ────────────────────────────────────────────────────
const MODULE_META = {
  apt:       { label: 'APT',        cls: 'badge bg-azure-lt text-azure' },
  docker:    { label: 'Docker',     cls: 'badge bg-blue-lt text-blue' },
  systemd:   { label: 'Systemd',    cls: 'badge bg-green-lt text-green' },
  journal:   { label: 'Journal',    cls: 'badge bg-purple-lt text-purple' },
  processes: { label: 'Processus',  cls: 'badge bg-orange-lt text-orange' },
  custom:    { label: 'Custom',     cls: 'badge bg-teal-lt text-teal' },
}

function moduleLabel(module) {
  return MODULE_META[module]?.label ?? module
}

function moduleClass(module) {
  return MODULE_META[module]?.cls ?? 'badge bg-secondary-lt text-secondary'
}

function cmdLabel(cmd) {
  const parts = [cmd.action]
  if (cmd.target) parts.push(cmd.target)
  return parts.join(' ')
}

function formatDuration(startedAt, endedAt) {
  if (!startedAt || !endedAt) return '—'
  const diff = Math.max(0, Math.round((new Date(endedAt) - new Date(startedAt)) / 1000))
  if (diff < 60) return `${diff}s`
  const m = Math.floor(diff / 60), s = diff % 60
  return s > 0 ? `${m}m ${s}s` : `${m}m`
}

// ── Status / UA helpers ───────────────────────────────────────────────────────
function statusClass(status) {
  return getStatusBadgeClass(status, 'badge bg-yellow-lt text-yellow')
}

function auditStatusClass(status) {
  const normalized = String(status || '').toLowerCase()
  if (normalized === 'completed' || normalized === 'success') return 'bg-green-lt text-green'
  if (normalized === 'failed' || normalized === 'error') return 'bg-red-lt text-red'
  if (normalized === 'running' || normalized === 'pending') return 'bg-yellow-lt text-yellow'
  return 'bg-secondary-lt text-secondary'
}

function parseUA(ua) {
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

function progressWidth(failCount) {
  const max = Math.max(...(security.value.top_failed_ips?.map(i => i.fail_count) || [1]))
  return max > 0 ? Math.round((failCount / max) * 100) : 0
}

// ── Log viewer ────────────────────────────────────────────────────────────────
function openLogViewer(cmd) {
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

function closeLogViewer() {
  closeStream()
  selectedCmd.value = null
  showLogViewer.value = false
}

function connectStream(commandId) {
  const syncCmdInList = (patch) => {
    const idx = cmds.value.findIndex((c) => c.id === commandId)
    if (idx === -1) return
    const next = [...cmds.value]
    next[idx] = { ...next[idx], ...patch }
    cmds.value = next
  }

  openCommandStream(commandId, {
    onInit(p) {
      if (selectedCmd.value) { selectedCmd.value.status = p.status; selectedCmd.value.output = p.output || '' }
      syncCmdInList({ status: p.status, output: p.output || '' })
    },
    onChunk(p) {
      if (selectedCmd.value) selectedCmd.value.output = (selectedCmd.value.output || '') + p.chunk
    },
    onStatus(p) {
      if (selectedCmd.value) { selectedCmd.value.status = p.status; if (p.output) selectedCmd.value.output = p.output }
      syncCmdInList({ status: p.status, ...(p.output ? { output: p.output } : {}) })
    },
  })
}

// ── Data fetching ─────────────────────────────────────────────────────────────
async function fetchCmds() {
  if (cmdsLoading.value) return
  cmdsLoading.value = true
  try {
    const res = await apiClient.getCommandsHistory(cmdsPage.value, cmdsLimit)
    const nextCmds = res.data?.commands || []
    cmds.value = nextCmds
    await reconcileCommandStatuses(nextCmds)
    cmdsTotal.value = res.data?.total || 0
    cmdsLoaded.value = true
    lastCmdFetchAt.value = Date.now()
  } catch { cmds.value = [] } finally { cmdsLoading.value = false }
}

async function reconcileCommandStatuses(list) {
  const ids = []
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

  const patchById = {}
  snapshots.forEach((result, idx) => {
    if (result.status !== 'fulfilled') return
    const cmd = result.value?.data
    if (!cmd?.id) return
    patchById[ids[idx]] = cmd
  })

  if (!Object.keys(patchById).length) return

  cmds.value = list.map((c) => {
    const snap = patchById[c.id]
    if (!snap) return c
    return {
      ...c,
      status: snap.status || c.status,
      output: snap.output ?? c.output,
      started_at: snap.started_at || c.started_at,
      ended_at: snap.ended_at || c.ended_at,
    }
  })

  if (selectedCmd.value?.id && patchById[selectedCmd.value.id]) {
    selectedCmd.value = {
      ...selectedCmd.value,
      ...patchById[selectedCmd.value.id],
    }
  }
}

async function fetchConnexions() {
  if (connexionsLoading.value) return
  connexionsLoading.value = true
  try {
    const [evRes, secRes] = await Promise.allSettled([
      apiClient.getLoginEventsAdmin(connexionsPage.value, connexionsLimit),
      apiClient.getSecuritySummary(),
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
      security.value = secRes.value.data || { stats: null, top_failed_ips: [] }
    } else {
      security.value = { stats: null, top_failed_ips: [] }
    }
    lastConnFetchAt.value = Date.now()
  } finally { connexionsLoading.value = false }
}

async function switchToCommandes() {
  activeTab.value = 'commandes'
  if (!cmdsLoaded.value) await fetchCmds()
}

async function switchToConnexions() {
  activeTab.value = 'connexions'
  if (!connexionsLoaded.value) await fetchConnexions()
}

function refresh() {
  if (activeTab.value === 'commandes') {
    cmdsLoaded.value = false
    fetchCmds()
  } else {
    connexionsLoaded.value = false
    fetchConnexions()
  }
}

// ── Pagination ────────────────────────────────────────────────────────────────
function selectCmdsPage(page) {
  if (page === cmdsPage.value) return
  cmdsPage.value = page
  closeLogViewer()
  fetchCmds()
}

function selectConnexionsPage(page) {
  if (page === connexionsPage.value) return
  connexionsPage.value = page
  fetchConnexions()
}

onMounted(fetchCmds)
onMounted(async () => {
  const cmdId = route.query.command
  if (cmdId) {
    try {
      const res = await apiClient.getCommandStatus(cmdId)
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
  if (auditPollTimer) {
    clearInterval(auditPollTimer)
    auditPollTimer = null
  }
})
</script>
