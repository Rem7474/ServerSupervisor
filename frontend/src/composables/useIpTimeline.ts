import { ref, computed, watch, toValue, type MaybeRefOrGetter } from 'vue'

type AnyRecord = Record<string, any>

export interface TimelineBucket {
  key: string
  startMs: number
  endMs: number
  count: number
  errorCount: number
  label: string
  rangeLabel: string
  title: string
}

export interface TimelineStatusGroup {
  key: string
  label: string
  badgeClass: string
  count: number
  events: AnyRecord[]
}

export interface TimelineGroup {
  key: string
  label: string
  rangeLabel: string
  count: number
  errorCount: number
  uniquePaths: number
  uniqueVhosts: number
  statusGroups: TimelineStatusGroup[]
}

export const AUTO_BUCKET_TARGET = 10

export const timelineIntervalOptions = [
  { value: 'auto', label: 'Auto', ms: 0 },
  { value: '10s', label: '10s', ms: 10 * 1000 },
  { value: '30s', label: '30s', ms: 30 * 1000 },
  { value: '1m', label: '1m', ms: 60 * 1000 },
  { value: '5m', label: '5m', ms: 5 * 60 * 1000 },
  { value: '10m', label: '10m', ms: 10 * 60 * 1000 },
  { value: '30m', label: '30m', ms: 30 * 60 * 1000 },
  { value: '1h', label: '1h', ms: 60 * 60 * 1000 },
]

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

function formatBucketLabel(startMs: number, bucketMs: number): string {
  const d = new Date(startMs)
  if (bucketMs < 60 * 1000) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

/** Tailwind/Tabler badge class for an individual HTTP status code. */
export function statusClass(status: number): string {
  if (status >= 500 && status < 600) return 'bg-red-lt text-red'
  if (status >= 400 && status < 500) return 'bg-yellow-lt text-yellow'
  if (status >= 200 && status < 300) return 'bg-green-lt text-green'
  if (status >= 300 && status < 400) return 'bg-azure-lt text-azure'
  return 'bg-secondary-lt text-secondary'
}

export function formatDate(v: string): string {
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v || '-'
  return d.toLocaleString()
}

export function truncate(s: string, max: number): string {
  return s.length > max ? s.slice(0, max) + '…' : s
}

export function bucketToneClass(bucket: TimelineBucket): string {
  if (bucket.count <= 0) return 'is-calm'
  const errorRate = bucket.errorCount / bucket.count
  if (errorRate >= 0.5) return 'is-hot'
  if (errorRate >= 0.2) return 'is-warm'
  return 'is-calm'
}

/**
 * useIpTimeline encapsulates the timeline bucketing / grouping / stats logic for
 * IPTimelineModal: it turns a flat list of request rows into adaptive time
 * buckets, a selected-bucket focus mode, KPI stats and per-bucket status groups.
 *
 * @param timeline reactive source of request rows (each with timestamp/status/…)
 * @param show     reactive modal visibility — used to reset focus on open
 */
export function useIpTimeline(
  timeline: MaybeRefOrGetter<AnyRecord[]>,
  show: MaybeRefOrGetter<boolean>,
) {
  const selectedBucketKey = ref('')
  const bucketFilterEnabled = ref(true)
  const selectedInterval = ref('auto')

  const rows = computed(() => toValue(timeline) || [])

  const timelineChrono = computed(() =>
    [...rows.value].sort((a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()),
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
    const list = timelineRows.value
    return {
      total: list.length,
      errors: list.filter((r) => Number(r.status) >= 400).length,
      uniquePaths: new Set(list.map((r) => String(r.path || ''))).size,
      uniqueVhosts: new Set(list.map((r) => String(r.domain || '(unknown)'))).size,
    }
  })

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
        const groupRows = [...events].sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
        const errorCount = groupRows.filter((r) => Number(r.status) >= 400).length
        const uniquePaths = new Set(groupRows.map((r) => String(r.path || ''))).size
        const uniqueVhosts = new Set(groupRows.map((r) => String(r.domain || '(unknown)'))).size
        const start = Number(key)
        const end = start + timelineBucketMs.value
        const byStatusFamily = new Map<string, AnyRecord[]>()
        for (const row of groupRows) {
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
          count: groupRows.length,
          errorCount,
          uniquePaths,
          uniqueVhosts,
          statusGroups,
        }
      })
  })

  // Reset focus state when the modal is (re)opened.
  watch(() => toValue(show), (val) => {
    if (val) {
      bucketFilterEnabled.value = true
      selectedInterval.value = 'auto'
      selectedBucketKey.value = ''
    }
  })

  // Keep the selected bucket valid as buckets recompute (interval change…).
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

  // On new data, focus the most recent bucket.
  watch(() => toValue(timeline), (newTimeline) => {
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

  return {
    // state
    selectedInterval,
    selectedBucketKey,
    bucketFilterEnabled,
    // constants
    timelineIntervalOptions,
    AUTO_BUCKET_TARGET,
    // derived
    timelineBucketLabel,
    timelineBuckets,
    selectedBucket,
    timelineStats,
    timelineStatusBreakdown,
    groupedTimeline,
    // actions
    setTimelineInterval,
    selectBucket,
    toggleBucketFilter,
    // display helpers
    bucketToneClass,
    statusClass,
    formatDate,
    truncate,
  }
}
