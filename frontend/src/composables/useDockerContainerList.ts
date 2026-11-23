import { computed, ref, watch, type Ref } from 'vue'
import { usePagination } from './usePagination'
import { getComposeInfo, isComposeContainer } from '../utils/dockerCompose'
import type { Container } from '../types/dockerContainersTab'

export const PAGE_SIZE = 25

// Groups states by operational severity rather than sorting alphabetically
// (which interleaved "created"/"dead"/"exited" with no meaningful order).
const STATE_RANK: Record<string, number> = {
  running: 0,
  restarting: 1,
  paused: 1,
  created: 2,
  exited: 3,
  dead: 4,
  removing: 4,
}

export function stateRank(state: string | undefined): number {
  return STATE_RANK[state || ''] ?? 5
}

/**
 * Filtering (debounced search + host/state/compose selects), sorting, paging
 * and row selection for the containers table.
 */
export function useDockerContainerList(containers: Ref<Container[]>) {
  const searchInput = ref('')
  const search = ref('')
  let searchDebounce: ReturnType<typeof setTimeout> | null = null
  watch(searchInput, (val) => {
    if (searchDebounce) clearTimeout(searchDebounce)
    searchDebounce = setTimeout(() => { search.value = val }, 300)
  })
  const stateFilter = ref('')
  const hostFilter = ref('')
  const composeFilter = ref('')
  const sortBy = ref<keyof Container>('hostname')
  const sortDir = ref<'asc' | 'desc'>('asc')

  function toggleSort(key: keyof Container): void {
    if (sortBy.value === key) {
      sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
      return
    }
    sortBy.value = key
    sortDir.value = 'asc'
  }

  const filteredContainers = computed(() => {
    return containers.value.filter((c) => {
      const matchSearch = !search.value ||
        c.name?.toLowerCase().includes(search.value.toLowerCase()) ||
        c.image?.toLowerCase().includes(search.value.toLowerCase()) ||
        getComposeInfo(c.labels).project?.toLowerCase().includes(search.value.toLowerCase())
      const matchState = !stateFilter.value || c.state === stateFilter.value
      const matchCompose = !composeFilter.value ||
        (composeFilter.value === 'compose' && isComposeContainer(c.labels)) ||
        (composeFilter.value === 'standalone' && !isComposeContainer(c.labels))
      const matchHost = !hostFilter.value || c.hostname === hostFilter.value
      return matchSearch && matchState && matchCompose && matchHost
    })
  })

  const sortedContainers = computed(() => {
    const dir = sortDir.value === 'asc' ? 1 : -1
    const sorted = [...filteredContainers.value]
    sorted.sort((a, b) => {
      if (sortBy.value === 'state') {
        return (stateRank(a.state) - stateRank(b.state)) * dir
      }

      let av: unknown = a[sortBy.value]
      let bv: unknown = b[sortBy.value]

      if (sortBy.value === 'image') {
        av = `${a.image || ''}:${a.image_tag || ''}`
        bv = `${b.image || ''}:${b.image_tag || ''}`
      }

      const aVal = String(av || '').toLowerCase()
      const bVal = String(bv || '').toLowerCase()
      return aVal.localeCompare(bVal) * dir
    })
    return sorted
  })

  const uniqueHosts = computed(() => {
    const seen = new Set<string>()
    return containers.value
      .filter((c) => { if (!c.hostname || seen.has(c.hostname)) return false; seen.add(c.hostname); return true })
      .map((c) => c.hostname!)
      .sort()
  })

  const { currentPage, totalPages, pagedItems: pagedContainers, resetPage, setPage } = usePagination({
    items: sortedContainers,
    pageSize: PAGE_SIZE,
  })

  const selectedIds = ref<Set<string>>(new Set())

  function toggleSelected(id: string, checked: boolean): void {
    const next = new Set(selectedIds.value)
    if (checked) next.add(id)
    else next.delete(id)
    selectedIds.value = next
  }

  // Scoped to the current page, matching the checkbox's own aria-label
  // (docker.selectAllVisibleAriaLabel) — selecting "all" used to silently span
  // every page of the current filter, which made a bulk stop/restart much
  // wider than what the screen showed.
  const allVisibleSelected = computed(() =>
    pagedContainers.value.length > 0 && pagedContainers.value.every((c) => selectedIds.value.has(c.id))
  )

  function toggleSelectAll(checked: boolean): void {
    const next = new Set(selectedIds.value)
    for (const c of pagedContainers.value) {
      if (checked) next.add(c.id)
      else next.delete(c.id)
    }
    selectedIds.value = next
  }

  function clearSelection(): void {
    selectedIds.value = new Set()
  }

  /** Selected containers in display order (across pages of the current filter). */
  function selectedContainers(): Container[] {
    return sortedContainers.value.filter((c) => selectedIds.value.has(c.id))
  }

  watch([search, stateFilter, hostFilter, composeFilter], () => {
    resetPage()
    clearSelection()
  })

  return {
    searchInput,
    search,
    stateFilter,
    hostFilter,
    composeFilter,
    sortBy,
    sortDir,
    toggleSort,
    sortedContainers,
    uniqueHosts,
    currentPage,
    totalPages,
    pagedContainers,
    setPage,
    selectedIds,
    toggleSelected,
    allVisibleSelected,
    toggleSelectAll,
    clearSelection,
    selectedContainers,
  }
}
