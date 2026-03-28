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
        <h2 class="page-title">Menaces web</h2>
        <div class="text-secondary">IPs suspectes, chemins scannés, corrélation multi-hôtes et chronologie détaillée</div>
      </div>
    </div>

    <div class="card mb-4">
      <div class="card-body d-flex flex-wrap gap-2 align-items-end threats-filters">
        <div class="threats-filter-field">
          <label class="form-label mb-1">Source</label>
          <select v-model="source" class="form-select form-select-sm" style="min-width: 9rem;">
            <option value="">Toutes</option>
            <option value="npm">npm</option>
            <option value="nginx">nginx</option>
            <option value="apache">apache</option>
            <option value="caddy">caddy</option>
          </select>
        </div>
        <div class="threats-filter-field">
          <label class="form-label mb-1">Hôte technique (ID)</label>
          <input v-model.trim="hostId" class="form-control form-control-sm" placeholder="(optionnel)" style="min-width: 14rem;" />
        </div>
        <button class="btn btn-primary btn-sm threats-refresh-btn" @click="loadThreats" :disabled="loading">
          <span v-if="loading" class="spinner-border spinner-border-sm me-1"></span>
          Rafraîchir
        </button>
      </div>
    </div>

    <div class="row row-cards mb-4">
      <div class="col-12 col-sm-4">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">Requêtes suspectes</div>
            <div class="h2 mb-0 text-orange">{{ threats.suspicious_requests || 0 }}</div>
          </div>
        </div>
      </div>
      <div class="col-12 col-sm-4">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">IPs suspectes</div>
            <div class="h2 mb-0">{{ threats.suspicious_ips || 0 }}</div>
          </div>
        </div>
      </div>
      <div class="col-12 col-sm-4">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">Domaines ciblés</div>
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
                  <th class="text-end">Chemins</th>
                  <th class="text-end">Domaines</th>
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
            <div v-if="!topPaths.length" class="text-center py-4 text-secondary small">Aucun chemin suspect.</div>
            <div v-else v-for="p in topPaths" :key="`${p.path}-${p.category}`" class="d-flex justify-content-between border-bottom px-3 py-2 top-path-row">
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
          <div class="card-header"><h3 class="card-title mb-0">Domaines les plus ciblés</h3></div>
          <div class="table-responsive">
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th>Domaine cible</th>
                  <th class="text-end">Hits</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="!mostTargetedHosts.length">
                  <td colspan="2" class="text-center text-secondary py-4">Aucun domaine ciblé</td>
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
          <div class="card-header"><h3 class="card-title mb-0">IP × Domaines (scan coordonné)</h3></div>
          <div class="table-responsive">
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th>IP</th>
                  <th class="text-end">Domaines</th>
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
        <div class="card-header d-flex align-items-center justify-content-between gap-2 flex-wrap timeline-header">
          <div>
            <h3 class="card-title mb-0">Chronologie IP: <span class="font-monospace">{{ selectedIP }}</span></h3>
            <div class="text-secondary small">Chronologie des requêtes suspectes</div>
          </div>
          <div class="d-flex gap-2 flex-wrap timeline-header-actions">
            <button class="btn btn-sm btn-outline-danger" @click="blockIP" :disabled="blockLoading">Bloquer cette IP</button>
            <button class="btn btn-sm btn-outline-secondary" @click="closeTimeline">Fermer</button>
          </div>
        </div>
        <div class="card-body p-0 timeline-body" style="max-height: 70vh; overflow: auto;">
          <div v-if="timelineLoading" class="text-center py-4 text-secondary">
            <span class="spinner-border spinner-border-sm me-2"></span>
            Chargement chronologie...
          </div>
          <div v-else-if="!timeline.length" class="text-center py-4 text-secondary">Aucune requête</div>
          <template v-else>
            <div class="timeline-frieze border-bottom px-3 py-3">
              <div class="timeline-controls d-flex align-items-center justify-content-between mb-2 gap-2 flex-wrap">
                <div class="d-flex align-items-center gap-2 flex-wrap">
                  <span class="small text-secondary">Intervalle:</span>
                  <div class="btn-group btn-group-sm" role="group" aria-label="Intervalle timeline">
                    <button
                      v-for="opt in timelineIntervalOptions"
                      :key="opt.value"
                      class="btn"
                      :class="selectedInterval === opt.value ? 'btn-primary' : 'btn-outline-secondary'"
                      @click="setTimelineInterval(opt.value)"
                    >
                      {{ opt.label }}
                    </button>
                  </div>
                </div>
                <div class="small text-secondary">
                  Regroupement: {{ timelineBucketLabel }} · {{ timelineBuckets.length }} tranches
                  <span v-if="selectedInterval === 'auto'" class="badge bg-azure-lt text-azure ms-1">
                    Auto cible ~{{ AUTO_BUCKET_TARGET }}
                  </span>
                </div>
                <button class="btn btn-sm btn-outline-secondary" @click="toggleBucketFilter">
                  {{ bucketFilterEnabled ? 'Afficher toutes les requêtes' : 'Filtrer sur la tranche sélectionnée' }}
                </button>
              </div>

              <div class="timeline-kpis mb-3">
                <div class="timeline-kpi-chip">
                  <span class="timeline-kpi-label">Requêtes affichées</span>
                  <span class="timeline-kpi-value">{{ timelineStats.total }}</span>
                </div>
                <div class="timeline-kpi-chip">
                  <span class="timeline-kpi-label">Erreurs</span>
                  <span class="timeline-kpi-value text-red">{{ timelineStats.errors }}</span>
                </div>
                <div class="timeline-kpi-chip">
                  <span class="timeline-kpi-label">Chemins uniques</span>
                  <span class="timeline-kpi-value">{{ timelineStats.uniquePaths }}</span>
                </div>
                <div class="timeline-kpi-chip">
                  <span class="timeline-kpi-label">Domaines cibles uniques</span>
                  <span class="timeline-kpi-value">{{ timelineStats.uniqueVhosts }}</span>
                </div>
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
              <div class="d-flex align-items-center gap-2 mb-1 flex-wrap">
                <span class="badge" :class="statusClass(r.status)">{{ r.status }}</span>
                <span class="badge bg-blue-lt text-blue">{{ r.method }}</span>
                <span class="badge bg-azure-lt text-azure">{{ r.source || 'log' }}</span>
                <span class="small text-secondary">{{ formatDate(r.timestamp) }}</span>
                <span class="small text-secondary">domaine cible: {{ r.domain || '-' }}</span>
              </div>
              <div class="font-monospace small mb-1">{{ r.domain || '(unknown)' }} {{ r.path }}</div>
              <div class="small text-secondary text-truncate" :title="r.user_agent || '-'">{{ r.user_agent || '-' }}</div>
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import apiClient from '../api'

