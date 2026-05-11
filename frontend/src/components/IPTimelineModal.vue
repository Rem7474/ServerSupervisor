<template>
  <div
    v-if="show"
    class="timeline-drawer-backdrop"
    @click.self="$emit('close')"
  >
    <div class="timeline-modal card shadow-lg">
      <div class="card-header d-flex align-items-center justify-content-between gap-2 flex-wrap timeline-header">
        <div>
          <h3 class="card-title mb-0">
            Chronologie IP: <span class="font-monospace">{{ ip }}</span>
          </h3>
          <div class="text-secondary small">
            Chronologie des requêtes suspectes
          </div>
        </div>
        <div class="d-flex gap-2 flex-wrap timeline-header-actions">
          <template v-if="!blocked">
            <select
              v-model="banDuration"
              class="form-select form-select-sm"
              style="width: auto;"
            >
              <option value="1h">1h</option>
              <option value="4h">4h</option>
              <option value="24h">24h</option>
              <option value="48h">48h</option>
              <option value="168h">7j</option>
            </select>
            <button
              class="btn btn-sm"
              :class="banError ? 'btn-danger' : 'btn-outline-danger'"
              :disabled="banLoading || !hostId"
              :title="!hostId ? 'Hôte non déterminé — renseigne le filtre Hôte' : ''"
              @click="handleBanClick"
            >
              <span
                v-if="banLoading"
                class="spinner-border spinner-border-sm me-1"
              />
              <span v-if="banLoading">Blocage…</span>
              <span v-else-if="banError">Erreur — Réessayer</span>
              <span v-else>Bloquer (CrowdSec)</span>
            </button>
          </template>
          <span
            v-else
            class="badge bg-success-lt text-success align-self-center"
          >
            IP bloquée par CrowdSec
          </span>
          <button
            class="btn btn-sm btn-outline-secondary"
            @click="$emit('close')"
          >
            Fermer
          </button>
        </div>
      </div>

      <div class="card-body p-0 timeline-body">
        <div
          v-if="loading"
          class="text-center py-4 text-secondary"
        >
          <span class="spinner-border spinner-border-sm me-2" />
          Chargement chronologie...
        </div>
        <div
          v-else-if="!timeline.length"
          class="text-center py-4 text-secondary"
        >
          Aucune requête
        </div>
        <template v-else>
          <div class="timeline-frieze border-bottom px-3 py-3">
            <div class="timeline-controls d-flex align-items-center justify-content-between mb-2 gap-2">
              <div class="timeline-interval-row">
                <span class="small text-secondary timeline-interval-label">Intervalle:</span>
                <div
                  class="timeline-interval-chips"
                  role="group"
                  aria-label="Intervalle timeline"
                >
                  <button
                    v-for="opt in timelineIntervalOptions"
                    :key="opt.value"
                    class="timeline-interval-chip btn btn-sm"
                    :class="selectedInterval === opt.value ? 'btn-primary' : 'btn-outline-secondary'"
                    @click="setTimelineInterval(opt.value)"
                  >
                    {{ opt.label }}
                  </button>
                </div>
              </div>
              <div class="small text-secondary">
                Regroupement: {{ timelineBucketLabel }} · {{ timelineBuckets.length }} tranches
                <span
                  v-if="selectedInterval === 'auto'"
                  class="badge bg-azure-lt text-azure ms-1"
                >
                  Auto cible ~{{ AUTO_BUCKET_TARGET }}
                </span>
              </div>
              <button
                class="btn btn-sm btn-outline-secondary"
                @click="toggleBucketFilter"
              >
                {{ bucketFilterEnabled ? 'Mode focus: tranche sélectionnée' : 'Mode global: toutes les tranches' }}
              </button>
            </div>

            <details
              class="timeline-controls-collapsible"
              open
            >
              <summary class="timeline-controls-toggle">
                <span>Statistiques</span>
                <span class="timeline-controls-toggle-arrow">▾</span>
              </summary>
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
              <div class="timeline-status-breakdown mb-3">
                <span
                  v-for="item in timelineStatusBreakdown"
                  :key="item.key"
                  class="badge"
                  :class="item.badgeClass"
                >
                  {{ item.label }}: {{ item.count }}
                </span>
              </div>
            </details>

            <div class="timeline-frieze-scroll">
              <div class="timeline-frieze-track">
                <div class="timeline-frieze-line" />
                <button
                  v-for="bucket in timelineBuckets"
                  :key="bucket.key"
                  class="timeline-frieze-item"
                  :class="{ active: selectedBucketKey === bucket.key }"
                  :title="bucket.title"
                  @click="selectBucket(bucket.key)"
                >
                  <span
                    class="timeline-frieze-dot"
                    :class="bucketToneClass(bucket)"
                  />
                  <span class="timeline-frieze-time">{{ bucket.label }}</span>
                  <span class="timeline-frieze-count">{{ bucket.count }}</span>
                </button>
              </div>
            </div>

            <div
              v-if="selectedBucket"
              class="small text-secondary mt-2"
            >
              Tranche sélectionnée: {{ selectedBucket.rangeLabel }} · {{ selectedBucket.count }} requête{{ selectedBucket.count > 1 ? 's' : '' }} · {{ selectedBucket.errorCount }} erreur{{ selectedBucket.errorCount > 1 ? 's' : '' }}
            </div>
          </div>

          <div class="timeline-groups">
            <div
              v-for="group in groupedTimeline"
              :key="group.key"
              class="timeline-group border-bottom"
            >
              <div class="timeline-group-header px-3 py-2">
                <div class="d-flex align-items-center gap-2 flex-wrap">
                  <span class="badge bg-azure-lt text-azure">{{ group.label }}</span>
                  <span class="small text-secondary">{{ group.rangeLabel }}</span>
                </div>
                <div class="timeline-group-kpis">
                  <span class="badge bg-blue-lt text-blue">{{ group.count }} req</span>
                  <span class="badge bg-red-lt text-red">{{ group.errorCount }} erreurs</span>
                  <span class="badge bg-yellow-lt text-yellow">{{ group.uniquePaths }} chemins</span>
                  <span class="badge bg-indigo-lt text-indigo">{{ group.uniqueVhosts }} domaines</span>
                </div>
              </div>

              <div class="timeline-group-events px-3 pb-3">
                <section
                  v-for="statusGroup in group.statusGroups"
                  :key="`${group.key}-${statusGroup.key}`"
                  class="timeline-status-group"
                >
                  <div class="timeline-status-group-header">
                    <span
                      class="badge"
                      :class="statusGroup.badgeClass"
                    >{{ statusGroup.label }}</span>
                    <span class="small text-secondary">{{ statusGroup.count }} log{{ statusGroup.count > 1 ? 's' : '' }}</span>
                  </div>

                  <div class="timeline-status-group-grid">
                    <article
                      v-for="(r, idx) in statusGroup.events"
                      :key="`${group.key}-${statusGroup.key}-${r.timestamp}-${idx}`"
                      class="timeline-event-card"
                    >
                      <div class="timeline-event-topline">
                        <div class="d-flex align-items-center gap-2 flex-wrap">
                          <span
                            class="badge"
                            :class="statusClass(r.status)"
                          >{{ r.status }}</span>
                          <span class="badge bg-blue-lt text-blue">{{ r.method }}</span>
                          <span class="badge bg-azure-lt text-azure">{{ r.source || 'log' }}</span>
                        </div>
                        <span class="small text-secondary">{{ formatDate(r.timestamp) }}</span>
                      </div>
                      <div class="timeline-event-path font-monospace small">
                        {{ r.domain || '(unknown)' }} {{ r.path }}
                      </div>
                      <div class="timeline-event-meta small text-secondary">
                        <span><strong>Domaine:</strong> {{ r.domain || '-' }}</span>
                        <span><strong>Hôte:</strong> {{ r.host_name || '-' }}</span>
                        <span
                          class="text-truncate"
                          :title="r.user_agent || '-'"
                        >
                          <strong>User-Agent:</strong> {{ r.user_agent || '-' }}
                        </span>
                        <span v-if="r.blocked">
                          <strong>Blocage:</strong>
                          <span
                            class="badge bg-success-lt text-success ms-1"
                            :title="r.blocked_reason || '-'"
                          >
                            {{ r.blocked_source || 'crowdsec' }}
                          </span>
                          <span
                            v-if="r.blocked_reason"
                            class="ms-1"
                          >
                            {{ truncate(r.blocked_reason, 64) }}
                          </span>
                        </span>
                        <span v-if="r.blocked_until">
                          <strong>Expire:</strong> {{ formatDate(String(r.blocked_until)) }}
                        </span>
                      </div>
                    </article>
                  </div>
                </section>
              </div>
            </div>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useConfirmDialog } from '../composables/useConfirmDialog'

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

