import { describe, it, expect, vi, afterEach } from 'vitest'
import { nextTick, ref } from 'vue'
import { useDockerContainerList, stateRank, PAGE_SIZE } from './useDockerContainerList'
import type { Container } from '../types/dockerContainersTab'

afterEach(() => vi.useRealTimers())

function c(over: Partial<Container> & { id: string }): Container {
  return { name: over.id, hostname: 'h1', host_id: 'h1', image: 'nginx', state: 'running', ...over }
}

describe('useDockerContainerList', () => {
  it('ranks states by operational severity, unknown states last', () => {
    expect(stateRank('running')).toBeLessThan(stateRank('exited'))
    expect(stateRank('exited')).toBeLessThan(stateRank('dead'))
    expect(stateRank('weird')).toBeGreaterThan(stateRank('dead'))
    expect(stateRank(undefined)).toBe(5)
  })

  it('sorts by state rank and flips on a second toggle', () => {
    const list = useDockerContainerList(ref([
      c({ id: 'a', state: 'dead' }), c({ id: 'b', state: 'running' }), c({ id: 'c', state: 'exited' }),
    ]))
    list.toggleSort('state')
    expect(list.sortedContainers.value.map((x) => x.id)).toEqual(['b', 'c', 'a'])
    list.toggleSort('state')
    expect(list.sortedContainers.value.map((x) => x.id)).toEqual(['a', 'c', 'b'])
  })

  it('sorts by image including the tag', () => {
    const list = useDockerContainerList(ref([
      c({ id: 'a', image: 'redis', image_tag: '7' }), c({ id: 'b', image: 'redis', image_tag: '6' }),
    ]))
    list.toggleSort('image')
    expect(list.sortedContainers.value.map((x) => x.id)).toEqual(['b', 'a'])
  })

  it('debounces the search and matches the compose project', async () => {
    vi.useFakeTimers()
    const list = useDockerContainerList(ref([
      c({ id: 'a', labels: { 'com.docker.compose.project': 'blog' } }), c({ id: 'b' }),
    ]))
    list.searchInput.value = 'blog'
    await nextTick()
    expect(list.sortedContainers.value).toHaveLength(2)
    vi.advanceTimersByTime(300)
    expect(list.sortedContainers.value.map((x) => x.id)).toEqual(['a'])
  })

  it('selects all on the current page only and clears the selection when a filter changes', async () => {
    const many = Array.from({ length: PAGE_SIZE + 5 }, (_, i) => c({ id: `c${String(i).padStart(2, '0')}`, name: `n${String(i).padStart(2, '0')}` }))
    const list = useDockerContainerList(ref(many))
    list.toggleSelectAll(true)
    expect(list.selectedIds.value.size).toBe(PAGE_SIZE)
    expect(list.allVisibleSelected.value).toBe(true)
    list.stateFilter.value = 'running'
    await nextTick()
    expect(list.selectedIds.value.size).toBe(0)
  })
})
