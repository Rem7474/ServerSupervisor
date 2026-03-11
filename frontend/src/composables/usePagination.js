import { computed, isRef, ref, watch } from 'vue'

/**
 * @template T
 * @typedef {{ value: T }} RefLike
 */

/**
 * @template T
 * @typedef {{
 *  items?: T[] | RefLike<T[]>,
 *  pageSize?: number,
 *  initialPage?: number,
 * }} PaginationOptions
 */

/**
 * @template T
 * @param {PaginationOptions<T>} [options]
 */
export function usePagination({ items, pageSize = 10, initialPage = 1 } = {}) {
  const currentPage = ref(initialPage)
  const safePageSize = Math.max(1, pageSize)

  const sourceItems = computed(() => {
    if (!items) return []
    return isRef(items) ? items.value : items
  })

  const totalItems = computed(() => sourceItems.value.length)
  const totalPages = computed(() => Math.max(1, Math.ceil(totalItems.value / safePageSize)))

  watch(totalPages, (maxPage) => {
    if (currentPage.value > maxPage) currentPage.value = maxPage
  })

  const pagedItems = computed(() => {
    const start = (currentPage.value - 1) * safePageSize
    return sourceItems.value.slice(start, start + safePageSize)
  })

  function nextPage() {
    if (currentPage.value < totalPages.value) currentPage.value += 1
  }

  function prevPage() {
    if (currentPage.value > 1) currentPage.value -= 1
  }

  function resetPage() {
    currentPage.value = 1
  }

  function setPage(page) {
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

/**
 * @param {{ currentPage: RefLike<number>, totalPages: RefLike<number> }} options
 */
export function useRemotePagination({ currentPage, totalPages }) {
  function nextPage() {
    if (currentPage.value < totalPages.value) currentPage.value += 1
  }

  function prevPage() {
    if (currentPage.value > 1) currentPage.value -= 1
  }

  return { nextPage, prevPage }
}