interface TimelineGroup {
  key: string
  label: string
  rangeLabel: string
  count: number
  errorCount: number
  uniquePaths: number
  uniqueVhosts: number
  statusGroups: TimelineStatusGroup[]
}

interface TimelineStatusGroup {
  key: string
  label: string
  badgeClass: string
  count: number
  events: AnyRecord[]
}

const props = defineProps({
  show: { type: Boolean, default: false },
  ip: { type: String, default: '' },
  timeline: { type: Array as () => AnyRecord[], default: () => [] },
  loading: { type: Boolean, default: false },
  blocked: { type: Boolean, default: false },
  banLoading: { type: Boolean, default: false },
  banError: { type: Boolean, default: false },
  hostId: { type: String, default: '' },
})

const emit = defineEmits(['close', 'ban'])

const dialog = useConfirmDialog()
const banDuration = ref('4h')
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

watch(() => props.show, (val) => {
  if (val) {
    bucketFilterEnabled.value = true
    selectedInterval.value = 'auto'
    selectedBucketKey.value = ''
  }
})

const timelineChrono = computed(() =>
  [...props.timeline].sort((a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime())
)

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
    5 * 1000, 10 * 1000, 15 * 1000, 30 * 1000, 60 * 1000,
    2 * 60 * 1000, 5 * 60 * 1000, 10 * 60 * 1000, 15 * 60 * 1000,
    30 * 60 * 1000, 60 * 60 * 1000, 2 * 60 * 60 * 1000, 3 * 60 * 60 * 1000,
    6 * 60 * 60 * 1000, 12 * 60 * 60 * 1000, 24 * 60 * 60 * 1000,
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
      buckets.set(key, { key, startMs, endMs: startMs + timelineBucketMs.value, count: 1, errorCount: Number(r.status) >= 400 ? 1 : 0 })
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

const timelineRows = computed(() => {
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
  const rows = timelineRows.value
  return {
    total: rows.length,
    errors: rows.filter((r) => Number(r.status) >= 400).length,
    uniquePaths: new Set(rows.map((r) => String(r.path || ''))).size,
    uniqueVhosts: new Set(rows.map((r) => String(r.domain || '(unknown)'))).size,
  }
})

const statusFamilyOrder = ['5xx', '4xx', '3xx', '2xx', 'other']

function statusFamilyKey(status: number | string): string {
  const code = Number(status)
  if (!Number.isFinite(code)) return 'other'
  if (code >= 500) return '5xx'
  if (code >= 400) return '4xx'
  if (code >= 300) return '3xx'
  if (code >= 200) return '2xx'
  return 'other'
}

function statusFamilyLabel(family: string): string {
  if (family === '5xx') return '5xx Serveur'
  if (family === '4xx') return '4xx Client'
  if (family === '3xx') return '3xx Redirection'
  if (family === '2xx') return '2xx Succès'
  return 'Autres statuts'
}

function statusFamilyBadgeClass(family: string): string {
  if (family === '5xx') return 'bg-red-lt text-red'
  if (family === '4xx') return 'bg-yellow-lt text-yellow'
  if (family === '3xx') return 'bg-azure-lt text-azure'
  if (family === '2xx') return 'bg-green-lt text-green'
  return 'bg-secondary-lt text-secondary'
}

const timelineStatusBreakdown = computed(() => {
  const counters = new Map<string, number>()
  for (const row of timelineRows.value) {
    const family = statusFamilyKey(row.status)
    counters.set(family, (counters.get(family) || 0) + 1)
  }
  return statusFamilyOrder
    .map((key) => ({ key, label: statusFamilyLabel(key), badgeClass: statusFamilyBadgeClass(key), count: counters.get(key) || 0 }))
    .filter((item) => item.count > 0)
})

const groupedTimeline = computed<TimelineGroup[]>(() => {
  const bucketsByKey = new Map(timelineBuckets.value.map((b) => [b.key, b]))
  const grouped = new Map<string, AnyRecord[]>()
  for (const row of timelineRows.value) {
    const ts = new Date(row.timestamp).getTime()
    if (!Number.isFinite(ts)) continue
    const bucketStart = Math.floor(ts / timelineBucketMs.value) * timelineBucketMs.value
    const key = String(bucketStart)
    const existing = grouped.get(key)
    if (existing) { existing.push(row); continue }
    grouped.set(key, [row])
  }
  return [...grouped.entries()]
    .sort((a, b) => Number(b[0]) - Number(a[0]))
    .map(([key, events]): TimelineGroup => {
      const bucket = bucketsByKey.get(key)
      const rows = [...events].sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
      const errorCount = rows.filter((r) => Number(r.status) >= 400).length
      const uniquePaths = new Set(rows.map((r) => String(r.path || ''))).size
      const uniqueVhosts = new Set(rows.map((r) => String(r.domain || '(unknown)'))).size
      const start = Number(key)
      const end = start + timelineBucketMs.value
      const byStatusFamily = new Map<string, AnyRecord[]>()
      for (const row of rows) {
        const family = statusFamilyKey(row.status)
        const existing = byStatusFamily.get(family)
        if (existing) { existing.push(row); continue }
        byStatusFamily.set(family, [row])
      }
      const statusGroups: TimelineStatusGroup[] = statusFamilyOrder
        .map((familyKey) => {
          const familyRows = byStatusFamily.get(familyKey) || []
          return { key: familyKey, label: statusFamilyLabel(familyKey), badgeClass: statusFamilyBadgeClass(familyKey), count: familyRows.length, events: familyRows }
        })
        .filter((group) => group.count > 0)
      return {
        key,
        label: bucket?.label || formatBucketLabel(start, timelineBucketMs.value),
        rangeLabel: bucket?.rangeLabel || `${new Date(start).toLocaleString()} → ${new Date(end).toLocaleString()}`,
        count: rows.length,
        errorCount,
        uniquePaths,
        uniqueVhosts,
        statusGroups,
      }
    })
})

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

watch(() => props.timeline, (newTimeline) => {
  if (!newTimeline?.length) {
    selectedBucketKey.value = ''
    return
  }
  const lastBucket = timelineBuckets.value[timelineBuckets.value.length - 1]
  if (lastBucket) selectedBucketKey.value = lastBucket.key
  bucketFilterEnabled.value = true
})

function setTimelineInterval(value: string) { selectedInterval.value = value }
function selectBucket(key: string) { selectedBucketKey.value = key; bucketFilterEnabled.value = true }
function toggleBucketFilter() { bucketFilterEnabled.value = !bucketFilterEnabled.value }

function bucketToneClass(bucket: TimelineBucket): string {
  if (bucket.count <= 0) return 'is-calm'
  const errorRate = bucket.errorCount / bucket.count
  if (errorRate >= 0.5) return 'is-hot'
  if (errorRate >= 0.2) return 'is-warm'
  return 'is-calm'
}

function formatBucketLabel(startMs: number, bucketMs: number): string {
  const d = new Date(startMs)
  if (bucketMs < 60 * 1000) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

function statusClass(status: number): string {
  if (status >= 500 && status < 600) return 'bg-red-lt text-red'
  if (status >= 400 && status < 500) return 'bg-yellow-lt text-yellow'
  if (status >= 200 && status < 300) return 'bg-green-lt text-green'
  if (status >= 300 && status < 400) return 'bg-azure-lt text-azure'
  return 'bg-secondary-lt text-secondary'
}

function formatDate(v: string): string {
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v || '-'
  return d.toLocaleString()
}

function truncate(s: string, max: number): string {
  return s.length > max ? s.slice(0, max) + '…' : s
}

async function handleBanClick() {
  const confirmed = await dialog.confirm({
    title: `Bloquer l'IP ${props.ip}`,
    message: `Bloquer ${props.ip} via CrowdSec pour ${banDuration.value} ?`,
    variant: 'danger',
  })
  if (!confirmed) return
  emit('ban', banDuration.value)
}
</script>

<style scoped>
.timeline-drawer-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(var(--tblr-dark-rgb, 15, 23, 42), 0.58);
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 0.75rem;
  z-index: 1060;
}

.timeline-modal {
  width: min(1400px, 98vw);
  height: min(96dvh, 960px);
  border-radius: 0.75rem;
  border: 1px solid var(--tblr-border-color);
  overflow: hidden;
  display: flex;
  flex-direction: column;
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

.timeline-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  background: linear-gradient(
    180deg,
    rgba(var(--tblr-primary-rgb, 32, 107, 196), 0.04) 0%,
    rgba(var(--tblr-primary-rgb, 32, 107, 196), 0.015) 100%
  );
}

.timeline-frieze {
  position: sticky;
  top: 0;
  z-index: 1;
  backdrop-filter: blur(3px);
  background: linear-gradient(180deg, var(--tblr-bg-surface-secondary, #f8fafc) 0%, var(--tblr-bg-surface, #ffffff) 100%);
}

.timeline-interval-row {
  flex: 1 1 auto;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.timeline-interval-chips {
  display: flex;
  gap: 0.3rem;
  flex-wrap: nowrap;
  overflow-x: auto;
  scroll-snap-type: x mandatory;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
  padding-bottom: 2px;
}

.timeline-interval-chips::-webkit-scrollbar {
  display: none;
}

.timeline-interval-chip {
  flex: 0 0 auto;
  scroll-snap-align: start;
  border-radius: 1rem;
  min-width: 2.5rem;
  padding: 0.2rem 0.6rem;
  font-size: 0.78rem;
}

.timeline-controls-collapsible {
  /* transparent wrapper */
}

.timeline-controls-collapsible > summary {
  display: none;
  list-style: none;
}

.timeline-controls-collapsible > summary::-webkit-details-marker {
  display: none;
}

.timeline-controls-toggle-arrow {
  display: inline-block;
  transition: transform 180ms ease;
}

.timeline-kpis {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.5rem;
}

.timeline-kpi-chip {
  border: 1px solid var(--tblr-border-color);
  border-radius: 0.5rem;
  padding: 0.45rem 0.6rem;
  background: var(--tblr-bg-surface-secondary, #f8fafc);
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
  background: linear-gradient(90deg, var(--tblr-primary) 0%, var(--tblr-azure) 100%);
}

.timeline-frieze-item {
  position: relative;
  z-index: 1;
  border: 1px solid transparent;
  border-radius: 0.5rem;
  background: var(--tblr-bg-surface-secondary, #f8fafc);
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
  border-color: rgba(var(--tblr-primary-rgb, 32, 107, 196), 0.35);
  background: rgba(var(--tblr-primary-rgb, 32, 107, 196), 0.08);
  transform: translateY(-1px);
}

.timeline-frieze-dot {
  display: inline-block;
  border-radius: 999px;
  background: var(--tblr-bg-surface, #ffffff);
  width: 14px;
  height: 14px;
  border: 3px solid var(--tblr-primary);
  box-shadow: 0 1px 0 rgba(var(--tblr-dark-rgb, 15, 23, 42), 0.15);
}

.timeline-frieze-dot.is-calm {
  border-color: var(--tblr-blue);
  background: var(--tblr-blue-lt);
}

.timeline-frieze-dot.is-warm {
  border-color: var(--tblr-yellow);
  background: var(--tblr-yellow-lt);
}

.timeline-frieze-dot.is-hot {
  border-color: var(--tblr-red);
  background: var(--tblr-red-lt);
}

.timeline-frieze-item.active .timeline-frieze-dot {
  border-color: var(--tblr-red);
}

.timeline-frieze-item.active {
  border-color: rgba(var(--tblr-red-rgb, 214, 57, 57), 0.35);
  background: rgba(var(--tblr-red-rgb, 214, 57, 57), 0.08);
}

.timeline-frieze-time {
  font-size: 0.72rem;
  color: var(--tblr-secondary);
}

.timeline-frieze-count {
  font-size: 0.72rem;
  font-weight: 600;
}

.timeline-groups {
  display: flex;
  flex-direction: column;
}

.timeline-group {
  background: var(--tblr-bg-surface, #ffffff);
}

.timeline-group-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
  background: var(--tblr-bg-surface-secondary, #f8fafc);
  border-bottom: 1px solid var(--tblr-border-color);
}

.timeline-group-kpis {
  display: flex;
  gap: 0.35rem;
  flex-wrap: wrap;
}

.timeline-group-events {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.timeline-status-breakdown {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex-wrap: wrap;
}

.timeline-status-group {
  border: 1px solid var(--tblr-border-color);
  border-radius: 0.6rem;
  padding: 0.5rem;
  background: var(--tblr-bg-surface, #ffffff);
}

.timeline-status-group-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
  margin-bottom: 0.5rem;
}

.timeline-status-group-grid {
  display: grid;
  gap: 0.55rem;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
}

.timeline-event-card {
  border: 1px solid var(--tblr-border-color);
  border-radius: 0.6rem;
  padding: 0.55rem 0.65rem;
  background: var(--tblr-bg-surface, #ffffff);
}

.timeline-event-topline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
  flex-wrap: wrap;
  margin-bottom: 0.35rem;
}

.timeline-event-path {
  margin-bottom: 0.35rem;
  overflow-wrap: anywhere;
}

.timeline-event-meta {
  display: grid;
  grid-template-columns: 1fr;
  gap: 0.25rem;
}

@media (max-width: 992px) {
  .timeline-modal {
    width: min(1400px, 100vw);
    height: 100dvh;
    border-radius: 0;
  }

  .timeline-kpis {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .timeline-modal {
    height: 100dvh;
    border-radius: 0;
  }

  .timeline-header.card-header {
    padding-top: 0.5rem;
    padding-bottom: 0.5rem;
  }

  .timeline-header .text-secondary.small {
    display: none;
  }

  .timeline-header .card-title {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font-size: 0.9rem;
  }

  .timeline-header-actions {
    width: 100%;
    flex-wrap: nowrap;
    gap: 0.4rem;
  }

  .timeline-header-actions .form-select {
    flex: 0 0 auto;
    width: auto;
    min-width: 0;
    font-size: 0.8rem;
    padding-inline: 0.4rem;
  }

  .timeline-header-actions .btn-outline-secondary {
    flex: 0 0 auto;
  }

  .timeline-header-actions .btn:not(.btn-outline-secondary) {
    flex: 1 1 auto;
    font-size: 0.78rem;
    padding: 0.25rem 0.5rem;
  }

  .timeline-interval-label {
    display: none;
  }

  .timeline-controls {
    flex-direction: column;
    align-items: stretch;
    gap: 0.35rem;
  }

  .timeline-kpis {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.3rem;
  }

  .timeline-kpi-chip {
    padding: 0.3rem 0.45rem;
  }

  .timeline-kpi-label {
    font-size: 0.67rem;
  }

  .timeline-kpi-value {
    font-size: 0.85rem;
  }

  .timeline-controls-collapsible > summary {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.3rem 0;
    margin-bottom: 0.4rem;
    cursor: pointer;
    font-size: 0.78rem;
    color: var(--tblr-secondary);
    border-bottom: 1px solid var(--tblr-border-color);
    user-select: none;
    -webkit-tap-highlight-color: transparent;
  }

  .timeline-controls-collapsible:not([open]) .timeline-controls-toggle-arrow {
    transform: rotate(-90deg);
  }

  .timeline-frieze {
    padding: 0.5rem 0.75rem;
  }

  .timeline-frieze-item {
    min-width: 52px;
    padding: 0.15rem 0.2rem;
  }

  .timeline-frieze-dot {
    width: 11px;
    height: 11px;
    border-width: 2px;
  }

  .timeline-frieze-time,
  .timeline-frieze-count {
    font-size: 0.65rem;
  }

  .timeline-group-header {
    padding-top: 0.35rem;
    padding-bottom: 0.35rem;
  }

  .timeline-group-kpis .badge:nth-child(n+3) {
    display: none;
  }

  .timeline-group-events {
    gap: 0.6rem;
  }

  .timeline-status-group-grid {
    grid-template-columns: 1fr;
  }

  .timeline-event-card {
    padding: 0.4rem 0.5rem;
  }

  .timeline-event-topline {
    margin-bottom: 0.2rem;
  }

  .timeline-event-path {
    margin-bottom: 0.2rem;
  }

  .timeline-event-meta {
    gap: 0.15rem;
  }
}
</style>
