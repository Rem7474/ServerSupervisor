import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../i18n'

const { getProxmoxSummary, getProxmoxNodes, getProxmoxInstances } = vi.hoisted(() => ({
  getProxmoxSummary: vi.fn(),
  getProxmoxNodes: vi.fn(),
  getProxmoxInstances: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { getProxmoxSummary, getProxmoxNodes, getProxmoxInstances },
  isApiAbort: () => false,
}))

import { useProxmox } from './useProxmox'

function mountUseProxmox() {
  let api!: ReturnType<typeof useProxmox>
  const wrapper = mount({
    setup() {
      api = useProxmox()
      return () => null
    },
  })
  return { wrapper, api: api! }
}

describe('useProxmox — load() error handling', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    getProxmoxSummary.mockResolvedValue({ data: {} })
    getProxmoxNodes.mockResolvedValue({ data: [] })
    getProxmoxInstances.mockResolvedValue({ data: [] })
  })

  it('surfaces the server-provided message when the summary fetch fails with a non-403 error', async () => {
    getProxmoxSummary.mockRejectedValue({ response: { status: 500, data: { error: 'db down' } } })

    const { api } = mountUseProxmox()
    await flushPromises()

    expect(api.error.value).toBe('db down')
  })

  it('falls back to the translated generic error when the summary fetch fails without a response body', async () => {
    getProxmoxSummary.mockRejectedValue({ response: { status: 500 } })

    const { api } = mountUseProxmox()
    await flushPromises()

    expect(api.error.value).toBe('Erreur lors du chargement.')
  })

  it('silently ignores a 403 on the instances fetch (viewer without connection-management rights)', async () => {
    getProxmoxInstances.mockRejectedValue({ response: { status: 403 } })

    const { api } = mountUseProxmox()
    await flushPromises()

    expect(api.error.value).toBe('')
    expect(api.instances.value).toEqual([])
  })

  it('surfaces the translated generic error when the instances fetch fails with a non-403 error', async () => {
    getProxmoxInstances.mockRejectedValue({ response: { status: 500 } })

    const { api } = mountUseProxmox()
    await flushPromises()

    expect(api.error.value).toBe('Erreur lors du chargement.')
  })

  it('surfaces the translated generic error when the nodes fetch fails with a non-403 error', async () => {
    getProxmoxNodes.mockRejectedValue({ response: { status: 500 } })

    const { api } = mountUseProxmox()
    await flushPromises()

    expect(api.error.value).toBe('Erreur lors du chargement.')
    expect(api.sortedNodes.value).toEqual([])
  })
})
