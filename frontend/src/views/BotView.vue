<template>
  <div>
    <div class="threats-topbar mb-3">
      <div class="d-flex align-items-center gap-2">
        <span class="fw-semibold">Menaces web</span>
      </div>
      <div class="d-flex align-items-center gap-2 flex-wrap">
        <span class="small text-secondary">Période :</span>
        <button
          v-for="p in periodOptions"
          :key="p.value"
          class="btn btn-sm"
          :class="period === p.value ? 'btn-primary' : 'btn-outline-secondary'"
          @click="setPeriod(p.value)"
        >
          {{ p.label }}
        </button>
      </div>
    </div>

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
          <template v-else>
            <div class="timeline-frieze border-bottom px-3 py-3">
              <div class="d-flex align-items-center justify-content-between mb-2 gap-2 flex-wrap">
                <div class="small text-secondary">
                  Frise regroupée par {{ timelineBucketLabel }}
                </div>
                <button class="btn btn-sm btn-outline-secondary" @click="toggleBucketFilter">
                  {{ bucketFilterEnabled ? 'Afficher toutes les requêtes' : 'Filtrer sur la tranche sélectionnée' }}
                </button>
              </div>

              <div class="timeline-frieze-scroll">
                <div class="timeline-frieze-track">
                  <div class="timeline-frieze-line"></div>
                  <button
                    v-for="bucket in timelineBuckets"
                    :key="bucket.key"
                    class="timeline-frieze-item"
                    :class="{ active: selectedBucketKey === bucket.key }"
                    :title="bucket.title"
                    @click="selectBucket(bucket.key)"
                  >
                    <span class="timeline-frieze-dot" :style="{ width: dotSize(bucket.count), height: dotSize(bucket.count) }"></span>
                    <span class="timeline-frieze-time">{{ bucket.label }}</span>
                    <span class="timeline-frieze-count">{{ bucket.count }}</span>
                  </button>
                </div>
              </div>

              <div v-if="selectedBucket" class="small text-secondary mt-2">
                Tranche sélectionnée: {{ selectedBucket.rangeLabel }} · {{ selectedBucket.count }} requête{{ selectedBucket.count > 1 ? 's' : '' }} · {{ selectedBucket.errorCount }} erreur{{ selectedBucket.errorCount > 1 ? 's' : '' }}
              </div>
            </div>

            <div v-for="(r, idx) in displayedTimeline" :key="`${r.timestamp}-${idx}`" class="border-bottom px-3 py-2">
            <div class="d-flex align-items-center gap-2 mb-1">
              <span class="badge" :class="statusClass(r.status)">{{ r.status }}</span>
              <span class="badge bg-blue-lt text-blue">{{ r.method }}</span>
              <span class="small text-secondary">{{ formatDate(r.timestamp) }}</span>
              <span class="small text-secondary">{{ r.host_name }}</span>
            </div>
            <div class="font-monospace small mb-1">{{ r.domain || '(unknown)' }} {{ r.path }}</div>
            <div class="small text-secondary text-truncate">{{ r.user_agent || '-' }}</div>
          </div>
          </template>
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
const periodOptions = [
  { value: '1h', label: '1h' },
  { value: '24h', label: '24h' },
  { value: '168h', label: '7j' },
  { value: '720h', label: '30j' },
]
const source = ref('')
const hostId = ref('')

const loading = ref(false)
const summary = ref<AnyRecord>({ threats: {} })

const showTimeline = ref(false)
const timelineLoading = ref(false)
const blockLoading = ref(false)
const selectedIP = ref('')
const timeline = ref<AnyRecord[]>([])
const selectedBucketKey = ref('')
const bucketFilterEnabled = ref(true)

const threats = computed(() => summary.value.threats || {})
const topIPs = computed(() => threats.value.top_ips || [])
const topPaths = computed(() => threats.value.top_paths || [])
const mostTargetedHosts = computed(() => threats.value.most_targeted_hosts || [])
const ipHostMatrix = computed(() => threats.value.ip_host_matrix || [])

