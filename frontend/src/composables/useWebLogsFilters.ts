import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import type { TimeRangeModel } from '../types/timeRange'

/**
 * Filter state shared by the Traffic and Threats pages (period or custom
 * range, log source, host), mirrored in the URL query so a refresh or a
 * shared link keeps the filter.
 */
export function useWebLogsFilters() {
  const route = useRoute()
  const router = useRouter()
  const { t } = useI18n()

  const query = (key: string): string | null => (typeof route.query[key] === 'string' ? route.query[key] as string : null)

  const period = ref(query('period') ?? '24h')
  const source = ref(query('source') ?? '')
  const hostId = ref(query('host_id') ?? '')
  const from = ref<string | null>(query('from'))
  const to = ref<string | null>(query('to'))
  const timeRange = ref<TimeRangeModel>({
    mode: from.value && to.value ? 'custom' : 'preset',
    period: period.value,
    from: from.value,
    to: to.value,
  })

  const periodOptions = computed(() => [
    { value: '1h', label: '1h' },
    { value: '24h', label: '24h' },
    { value: '168h', label: t('security.period7dLabel') },
    { value: '720h', label: t('security.period30dLabel') },
  ])

  watch([period, source, hostId, from, to], ([p, s, h, f, toVal]) => {
    router.replace({ query: { ...route.query, period: p, source: s || undefined, host_id: h || undefined, from: f || undefined, to: toVal || undefined } })
  })

  /** Applies the time-range picker's value to the period/from/to filters. */
  function applyTimeRange(): void {
    if (timeRange.value.mode === 'custom' && timeRange.value.from && timeRange.value.to) {
      from.value = timeRange.value.from
      to.value = timeRange.value.to
    } else {
      period.value = timeRange.value.period
      from.value = null
      to.value = null
    }
  }

  /** The current filters in the shape the domain / IP detail endpoints take. */
  function detailFilters() {
    return {
      period: period.value,
      hostId: hostId.value || undefined,
      source: source.value || undefined,
      from: from.value || undefined,
      to: to.value || undefined,
    }
  }

  return { period, source, hostId, from, to, timeRange, periodOptions, applyTimeRange, detailFilters }
}
