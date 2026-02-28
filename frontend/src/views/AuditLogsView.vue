<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
        <h2 class="page-title">Audit</h2>
        <div class="text-secondary">Historique des actions et des connexions</div>
      </div>
      <div class="d-flex align-items-center gap-2">
        <button class="btn btn-outline-secondary" @click="refresh" :disabled="loading || connexionsLoading">Actualiser</button>
      </div>
    </div>

    <!-- Tab navigation -->
    <ul class="nav nav-tabs mb-4">
      <li class="nav-item">
        <a class="nav-link" :class="{ active: activeTab === 'logs' }" href="#" @click.prevent="activeTab = 'logs'">
          Logs d'audit
        </a>
      </li>
      <li v-if="auth.role === 'admin'" class="nav-item">
        <a class="nav-link" :class="{ active: activeTab === 'connexions' }" href="#" @click.prevent="switchToConnexions">
          Connexions
        </a>
      </li>
    </ul>

    <!-- Logs d'audit tab -->
    <div v-show="activeTab === 'logs'">
      <div class="card">
        <div class="table-responsive">
          <table class="table table-vcenter card-table">
            <thead>
              <tr>
                <th>Date</th>
                <th>Utilisateur</th>
                <th>Action</th>
                <th>Hote</th>
                <th>IP</th>
                <th>Statut</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="log in logs" :key="log.id">
                <td>{{ formatDate(log.created_at) }}</td>
                <td class="fw-semibold">{{ log.username }}</td>
                <td><code>{{ log.action }}</code></td>
                <td>
                  <router-link
                    v-if="log.host_id"
                    :to="`/hosts/${log.host_id}`"
                    class="text-decoration-none fw-semibold"
                  >
                    {{ log.host_name || log.host_id }}
                  </router-link>
                  <span v-else class="text-secondary">-</span>
                </td>
                <td class="text-secondary small">{{ log.ip_address || '-' }}</td>
                <td>
                  <span :class="statusClass(log.status)">{{ log.status }}</span>
                </td>
              </tr>
              <tr v-if="!logs.length && !loading">
                <td colspan="6" class="text-center text-secondary py-4">Aucun log disponible</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="card-footer d-flex align-items-center justify-content-between">
          <div class="text-secondary small">Page {{ page }}</div>
          <div class="btn-group">
            <button class="btn btn-outline-secondary" @click="prevPage" :disabled="page <= 1 || loading">Precedent</button>
            <button class="btn btn-outline-secondary" @click="nextPage" :disabled="logs.length < limit || loading">Suivant</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Connexions tab (admin only) -->
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
            <button class="btn btn-outline-secondary" @click="prevConnexionsPage" :disabled="connexionsPage <= 1 || connexionsLoading">Precedent</button>
            <button class="btn btn-outline-secondary" @click="nextConnexionsPage" :disabled="connexionsPage >= totalConnexionsPages || connexionsLoading">Suivant</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import apiClient from '../api'
import { formatDateTime as formatDate } from '../utils/formatters'

const auth = useAuthStore()

const activeTab = ref('logs')

// Audit logs
const logs = ref([])
const page = ref(1)
const limit = ref(50)
const loading = ref(false)

// Connexions (admin)
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

function statusClass(status) {
  if (status === 'completed') return 'badge bg-green-lt text-green'
  if (status === 'failed') return 'badge bg-red-lt text-red'
  return 'badge bg-yellow-lt text-yellow'
}

async function fetchLogs() {
  loading.value = true
  try {
    const res = await apiClient.getAuditLogs(page.value, limit.value)
    logs.value = res.data?.logs || []
  } catch {
    logs.value = []
  } finally {
    loading.value = false
  }
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
  } catch {
    connexions.value = []
  } finally {
    connexionsLoading.value = false
  }
}

async function switchToConnexions() {
  activeTab.value = 'connexions'
  if (!connexionsLoaded.value) {
    await fetchConnexions()
  }
}

function refresh() {
  if (activeTab.value === 'logs') {
    fetchLogs()
  } else {
    connexionsLoaded.value = false
    fetchConnexions()
  }
}

function nextPage() {
  if (logs.value.length < limit.value) return
  page.value += 1
  fetchLogs()
}

function prevPage() {
  if (page.value <= 1) return
  page.value -= 1
  fetchLogs()
}

function nextConnexionsPage() {
  if (connexionsPage.value >= totalConnexionsPages.value) return
  connexionsPage.value += 1
  fetchConnexions()
}

function prevConnexionsPage() {
  if (connexionsPage.value <= 1) return
  connexionsPage.value -= 1
  fetchConnexions()
}

onMounted(fetchLogs)
</script>
