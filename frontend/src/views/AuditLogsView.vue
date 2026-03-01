<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
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
    <div v-show="activeTab === 'commandes'">
      <div class="card mb-3">
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
                    class="btn btn-sm btn-outline-secondary"
                    @click="openLogViewer(cmd)"
                    :disabled="!cmd.output && cmd.status === 'pending'"
                  >Logs</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="card-footer d-flex align-items-center justify-content-between">
          <div class="text-secondary small">
            {{ cmdsTotal }} commande{{ cmdsTotal !== 1 ? 's' : '' }} — page {{ cmdsPage }} / {{ totalCmdsPages }}
          </div>
          <div class="btn-group">
            <button class="btn btn-outline-secondary" @click="prevCmdsPage" :disabled="cmdsPage <= 1 || cmdsLoading">Précédent</button>
            <button class="btn btn-outline-secondary" @click="nextCmdsPage" :disabled="cmdsPage >= totalCmdsPages || cmdsLoading">Suivant</button>
          </div>
        </div>
      </div>

      <!-- Log viewer panel -->
      <div v-if="selectedCmd" class="card">
        <div class="card-header d-flex align-items-center justify-content-between" style="background: #1e293b; border-color: rgba(255,255,255,0.1);">
          <div class="d-flex align-items-center gap-3">
            <span :class="moduleClass(selectedCmd.module)">{{ moduleLabel(selectedCmd.module) }}</span>
            <code style="color: #94a3b8;">{{ cmdLabel(selectedCmd) }}</code>
            <span class="text-secondary small">— {{ selectedCmd.host_name || selectedCmd.host_id }}</span>
            <span :class="statusClass(selectedCmd.status)">{{ selectedCmd.status }}</span>
          </div>
          <button class="btn btn-sm btn-ghost-secondary" @click="closeLogViewer">✕</button>
        </div>
        <pre
          ref="logViewerEl"
          style="
            background: #0f172a;
            color: #e2e8f0;
            padding: 1rem;
            margin: 0;
            max-height: 500px;
            overflow-y: auto;
            font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
            font-size: 0.813rem;
            line-height: 1.5;
            border-radius: 0 0 0.5rem 0.5rem;
          "
        >{{ liveOutput || 'Aucune sortie disponible.' }}</pre>
      </div>
    </div>

    <!-- ── Connexions tab (admin only) ────────────────────────────────────── -->
    <div v-show="activeTab === 'connexions'">
      <!-- Stats cards -->
      <div class="row row-cards mb-4">
        <div class="col-sm-4">
          <div class="card">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">Connexions (24h)</div>
              <div class="h2 mb-0">{{ security.stats_24h?.total ?? '—' }}</div>
            </div>
          </div>
        </div>
        <div class="col-sm-4">
          <div class="card">
            <div class="card-body text-center">
              <div class="text-secondary small mb-1">Échecs (24h)</div>
              <div class="h2 mb-0 text-danger">{{ security.stats_24h?.failures ?? '—' }}</div>
            </div>
          </div>
        </div>
        <div class="col-sm-4">
          <div class="card">
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
                  <span class="badge" :class="ev.success ? 'bg-success' : 'bg-danger'">
                    {{ ev.success ? 'Succès' : 'Échec' }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="card-footer d-flex align-items-center justify-content-between">
          <div class="text-secondary small">Page {{ connexionsPage }} / {{ totalConnexionsPages }}</div>
          <div class="btn-group">
            <button class="btn btn-outline-secondary" @click="prevConnexionsPage" :disabled="connexionsPage <= 1 || connexionsLoading">Précédent</button>
            <button class="btn btn-outline-secondary" @click="nextConnexionsPage" :disabled="connexionsPage >= totalConnexionsPages || connexionsLoading">Suivant</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useAuthStore } from '../stores/auth'
import apiClient from '../api'
import { formatDateTime as formatDate } from '../utils/formatters'

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
const liveOutput = ref('')
const logViewerEl = ref(null)
let streamWs = null

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
}

function moduleLabel(module) {
  return MODULE_META[module]?.label ?? module
}

function moduleClass(module) {
  return MODULE_META[module]?.cls ?? 'badge bg-secondary'
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
  if (status === 'completed') return 'badge bg-green-lt text-green'
  if (status === 'failed') return 'badge bg-red-lt text-red'
  return 'badge bg-yellow-lt text-yellow'
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
  if (selectedCmd.value?.id === cmd.id) return
  closeLogViewer()
  selectedCmd.value = cmd
  liveOutput.value = renderOutput(cmd.output || '')

  if (cmd.status === 'running' || cmd.status === 'pending') {
    connectStream(cmd.id)
  }

  nextTick(() => {
    if (logViewerEl.value) logViewerEl.value.scrollTop = logViewerEl.value.scrollHeight
  })
}

function closeLogViewer() {
  if (streamWs) { streamWs.close(); streamWs = null }
  selectedCmd.value = null
  liveOutput.value = ''
}

function connectStream(commandId) {
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  streamWs = new WebSocket(`${protocol}://${window.location.host}/api/v1/ws/commands/stream/${commandId}`)
  streamWs.onopen = () => streamWs.send(JSON.stringify({ type: 'auth', token: auth.token }))
  streamWs.onmessage = (event) => {
    try {
      const p = JSON.parse(event.data)
      if (p.type === 'cmd_stream_init') {
        if (selectedCmd.value) selectedCmd.value.status = p.status
        liveOutput.value = renderOutput(p.output || '')
      } else if (p.type === 'cmd_stream') {
        liveOutput.value += p.chunk
      } else if (p.type === 'cmd_status_update') {
        if (selectedCmd.value) selectedCmd.value.status = p.status
        if (p.output) liveOutput.value = renderOutput(p.output)
      }
      nextTick(() => {
        if (logViewerEl.value) logViewerEl.value.scrollTop = logViewerEl.value.scrollHeight
      })
    } catch { /* ignore */ }
  }
}

function renderOutput(raw) {
  if (!raw) return ''
  const lines = ['']
  let cur = ''
  for (const ch of raw) {
    if (ch === '\r') { cur = ''; lines[lines.length - 1] = ''; continue }
    if (ch === '\n') { cur = ''; lines.push(''); continue }
    cur += ch; lines[lines.length - 1] = cur
  }
  return lines.join('\n')
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
function nextCmdsPage() {
  if (cmdsPage.value >= totalCmdsPages.value) return
  cmdsPage.value += 1; closeLogViewer(); fetchCmds()
}
function prevCmdsPage() {
  if (cmdsPage.value <= 1) return
  cmdsPage.value -= 1; closeLogViewer(); fetchCmds()
}

function nextConnexionsPage() {
  if (connexionsPage.value >= totalConnexionsPages.value) return
  connexionsPage.value += 1; fetchConnexions()
}
function prevConnexionsPage() {
  if (connexionsPage.value <= 1) return
  connexionsPage.value -= 1; fetchConnexions()
}

onMounted(fetchCmds)
onUnmounted(() => { if (streamWs) streamWs.close() })
</script>
