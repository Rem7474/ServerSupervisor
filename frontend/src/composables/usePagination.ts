import { computed, isRef, ref, watch, Ref, ComputedRef } from 'vue'

type RefLike<T> = { value: T } | Ref<T>

interface PaginationOptions<T> {
  items?: T[] | RefLike<T[]>
  pageSize?: number
  initialPage?: number
}

interface PaginationApi<T> {
  currentPage: Ref<number>
  totalItems: ComputedRef<number>
  totalPages: ComputedRef<number>
  pagedItems: ComputedRef<T[]>
  nextPage: () => void
  prevPage: () => void
  resetPage: () => void
  setPage: (page: number) => void
}

interface RemotePaginationApi {
  nextPage: () => void
  prevPage: () => void
}

export function usePagination<T>({ items, pageSize = 10, initialPage = 1 }: PaginationOptions<T> = {}): PaginationApi<T> {
  const currentPage: Ref<number> = ref(initialPage)
  const safePageSize = Math.max(1, pageSize)

  const sourceItems: ComputedRef<T[]> = computed(() => {
    if (!items) return []
    return isRef(items) ? (items as any).value : items
  })

  const totalItems: ComputedRef<number> = computed(() => sourceItems.value.length)
  const totalPages: ComputedRef<number> = computed(() => Math.max(1, Math.ceil(totalItems.value / safePageSize)))

  watch(totalPages, (maxPage) => {
    if (currentPage.value > maxPage) currentPage.value = maxPage
  })

  const pagedItems: ComputedRef<T[]> = computed(() => {
    const start = (currentPage.value - 1) * safePageSize
    return sourceItems.value.slice(start, start + safePageSize)
  })

  function nextPage(): void {
    if (currentPage.value < totalPages.value) currentPage.value += 1
  }

  function prevPage(): void {
    if (currentPage.value > 1) currentPage.value -= 1
  }

  function resetPage(): void {
    currentPage.value = 1
  }

  function setPage(page: number): void {
    currentPage.value = Math.min(Math.max(1, page), totalPages.value)
  }

  return {
    currentPage,
    totalItems,
    totalPages,
    pagedItems,
    nextPage,
    prevPage,
    resetPage,
    setPage,
  }
}

interface RemotePaginationOptions {
  currentPage: RefLike<number>
  totalPages: RefLike<number>
}

export function useRemotePagination({ currentPage, totalPages }: RemotePaginationOptions): RemotePaginationApi {
  const safeCurrentPage = isRef(currentPage) ? currentPage : (currentPage as { value: number })
  const safeTotalPages = isRef(totalPages) ? totalPages : (totalPages as { value: number })

  function nextPage(): void {
    if (safeCurrentPage.value < safeTotalPages.value) safeCurrentPage.value += 1
  }

  function prevPage(): void {
    if (safeCurrentPage.value > 1) safeCurrentPage.value -= 1
  }

  return { nextPage, prevPage }
}
