<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
        <div class="page-pretitle">
          <router-link to="/" class="text-decoration-none">Dashboard</router-link>
          <span class="text-muted mx-1">/</span>
          <span>Menaces web</span>
        </div>
        <h2 class="page-title">Threats</h2>
        <div class="text-secondary">IPs suspectes, chemins scannés, corrélation multi-hôtes et timeline détaillée</div>
      </div>
    </div>

    <div class="card mb-4">
      <div class="card-body d-flex flex-wrap gap-2 align-items-end">
        <div>
          <label class="form-label mb-1">Période</label>
          <select v-model="period" class="form-select form-select-sm" style="min-width: 7rem;">
            <option value="24h">24h</option>
            <option value="168h">7j</option>
            <option value="720h">30j</option>
          </select>
        </div>
        <div>
          <label class="form-label mb-1">Source</label>
          <select v-model="source" class="form-select form-select-sm" style="min-width: 9rem;">
            <option value="">Toutes</option>
            <option value="npm">npm</option>
            <option value="nginx">nginx</option>
            <option value="apache">apache</option>
            <option value="caddy">caddy</option>
          </select>
        </div>
        <div>
          <label class="form-label mb-1">Host ID</label>
          <input v-model.trim="hostId" class="form-control form-control-sm" placeholder="(optionnel)" style="min-width: 14rem;" />
        </div>
        <button class="btn btn-primary btn-sm" @click="loadThreats" :disabled="loading">
          <span v-if="loading" class="spinner-border spinner-border-sm me-1"></span>
          Rafraîchir
        </button>
      </div>
    </div>

    <div class="row row-cards mb-4">
      <div class="col-4">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">Requêtes suspectes</div>
            <div class="h2 mb-0 text-orange">{{ threats.suspicious_requests || 0 }}</div>
          </div>
        </div>
      </div>
      <div class="col-4">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">IPs suspectes</div>
            <div class="h2 mb-0">{{ threats.suspicious_ips || 0 }}</div>
          </div>
        </div>
      </div>
      <div class="col-4">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">Hôtes ciblés</div>
            <div class="h2 mb-0">{{ threats.targeted_hosts || 0 }}</div>
          </div>
        </div>
      </div>
    </div>

    <div class="row row-cards">
      <div class="col-lg-7">
        <div class="card h-100">
          <div class="card-header"><h3 class="card-title mb-0">IPs suspectes</h3></div>
          <div class="table-responsive">
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th>IP</th>
                  <th class="text-end">Hits</th>
                  <th class="text-end">Paths</th>
                  <th class="text-end">Hôtes</th>
                  <th>Niveau</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="!topIPs.length">
                  <td colspan="6" class="text-center text-secondary py-4">Aucune IP suspecte sur la période.</td>
                </tr>
                <tr v-for="ip in topIPs" :key="ip.ip">
                  <td class="font-monospace small">{{ ip.ip }}</td>
                  <td class="text-end">{{ ip.hits || 0 }}</td>
                  <td class="text-end">{{ ip.unique_paths || 0 }}</td>
                  <td class="text-end">{{ ip.host_count || 0 }}</td>
                  <td><span class="badge" :class="levelClass(ip.level)">{{ ip.level || 'LOW' }}</span></td>
                  <td class="text-end">
                    <button class="btn btn-sm btn-outline-primary" @click="openTimeline(ip.ip)">Timeline</button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <div class="col-lg-5">
        <div class="card h-100">
          <div class="card-header"><h3 class="card-title mb-0">Top chemins scannés</h3></div>
          <div class="card-body p-0">
            <div v-if="!topPaths.length" class="text-center py-4 text-secondary small">Aucun path suspect.</div>
            <div v-else v-for="p in topPaths" :key="`${p.path}-${p.category}`" class="d-flex justify-content-between border-bottom px-3 py-2">
              <div>
                <div class="font-monospace small">{{ p.path }}</div>
                <div class="small text-secondary">{{ p.category || 'Unknown' }}</div>
              </div>
              <span class="badge bg-yellow-lt text-yellow">{{ p.hits }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="row row-cards mt-4">
      <div class="col-lg-6">
        <div class="card h-100">
          <div class="card-header"><h3 class="card-title mb-0">Hôtes les plus ciblés</h3></div>
          <div class="table-responsive">
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th>Hôte</th>
                  <th class="text-end">Hits</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="!mostTargetedHosts.length">
                  <td colspan="2" class="text-center text-secondary py-4">Aucun hôte ciblé</td>
                </tr>
                <tr v-for="h in mostTargetedHosts" :key="h.host_id">
                  <td>{{ h.host_name || h.host_id }}</td>
                  <td class="text-end">{{ h.hits || 0 }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
      <div class="col-lg-6">
        <div class="card h-100">
          <div class="card-header"><h3 class="card-title mb-0">IP × Hôtes (scan coordonné)</h3></div>
          <div class="table-responsive">
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th>IP</th>
                  <th class="text-end">Hôtes</th>
                  <th class="text-end">Hits</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="!ipHostMatrix.length">
                  <td colspan="3" class="text-center text-secondary py-4">Pas de scan coordonné détecté</td>
                </tr>
                <tr v-for="m in ipHostMatrix" :key="m.ip">
                  <td class="font-monospace small">{{ m.ip }}</td>
                  <td class="text-end">{{ m.host_count || 0 }}</td>
                  <td class="text-end">{{ m.hits || 0 }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>

    <div v-if="showTimeline" class="timeline-drawer-backdrop" @click.self="closeTimeline">
      <div class="timeline-drawer card shadow-lg">
        <div class="card-header d-flex align-items-center justify-content-between">
          <div>
            <h3 class="card-title mb-0">Timeline IP: <span class="font-monospace">{{ selectedIP }}</span></h3>
            <div class="text-secondary small">Chronologie des requêtes suspectes</div>
          </div>
          <div class="d-flex gap-2">
            <button class="btn btn-sm btn-outline-danger" @click="blockIP" :disabled="blockLoading">Bloquer cette IP</button>
            <button class="btn btn-sm btn-outline-secondary" @click="closeTimeline">Fermer</button>
          </div>
        </div>
        <div class="card-body p-0" style="max-height: 70vh; overflow: auto;">
          <div v-if="timelineLoading" class="text-center py-4 text-secondary">
            <span class="spinner-border spinner-border-sm me-2"></span>
            Chargement timeline...
          </div>
          <div v-else-if="!timeline.length" class="text-center py-4 text-secondary">Aucune requête</div>
          <div v-else v-for="(r, idx) in timeline" :key="`${r.timestamp}-${idx}`" class="border-bottom px-3 py-2">
            <div class="d-flex align-items-center gap-2 mb-1">
              <span class="badge" :class="statusClass(r.status)">{{ r.status }}</span>
              <span class="badge bg-blue-lt text-blue">{{ r.method }}</span>
              <span class="small text-secondary">{{ formatDate(r.timestamp) }}</span>
              <span class="small text-secondary">{{ r.host_name }}</span>
            </div>
            <div class="font-monospace small mb-1">{{ r.domain || '(unknown)' }} {{ r.path }}</div>
            <div class="small text-secondary text-truncate">{{ r.user_agent || '-' }}</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import apiClient from '../api'

type AnyRecord = Record<string, any>

const period = ref('24h')
const source = ref('')
const hostId = ref('')

const loading = ref(false)
const summary = ref<AnyRecord>({ threats: {} })

const showTimeline = ref(false)
const timelineLoading = ref(false)
const blockLoading = ref(false)
const selectedIP = ref('')
const timeline = ref<AnyRecord[]>([])

const threats = computed(() => summary.value.threats || {})
const topIPs = computed(() => threats.value.top_ips || [])
const topPaths = computed(() => threats.value.top_paths || [])
const mostTargetedHosts = computed(() => threats.value.most_targeted_hosts || [])
const ipHostMatrix = computed(() => threats.value.ip_host_matrix || [])

function levelClass(level: string): string {
  switch (level) {
    case 'CRITICAL': return 'bg-red-lt text-red'
    case 'HIGH': return 'bg-orange-lt text-orange'
    case 'MEDIUM': return 'bg-yellow-lt text-yellow'
    default: return 'bg-azure-lt text-azure'
  }
}

function statusClass(status: number): string {
  if (status >= 200 && status < 300) return 'bg-green-lt text-green'
  if (status >= 300 && status < 400) return 'bg-yellow-lt text-yellow'
  return 'bg-red-lt text-red'
}

function formatDate(v: string): string {
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v || '-'
  return d.toLocaleString()
}

async function loadThreats() {
  loading.value = true
  try {
    const res = await apiClient.getWebLogsSummary(period.value, hostId.value || undefined, source.value || undefined)
    summary.value = { threats: res.data?.threats || {} }
  } catch (err) {
    console.error('Failed to load threats summary', err)
  } finally {
    loading.value = false
  }
}

async function openTimeline(ip: string) {
  selectedIP.value = ip
  showTimeline.value = true
  timelineLoading.value = true
  try {
    const res = await apiClient.getIPTimeline(ip, hostId.value || undefined, period.value, 500)
    timeline.value = res.data?.requests || []
  } catch (err) {
    console.error('Failed to load IP timeline', err)
    timeline.value = []
  } finally {
    timelineLoading.value = false
  }
}

function closeTimeline() {
  showTimeline.value = false
  timeline.value = []
  selectedIP.value = ''
}

async function blockIP() {
  // Reuses existing unblock endpoint path contract inversely is not implemented server-side yet.
  blockLoading.value = true
  try {
    console.warn('Block endpoint not implemented yet for IP', selectedIP.value)
  } finally {
    blockLoading.value = false
  }
}

onMounted(loadThreats)
</script>

<style scoped>
.timeline-drawer-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.35);
  display: flex;
  justify-content: flex-end;
  z-index: 1060;
}

.timeline-drawer {
  width: min(760px, 94vw);
  height: 100vh;
  border-radius: 0;
  border: 0;
}
</style>