const timelineChrono = computed(() => {
  return [...timeline.value].sort((a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime())
})

const timelineBucketMs = computed(() => {
  const n = timelineChrono.value.length
  if (n > 220) return 60 * 60 * 1000
  if (n > 80) return 5 * 60 * 1000
  return 60 * 1000
})

const timelineBucketLabel = computed(() => {
  if (timelineBucketMs.value === 60 * 60 * 1000) return 'heure'
  if (timelineBucketMs.value === 5 * 60 * 1000) return '5 minutes'
  return 'minute'
})

const timelineBuckets = computed(() => {
  const buckets = new Map<string, AnyRecord>()
  for (const r of timelineChrono.value) {
    const ts = new Date(r.timestamp).getTime()
    if (!Number.isFinite(ts)) continue
    const startMs = Math.floor(ts / timelineBucketMs.value) * timelineBucketMs.value
    const key = String(startMs)
    const existing = buckets.get(key)
    if (!existing) {
      buckets.set(key, {
        key,
        startMs,
        endMs: startMs + timelineBucketMs.value,
        count: 1,
        errorCount: Number(r.status) >= 400 ? 1 : 0,
      })
      continue
    }
    existing.count += 1
    if (Number(r.status) >= 400) existing.errorCount += 1
  }

  return [...buckets.values()]
    .sort((a, b) => a.startMs - b.startMs)
    .map((b) => ({
      ...b,
      label: formatBucketLabel(b.startMs, timelineBucketMs.value),
      rangeLabel: `${new Date(b.startMs).toLocaleTimeString()} → ${new Date(b.endMs).toLocaleTimeString()}`,
      title: `${new Date(b.startMs).toLocaleString()} (${b.count} req)`
    }))
})

const selectedBucket = computed(() => {
  if (!selectedBucketKey.value) return null
  return timelineBuckets.value.find((b) => b.key === selectedBucketKey.value) || null
})

const displayedTimeline = computed(() => {
  if (!bucketFilterEnabled.value || !selectedBucket.value) return timeline.value
  const start = selectedBucket.value.startMs
  const end = selectedBucket.value.endMs
  return timeline.value.filter((r) => {
    const ts = new Date(r.timestamp).getTime()
    return Number.isFinite(ts) && ts >= start && ts < end
  })
})

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

function setPeriod(value: string) {
  if (period.value === value) return
  period.value = value
  void loadThreats()
}

async function openTimeline(ip: string) {
  selectedIP.value = ip
  showTimeline.value = true
  timelineLoading.value = true
  try {
    const res = await apiClient.getIPTimeline(ip, hostId.value || undefined, period.value, 500)
    timeline.value = res.data?.requests || []
    selectedBucketKey.value = timelineBuckets.value.length ? timelineBuckets.value[timelineBuckets.value.length - 1].key : ''
    bucketFilterEnabled.value = true
  } catch (err) {
    console.error('Failed to load IP timeline', err)
    timeline.value = []
    selectedBucketKey.value = ''
  } finally {
    timelineLoading.value = false
  }
}

function closeTimeline() {
  showTimeline.value = false
  timeline.value = []
  selectedIP.value = ''
  selectedBucketKey.value = ''
}

function selectBucket(key: string) {
  selectedBucketKey.value = key
  bucketFilterEnabled.value = true
}

function toggleBucketFilter() {
  bucketFilterEnabled.value = !bucketFilterEnabled.value
}

function dotSize(count: number): string {
  const max = Math.max(...timelineBuckets.value.map((b) => Number(b.count) || 0), 1)
  const ratio = (Number(count) || 0) / max
  return `${Math.round(10 + ratio * 14)}px`
}

function formatBucketLabel(startMs: number, bucketMs: number): string {
  const d = new Date(startMs)
  if (bucketMs >= 60 * 60 * 1000) {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
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
.threats-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

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

.timeline-frieze-scroll {
  overflow-x: auto;
  padding-bottom: 0.25rem;
}

.timeline-frieze-track {
  position: relative;
  min-width: max-content;
  display: flex;
  gap: 1rem;
  padding: 1.25rem 0.25rem 0.2rem;
}

.timeline-frieze-line {
  position: absolute;
  left: 0;
  right: 0;
  top: 1.9rem;
  height: 4px;
  border-radius: 4px;
  background: linear-gradient(90deg, #3a3f92 0%, #1b2a6d 100%);
}

.timeline-frieze-item {
  position: relative;
  z-index: 1;
  border: 0;
  background: transparent;
  color: inherit;
  min-width: 64px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.25rem;
}

.timeline-frieze-dot {
  display: inline-block;
  border-radius: 999px;
  background: #fff;
  border: 4px solid #2a2f8a;
  box-shadow: 0 1px 0 rgba(0, 0, 0, 0.08);
}

.timeline-frieze-item.active .timeline-frieze-dot {
  border-color: #e24b4a;
}

.timeline-frieze-time {
  font-size: 0.72rem;
  color: var(--tblr-secondary);
}

.timeline-frieze-count {
  font-size: 0.72rem;
  font-weight: 600;
}
</style>