type AnyRecord = Record<string, any>

interface TimelineBucket {
  key: string
  startMs: number
  endMs: number
  count: number
  errorCount: number
  label: string
  rangeLabel: string
  title: string
}

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
const selectedInterval = ref('auto')

const timelineIntervalOptions = [
  { value: 'auto', label: 'Auto', ms: 0 },
  { value: '10s', label: '10s', ms: 10 * 1000 },
  { value: '30s', label: '30s', ms: 30 * 1000 },
  { value: '1m', label: '1m', ms: 60 * 1000 },
  { value: '5m', label: '5m', ms: 5 * 60 * 1000 },
  { value: '10m', label: '10m', ms: 10 * 60 * 1000 },
  { value: '30m', label: '30m', ms: 30 * 60 * 1000 },
  { value: '1h', label: '1h', ms: 60 * 60 * 1000 },
]

const AUTO_BUCKET_TARGET = 10

const threats = computed(() => summary.value.threats || {})
const topIPs = computed(() => threats.value.top_ips || [])
const topPaths = computed(() => threats.value.top_paths || [])
const mostTargetedHosts = computed(() => threats.value.most_targeted_hosts || [])
const ipHostMatrix = computed(() => threats.value.ip_host_matrix || [])

const timelineChrono = computed(() => {
  return [...timeline.value].sort((a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime())
})

const timelineSpanMs = computed(() => {
  if (timelineChrono.value.length < 2) return 0
  const first = new Date(timelineChrono.value[0]?.timestamp).getTime()
  const last = new Date(timelineChrono.value[timelineChrono.value.length - 1]?.timestamp).getTime()
  if (!Number.isFinite(first) || !Number.isFinite(last)) return 0
  return Math.max(0, last - first)
})

const autoTimelineBucketMs = computed(() => {
  const span = timelineSpanMs.value
  if (span <= 0) return 10 * 1000

  const candidateSteps = [
    5 * 1000,
    10 * 1000,
    15 * 1000,
    30 * 1000,
    60 * 1000,
    2 * 60 * 1000,
    5 * 60 * 1000,
    10 * 60 * 1000,
    15 * 60 * 1000,
    30 * 60 * 1000,
    60 * 60 * 1000,
    2 * 60 * 60 * 1000,
    3 * 60 * 60 * 1000,
    6 * 60 * 60 * 1000,
    12 * 60 * 60 * 1000,
    24 * 60 * 60 * 1000,
  ]

  let best = candidateSteps[0]
  let bestDiff = Number.POSITIVE_INFINITY
  for (const step of candidateSteps) {
    const projectedBuckets = Math.max(1, Math.ceil(span / step))
    const diff = Math.abs(projectedBuckets - AUTO_BUCKET_TARGET)
    if (diff < bestDiff || (diff === bestDiff && step < best)) {
      bestDiff = diff
      best = step
    }
  }

  return best
})

const timelineBucketMs = computed(() => {
  const selected = timelineIntervalOptions.find((o) => o.value === selectedInterval.value)
  if (!selected || selected.ms <= 0) return autoTimelineBucketMs.value
  return selected.ms
})

const timelineBucketLabel = computed(() => {
  const ms = timelineBucketMs.value
  if (ms < 60 * 1000) return `${Math.round(ms / 1000)} secondes`
  if (ms < 60 * 60 * 1000) return `${Math.round(ms / (60 * 1000))} minutes`
  return `${Math.round(ms / (60 * 60 * 1000))} heure(s)`
})

const timelineBuckets = computed<TimelineBucket[]>(() => {
  const buckets = new Map<string, Omit<TimelineBucket, 'label' | 'rangeLabel' | 'title'>>()
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
    .map((b): TimelineBucket => ({
      ...b,
      label: formatBucketLabel(b.startMs, timelineBucketMs.value),
      rangeLabel: `${new Date(b.startMs).toLocaleString()} → ${new Date(b.endMs).toLocaleString()}`,
      title: `${new Date(b.startMs).toLocaleString()} (${b.count} req)`,
    }))
})

const selectedBucket = computed(() => {
  if (!selectedBucketKey.value) return null
  return timelineBuckets.value.find((b) => b.key === selectedBucketKey.value) || null
})

const displayedTimeline = computed(() => {
  const source = timelineChrono.value
  if (!bucketFilterEnabled.value || !selectedBucket.value) return [...source].reverse()
  const start = selectedBucket.value.startMs
  const end = selectedBucket.value.endMs
  return source.filter((r) => {
    const ts = new Date(r.timestamp).getTime()
    return Number.isFinite(ts) && ts >= start && ts < end
  }).reverse()
})

const timelineStats = computed(() => {
  const rows = displayedTimeline.value
  const errors = rows.filter((r) => Number(r.status) >= 400).length
  const uniquePaths = new Set(rows.map((r) => String(r.path || ''))).size
  const uniqueVhosts = new Set(rows.map((r) => String(r.domain || '(unknown)'))).size
  return {
    total: rows.length,
    errors,
    uniquePaths,
    uniqueVhosts,
  }
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

function setTimelineInterval(value: string) {
  selectedInterval.value = value
}

function dotSize(count: number): string {
  const max = Math.max(...timelineBuckets.value.map((b) => Number(b.count) || 0), 1)
  const ratio = (Number(count) || 0) / max
  return `${Math.round(10 + ratio * 14)}px`
}

function formatBucketLabel(startMs: number, bucketMs: number): string {
  const d = new Date(startMs)
  if (bucketMs < 60 * 1000) {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  }
  if (bucketMs >= 60 * 60 * 1000) {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

watch([timelineBuckets, timelineBucketMs], () => {
  if (!timelineBuckets.value.length) {
    selectedBucketKey.value = ''
    return
  }
  const stillExists = timelineBuckets.value.some((b) => b.key === selectedBucketKey.value)
  if (!stillExists) {
    selectedBucketKey.value = timelineBuckets.value[timelineBuckets.value.length - 1].key
  }
})

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

.timeline-header {
  position: sticky;
  top: 0;
  z-index: 2;
  background: var(--tblr-bg-surface);
  border-bottom: 1px solid var(--tblr-border-color);
}

.timeline-header-actions {
  justify-content: flex-end;
}

.timeline-frieze-scroll {
  overflow-x: auto;
  padding-bottom: 0.25rem;
}

.timeline-frieze {
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.03) 0%, rgba(15, 23, 42, 0.01) 100%);
}

.timeline-controls .btn-group .btn {
  min-width: 44px;
}

.timeline-kpis {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.5rem;
}

.timeline-kpi-chip {
  border: 1px solid rgba(148, 163, 184, 0.25);
  border-radius: 0.5rem;
  padding: 0.45rem 0.6rem;
  background: rgba(15, 23, 42, 0.02);
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.timeline-kpi-label {
  font-size: 0.72rem;
  color: var(--tblr-secondary);
}

.timeline-kpi-value {
  font-weight: 700;
  font-size: 1rem;
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
  border: 1px solid transparent;
  border-radius: 0.5rem;
  background: rgba(15, 23, 42, 0.02);
  color: inherit;
  min-width: 64px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.25rem;
  padding: 0.2rem 0.25rem;
  transition: transform 120ms ease, border-color 120ms ease, background-color 120ms ease;
}

.timeline-frieze-item:hover {
  border-color: rgba(58, 63, 146, 0.28);
  background: rgba(58, 63, 146, 0.08);
  transform: translateY(-1px);
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

.timeline-frieze-item.active {
  border-color: rgba(226, 75, 74, 0.35);
  background: rgba(226, 75, 74, 0.08);
}

.timeline-frieze-time {
  font-size: 0.72rem;
  color: var(--tblr-secondary);
}

.timeline-frieze-count {
  font-size: 0.72rem;
  font-weight: 600;
}

@media (max-width: 992px) {
  .threats-filters {
    align-items: stretch !important;
  }

  .threats-filter-field {
    flex: 1 1 220px;
  }

  .threats-filter-field .form-select,
  .threats-filter-field .form-control {
    min-width: 0 !important;
    width: 100%;
  }

  .threats-refresh-btn {
    width: 100%;
  }

  .top-path-row {
    gap: 0.5rem;
    align-items: flex-start;
  }

  .top-path-row .font-monospace {
    overflow-wrap: anywhere;
  }

  .timeline-drawer {
    width: min(760px, 100vw);
  }
}

@media (max-width: 992px) {
  .timeline-kpis {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .timeline-header-actions {
    width: 100%;
  }

  .timeline-header-actions .btn {
    flex: 1 1 auto;
  }

  .timeline-drawer {
    height: 100dvh;
  }

  .timeline-body {
    max-height: calc(100dvh - 78px) !important;
  }

  .timeline-kpis {
    grid-template-columns: 1fr;
  }
}
</style>
