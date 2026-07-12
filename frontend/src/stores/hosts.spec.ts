import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { Host } from '../types/host'

vi.mock('../api', () => ({
  default: {
    getHosts: vi.fn(),
  },
}))

import apiClient from '../api'
import { useHostsStore } from './hosts'

function makeHost(overrides: Partial<Host> = {}): Host {
  return {
    id: 'h1',
    name: 'host-1',
    hostname: 'host-1',
    ip_address: '10.0.0.1',
    status: 'online',
    ...overrides,
  } as Host
}

describe('stores/hosts', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('fetches hosts and marks the cache fresh', async () => {
    const hosts = [makeHost()]
    vi.mocked(apiClient.getHosts).mockResolvedValue({ data: hosts } as never)

    const store = useHostsStore()
    await store.fetchHosts()

    expect(apiClient.getHosts).toHaveBeenCalledTimes(1)
    expect(store.hosts).toEqual(hosts)
  })

  it('skips the network call when the TTL has not expired', async () => {
    vi.mocked(apiClient.getHosts).mockResolvedValue({ data: [makeHost()] } as never)
    const store = useHostsStore()

    await store.fetchHosts()
    await store.fetchHosts()

    expect(apiClient.getHosts).toHaveBeenCalledTimes(1)
  })

  it('re-fetches once the TTL (60s) has expired', async () => {
    vi.useFakeTimers()
    vi.mocked(apiClient.getHosts).mockResolvedValue({ data: [makeHost()] } as never)
    const store = useHostsStore()

    await store.fetchHosts()
    vi.advanceTimersByTime(60_001)
    await store.fetchHosts()

    expect(apiClient.getHosts).toHaveBeenCalledTimes(2)
  })

  it('force-refetches even when the cache is still fresh', async () => {
    vi.mocked(apiClient.getHosts).mockResolvedValue({ data: [makeHost()] } as never)
    const store = useHostsStore()

    await store.fetchHosts()
    await store.fetchHosts(true)

    expect(apiClient.getHosts).toHaveBeenCalledTimes(2)
  })

  it('keeps the stale list and does not throw when the fetch fails', async () => {
    const hosts = [makeHost()]
    vi.mocked(apiClient.getHosts)
      .mockResolvedValueOnce({ data: hosts } as never)
      .mockRejectedValueOnce(new Error('network down'))
    const store = useHostsStore()

    await store.fetchHosts()
    await store.fetchHosts(true)

    expect(store.hosts).toEqual(hosts)
    expect(store.loading).toBe(false)
  })

  it('setHosts replaces the list and marks the cache fresh (skips the next fetch)', async () => {
    const store = useHostsStore()
    const pushed = [makeHost({ id: 'h2', status: 'offline' })]

    store.setHosts(pushed)
    await store.fetchHosts()

    expect(store.hosts).toEqual(pushed)
    expect(apiClient.getHosts).not.toHaveBeenCalled()
  })

  it('invalidate forces the next fetchHosts to hit the network again', async () => {
    vi.mocked(apiClient.getHosts).mockResolvedValue({ data: [makeHost()] } as never)
    const store = useHostsStore()

    await store.fetchHosts()
    store.invalidate()
    await store.fetchHosts()

    expect(apiClient.getHosts).toHaveBeenCalledTimes(2)
  })

  it('upsert updates an existing host in place without duplicating it', () => {
    const store = useHostsStore()
    store.setHosts([makeHost({ id: 'h1', status: 'online' }), makeHost({ id: 'h2', status: 'online' })])

    store.upsert(makeHost({ id: 'h1', status: 'offline' }))

    expect(store.hosts).toHaveLength(2)
    expect(store.hosts.find((h) => h.id === 'h1')?.status).toBe('offline')
  })

  it('upsert appends the host when it is not already in the list', () => {
    const store = useHostsStore()
    store.setHosts([makeHost({ id: 'h1' })])

    store.upsert(makeHost({ id: 'h3' }))

    expect(store.hosts.map((h) => h.id)).toEqual(['h1', 'h3'])
  })

  it('remove drops the host by id and leaves the rest untouched', () => {
    const store = useHostsStore()
    store.setHosts([makeHost({ id: 'h1' }), makeHost({ id: 'h2' })])

    store.remove('h1')

    expect(store.hosts.map((h) => h.id)).toEqual(['h2'])
  })
})
