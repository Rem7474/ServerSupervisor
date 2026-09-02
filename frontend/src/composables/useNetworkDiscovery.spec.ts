import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises } from '@vue/test-utils'

const { discoverHosts, registerHostsBulk } = vi.hoisted(() => ({
  discoverHosts: vi.fn(),
  registerHostsBulk: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { discoverHosts, registerHostsBulk },
}))
vi.mock('../api/client', () => ({
  getApiErrorMessage: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}))

import { useNetworkDiscovery } from './useNetworkDiscovery'

const RESULTS = [
  { ip_address: '10.0.0.1', responded: true, latency_ms: 3, already_registered: false },
  {
    ip_address: '10.0.0.2',
    responded: true,
    latency_ms: 1,
    already_registered: true,
    existing_host_id: 'h1',
    existing_host_name: 'web',
  },
  { ip_address: '10.0.0.3', responded: false, already_registered: false },
]

describe('useNetworkDiscovery', () => {
  beforeEach(() => {
    discoverHosts.mockReset()
    registerHostsBulk.mockReset()
  })

  it('scans and separates new/reachable candidates from registered/unreachable ones', async () => {
    discoverHosts.mockResolvedValue({ data: { results: RESULTS } })
    const disco = useNetworkDiscovery()
    disco.cidr.value = '10.0.0.0/29'
    await disco.scan()
    await flushPromises()

    expect(discoverHosts).toHaveBeenCalledWith('10.0.0.0/29')
    expect(disco.hasScanned.value).toBe(true)
    expect(disco.results.value).toHaveLength(3)
    expect(disco.candidates.value.map((c) => c.ip_address)).toEqual(['10.0.0.1'])
    expect(disco.names['10.0.0.1']).toBe('10.0.0.1')
  })

  it('surfaces the API error message on a failed scan', async () => {
    discoverHosts.mockRejectedValue(new Error('boom'))
    const disco = useNetworkDiscovery()
    disco.cidr.value = '10.0.0.0/29'
    await disco.scan()
    await flushPromises()

    expect(disco.scanError.value).toBe('boom')
    expect(disco.results.value).toEqual([])
  })

  it('toggles selection and select-all only over candidates', async () => {
    discoverHosts.mockResolvedValue({
      data: {
        results: [
          { ip_address: '10.0.0.1', responded: true, already_registered: false },
          { ip_address: '10.0.0.2', responded: true, already_registered: false },
        ],
      },
    })
    const disco = useNetworkDiscovery()
    disco.cidr.value = '10.0.0.0/29'
    await disco.scan()
    await flushPromises()

    expect(disco.allSelected.value).toBe(false)
    disco.toggleSelectAll()
    expect(disco.selected.value.size).toBe(2)
    expect(disco.allSelected.value).toBe(true)
    disco.toggleSelected('10.0.0.1')
    expect(disco.selected.value.has('10.0.0.1')).toBe(false)
    expect(disco.allSelected.value).toBe(false)
  })

  it('bulk-adds only the selected candidates with their (possibly edited) names', async () => {
    discoverHosts.mockResolvedValue({ data: { results: RESULTS } })
    registerHostsBulk.mockResolvedValue({
      data: {
        created: 1,
        results: [
          { name: 'switch-1', ip_address: '10.0.0.1', created: true, host_id: 'h9', api_key: 'h9.secret' },
        ],
      },
    })
    const disco = useNetworkDiscovery()
    disco.cidr.value = '10.0.0.0/29'
    await disco.scan()
    await flushPromises()

    disco.names['10.0.0.1'] = 'switch-1'
    disco.toggleSelected('10.0.0.1')
    await disco.addSelected()
    await flushPromises()

    expect(registerHostsBulk).toHaveBeenCalledWith([{ name: 'switch-1', ip_address: '10.0.0.1' }])
    expect(disco.bulkResults.value).toEqual([
      { name: 'switch-1', ip_address: '10.0.0.1', created: true, host_id: 'h9', api_key: 'h9.secret' },
    ])
  })

  it('does nothing when adding with an empty selection', async () => {
    const disco = useNetworkDiscovery()
    await disco.addSelected()
    expect(registerHostsBulk).not.toHaveBeenCalled()
  })

  it('surfaces the API error message on a failed bulk add', async () => {
    discoverHosts.mockResolvedValue({ data: { results: RESULTS } })
    registerHostsBulk.mockRejectedValue(new Error('bulk add failed'))
    const disco = useNetworkDiscovery()
    disco.cidr.value = '10.0.0.0/29'
    await disco.scan()
    await flushPromises()
    disco.toggleSelected('10.0.0.1')

    await disco.addSelected()
    await flushPromises()

    expect(disco.addError.value).toBe('bulk add failed')
    expect(disco.adding.value).toBe(false)
  })

  it('resets all state back to its initial values', async () => {
    discoverHosts.mockResolvedValue({ data: { results: RESULTS } })
    const disco = useNetworkDiscovery()
    disco.cidr.value = '10.0.0.0/29'
    await disco.scan()
    await flushPromises()
    disco.toggleSelected('10.0.0.1')

    disco.reset()

    expect(disco.cidr.value).toBe('')
    expect(disco.results.value).toEqual([])
    expect(disco.hasScanned.value).toBe(false)
    expect(disco.selected.value.size).toBe(0)
    expect(disco.names).toEqual({})
    expect(disco.bulkResults.value).toBeNull()
    expect(disco.addError.value).toBe('')
    expect(disco.scanError.value).toBe('')
  })
})
