<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
        <div class="page-pretitle">
          <router-link to="/" class="text-decoration-none">Dashboard</router-link>
          <span class="text-muted mx-1">/</span>
          <span>Audit</span>
        </div>
        <h2 class="page-title">Audit</h2>
        <div class="text-secondary">Historique des actions, connexions et commandes</div>
      </div>
      <div class="d-flex align-items-center gap-2">
        <button class="btn btn-outline-secondary" @click="refresh" :disabled="connexionsLoading || cmdsLoading">Actualiser</button>
      </div>
    </div>

    <!-- Tab navigation -->
    <ul class="nav nav-tabs mb-4">
      <li v-if="canViewCommands" class="nav-item">
        <a class="nav-link" :class="{ active: activeTab === 'commandes' }" href="#" @click.prevent="switchToCommandes">
          Commandes
        </a>
      </li>
      <li v-if="auth.role === 'admin'" class="nav-item">
        <a class="nav-link" :class="{ active: activeTab === 'connexions' }" href="#" @click.prevent="switchToConnexions">
          Connexions
        </a>
      </li>
    </ul>

    <!-- ── Commandes tab ────────────────────────────────────────────────────── -->
    <div v-show="activeTab === 'commandes'" class="side-layout">
      <!-- Left: table -->
      <div class="side-main">
        <div class="card">
          <div class="table-responsive">
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Hôte</th>
                  <th>Type</th>
                  <th>Commande</th>
                  <th>Utilisateur</th>
                  <th>Statut</th>
                  <th>Durée</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="cmdsLoading">
                  <td colspan="8" class="text-center text-secondary py-3">Chargement...</td>
                </tr>
                <tr v-else-if="!cmds.length">
                  <td colspan="8" class="text-center text-secondary py-4">Aucune commande enregistrée</td>
                </tr>
                <tr
                  v-for="cmd in cmds"
                  :key="cmd.id"
                  :class="{ 'table-active': selectedCmd?.id === cmd.id }"
                >
                  <td class="text-secondary small">{{ formatDate(cmd.created_at) }}</td>
                  <td>
                    <router-link :to="`/hosts/${cmd.host_id}`" class="text-decoration-none fw-semibold">
                      {{ cmd.host_name || cmd.host_id }}
                    </router-link>
                  </td>
                  <td>
                    <span :class="moduleClass(cmd.module)">{{ moduleLabel(cmd.module) }}</span>
                  </td>
                  <td>
                    <code class="small">{{ cmdLabel(cmd) }}</code>
                  </td>
                  <td class="text-secondary small">{{ cmd.triggered_by || '—' }}</td>
                  <td>
                    <span :class="statusClass(cmd.status)">{{ cmd.status }}</span>
                  </td>
                  <td class="text-secondary small">{{ formatDuration(cmd.started_at, cmd.ended_at) }}</td>
                  <td>
                    <button
                      class="btn btn-sm btn-ghost-secondary"
                      @click="openLogViewer(cmd)"
                      :disabled="!cmd.output && cmd.status === 'pending'"
                      title="Voir les logs"
                    >
                      <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-sm" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M4 6l16 0" /><path d="M4 12l16 0" /><path d="M4 18l12 0" /></svg>
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
              <div class="text-secondary small mb-1">Connexions (24h)</div>
              <div class="h2 mb-0">{{ security.stats_24h?.total ?? '—' }}</div>
            </div>
          </div>
        </div>
        <div class="col-sm-4">
          <div class="card card-sm h-100">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">Échecs (24h)</div>
              <div class="h2 mb-0 text-danger">{{ security.stats_24h?.failures ?? '—' }}</div>
            </div>
          </div>
        </div>
        <div class="col-sm-4">
          <div class="card card-sm h-100">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">IPs uniques (24h)</div>
              <div class="h2 mb-0 text-azure">{{ security.stats_24h?.unique_ips ?? '—' }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- Top failed IPs -->
      <div class="card mb-4">
        <div class="card-header">
          <h3 class="card-title">Top IPs — échecs de connexion (24h)</h3>
        </div>
        <div class="card-body p-0">
          <div v-if="!security.top_failed_ips?.length" class="text-center py-4 text-secondary small">
            Aucun échec enregistré sur cette période
          </div>
          <div v-else>
            <div v-for="item in security.top_failed_ips" :key="item.ip_address" class="px-3 py-2 border-bottom">
              <div class="d-flex align-items-center justify-content-between mb-1">
                <span class="font-monospace small">{{ item.ip_address }}</span>
                <span class="badge bg-red-lt text-red">{{ item.fail_count }} échecs</span>
              </div>
              <div class="progress" style="height: 4px;">
                <div
                  class="progress-bar bg-danger"
                  :style="{ width: progressWidth(item.fail_count) + '%' }"
                ></div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- All login events -->
      <div class="card">
        <div class="card-header">
          <h3 class="card-title">Toutes les connexions</h3>
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
                <td colspan="6" class="text-center text-secondary py-3">Chargement...</td>
              </tr>
              <tr v-else-if="!connexions.length">
                <td colspan="6" class="text-center text-secondary py-4">Aucune connexion enregistrée</td>
              </tr>
              <tr v-for="ev in connexions" :key="ev.id">
                <td class="text-secondary small">{{ formatDate(ev.created_at) }}</td>
                <td class="fw-semibold">{{ ev.username }}</td>
                <td class="text-secondary small font-monospace">{{ ev.ip_address }}</td>
                <td class="text-secondary small">{{ parseUA(ev.user_agent).browser }}</td>
                <td class="text-secondary small">{{ parseUA(ev.user_agent).os }}</td>
                <td>
                  <span class="badge" :class="ev.success ? 'bg-green-lt text-green' : 'bg-red-lt text-red'">
                    {{ ev.success ? 'Succès' : 'Échec' }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="card-footer d-flex align-items-center justify-content-between">
          <div class="text-secondary small">Page {{ connexionsPage }} / {{ totalConnexionsPages }}</div>
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
import { useDateFormatter } from '../composables/useDateFormatter'
import { useStatusBadge } from '../composables/useStatusBadge'
import { useCommandStream } from '../composables/useCommandStream'
import PaginationNav from '../components/PaginationNav.vue'
import CommandLogPanel from '../components/CommandLogPanel.vue'

const { formatLocaleDateTime: formatDate } = useDateFormatter()
const { getStatusBadgeClass } = useStatusBadge()

const route = useRoute()
const auth = useAuthStore()
const canViewCommands = computed(() => auth.role === 'admin' || auth.role === 'operator')

const activeTab = ref('commandes')

// ── Commands history ─────────────────────────────────────────────────────────
const cmds = ref([])
const cmdsPage = ref(1)
const cmdsLimit = 50
const cmdsTotal = ref(0)
const cmdsLoading = ref(false)
const cmdsLoaded = ref(false)

const totalCmdsPages = computed(() => Math.max(1, Math.ceil(cmdsTotal.value / cmdsLimit)))

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
const security = ref({ stats_24h: null, top_failed_ips: [] })

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
  openCommandStream(commandId, {
    onInit(p) {
      if (selectedCmd.value) { selectedCmd.value.status = p.status; selectedCmd.value.output = p.output || '' }
    },
    onChunk(p) {
      if (selectedCmd.value) selectedCmd.value.output = (selectedCmd.value.output || '') + p.chunk
    },
    onStatus(p) {
      if (selectedCmd.value) { selectedCmd.value.status = p.status; if (p.output) selectedCmd.value.output = p.output }
    },
  })
}

// ── Data fetching ─────────────────────────────────────────────────────────────
async function fetchCmds() {
  cmdsLoading.value = true
  try {
    const res = await apiClient.getCommandsHistory(cmdsPage.value, cmdsLimit)
    cmds.value = res.data?.commands || []
    cmdsTotal.value = res.data?.total || 0
    cmdsLoaded.value = true
  } catch { cmds.value = [] } finally { cmdsLoading.value = false }
}

async function fetchConnexions() {
  connexionsLoading.value = true
  try {
    const [evRes, secRes] = await Promise.all([
      apiClient.getLoginEventsAdmin(connexionsPage.value, connexionsLimit),
      apiClient.getSecuritySummary(),
    ])
    connexions.value = evRes.data?.events || []
    connexionsTotal.value = evRes.data?.total || 0
    security.value = secRes.data || { stats_24h: null, top_failed_ips: [] }
    connexionsLoaded.value = true
  } catch { connexions.value = [] } finally { connexionsLoading.value = false }
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
      fetchCmds()
    } else if (auth.role === 'admin') {
      fetchConnexions()
    }
  }, 30_000)
})

onUnmounted(() => {
  if (auditPollTimer) {
    clearInterval(auditPollTimer)
    auditPollTimer = null
  }
})
</script>
