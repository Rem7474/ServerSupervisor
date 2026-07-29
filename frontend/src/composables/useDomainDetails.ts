import { computed, reactive, ref } from 'vue'
import api from '../api'
import { getApiErrorMessage } from '../api/client'
import type { DomainDetailsParams } from '../types/security'

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- GetDomainDetails is an ad-hoc server-side aggregate (map[string]any), no Go model to type against — see types/security.ts
type AnyRecord = Record<string, any>

export type DomainDetailsSortKey = 'time' | 'status' | 'bytes'
export type DomainDetailsSortDir = 'asc' | 'desc'
export type DomainDetailsFilterKey = 'status' | 'method' | 'path' | 'ip'

const PAGE_SIZE = 25

// Shared behind the "Détails domaine" drawer used by ExposureDomainsPanel,
// TrafficOverviewPanel and ThreatsPanel (via useBot.ts) — each of those used
// to hand-roll its own copy of this same show/loading/details/open/close
// state and a fixed 300-row, unfilterable fetch. Centralizing it here means
// the interactive filter/sort/pagination behavior only has to be written
// (and fixed) once.
export function useDomainDetails() {
  const show = ref(false)
  const domain = ref('')
  const period = ref('24h')
  const loading = ref(false)
  const error = ref('')
  const details = ref<AnyRecord>({})

  // Kept as plain strings (not DomainDetailsParams['status']'s narrower
  // union) so setFilter can stay one generic function for all four keys;
  // narrowed once at the API call boundary below.
  const filters = reactive({
    status: '',
    method: '',
    path: '',
    ip: '',
  })
  const sortKey = ref<DomainDetailsSortKey>('time')
  const sortDir = ref<DomainDetailsSortDir>('desc')
  const page = ref(1)

  const totalPages = computed(() => {
    const total = Number(details.value?.total ?? 0)
    return Math.max(1, Math.ceil(total / PAGE_SIZE))
  })
  const hasActiveFilters = computed(() =>
    !!(filters.status || filters.method || filters.path || filters.ip)
  )

  // Captured at open() from the caller's own current page-level filters
  // (Traffic/Threats' host/source selectors) — not reactive to them changing
  // while the drawer is open, same as everything else here resetting on open.
  let context: { hostId?: string; source?: string } = {}

  async function load(): Promise<void> {
    if (!domain.value) return
    loading.value = true
    error.value = ''
    try {
      const res = await api.getDomainDetails(domain.value, period.value, {
        hostId: context.hostId,
        source: context.source,
        page: page.value,
        limit: PAGE_SIZE,
        status: (filters.status || undefined) as DomainDetailsParams['status'],
        method: filters.method || undefined,
        path: filters.path || undefined,
        ip: filters.ip || undefined,
        sort: sortKey.value,
        dir: sortDir.value,
      })
      details.value = res.data?.details || {}
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e, 'Erreur de chargement')
      details.value = {}
    } finally {
      loading.value = false
    }
  }

  function open(domainName: string, opts: { period?: string; hostId?: string; source?: string } = {}): void {
    domain.value = domainName
    period.value = opts.period ?? '24h'
    context = { hostId: opts.hostId, source: opts.source }
    filters.status = ''
    filters.method = ''
    filters.path = ''
    filters.ip = ''
    sortKey.value = 'time'
    sortDir.value = 'desc'
    page.value = 1
    show.value = true
    void load()
  }

  function close(): void {
    show.value = false
    domain.value = ''
    details.value = {}
  }

  function setFilter(key: DomainDetailsFilterKey, value: string): void {
    filters[key] = value
    page.value = 1
    void load()
  }

  function clearFilters(): void {
    filters.status = ''
    filters.method = ''
    filters.path = ''
    filters.ip = ''
    page.value = 1
    void load()
  }

  function toggleSort(key: DomainDetailsSortKey): void {
    if (sortKey.value === key) {
      sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
    } else {
      sortKey.value = key
      sortDir.value = 'desc'
    }
    void load()
  }

  function setPage(p: number): void {
    if (p < 1 || p > totalPages.value) return
    page.value = p
    void load()
  }

  return {
    show,
    domain,
    period,
    loading,
    error,
    details,
    filters,
    sortKey,
    sortDir,
    page,
    pageSize: PAGE_SIZE,
    totalPages,
    hasActiveFilters,
    open,
    close,
    setFilter,
    clearFilters,
    toggleSort,
    setPage,
  }
}
