import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { setLocale } from '../i18n'

const api = vi.hoisted(() => ({
  getUptimeProbes: vi.fn(),
  getUptimeStats: vi.fn(),
  getUptimeHistory: vi.fn(),
  checkUptimeProbeNow: vi.fn(),
  createUptimeProbe: vi.fn(),
  updateUptimeProbe: vi.fn(),
  deleteUptimeProbe: vi.fn(),
}))
const npmApi = vi.hoisted(() => ({ updateProxyHost: vi.fn() }))
const confirm = vi.hoisted(() => vi.fn())

vi.mock('../api', () => ({ default: api }))
vi.mock('../api/npm', () => ({ npmApi }))
vi.mock('./useConfirmDialog', () => ({ useConfirmDialog: () => ({ confirm }) }))

import { useUptimeProbes } from './useUptimeProbes'

const serverError = (msg: string) => ({ isAxiosError: true, message: 'Request failed', response: { data: { error: msg } } })

let wrapper: VueWrapper | undefined
function mountProbes() {
  let result!: ReturnType<typeof useUptimeProbes>
  wrapper = mount(defineComponent({
    setup() {
      result = useUptimeProbes({ withStats: false })
      return () => h('div')
    },
  }))
  return result
}

const probe = { id: 'p1', name: 'api', type: 'http', target: 'https://example.com', enabled: true } as never
const npmProbe = { ...(probe as object), npm_proxy_host_id: 3, npm_proxy_host_domain: 'example.com' } as never

describe('useUptimeProbes', () => {
  beforeEach(() => {
    setLocale('fr')
    for (const fn of [...Object.values(api), npmApi.updateProxyHost, confirm]) fn.mockReset()
    api.getUptimeProbes.mockResolvedValue({ data: { probes: [] } })
    api.getUptimeHistory.mockResolvedValue({ data: [] })
    confirm.mockResolvedValue(true)
  })

  afterEach(() => wrapper?.unmount())

  it('shows the server message when the list fails to load', async () => {
    api.getUptimeProbes.mockRejectedValue(serverError('base indisponible'))
    const probes = mountProbes()
    await flushPromises()
    expect(probes.error.value).toBe('base indisponible')
  })

  it('shows the translated fallback instead of axios jargon on a network failure', async () => {
    api.getUptimeProbes.mockRejectedValue({ isAxiosError: true, message: 'Network Error' })
    const probes = mountProbes()
    await flushPromises()
    expect(probes.error.value).not.toBe('')
    expect(probes.error.value).not.toContain('Network Error')
  })

  it('reports a failed manual check and clears the in-flight marker', async () => {
    const probes = mountProbes()
    await flushPromises()
    api.checkUptimeProbeNow.mockRejectedValue(serverError('cible injoignable'))
    await probes.checkProbeNow(probe)
    expect(probes.error.value).toBe('cible injoignable')
    expect(probes.checkingProbeId.value).toBe('')
  })

  it('keeps the form open with the error when saving fails', async () => {
    const probes = mountProbes()
    await flushPromises()
    probes.openCreateProbe()
    api.createUptimeProbe.mockRejectedValue(serverError('URL invalide'))
    await probes.saveProbe()
    expect(probes.probeFormError.value).toBe('URL invalide')
    expect(probes.probeModalOpen.value).toBe(true)
    expect(probes.savingProbe.value).toBe(false)
  })

  it('turns the NPM monitoring flag off before deleting an NPM-linked probe', async () => {
    const probes = mountProbes()
    await flushPromises()
    await probes.confirmDeleteProbe(npmProbe)
    expect(npmApi.updateProxyHost).toHaveBeenCalledWith(3, { uptime_monitoring_enabled: false })
    expect(api.deleteUptimeProbe).toHaveBeenCalledWith('p1')
  })

  it('reports a failed deletion', async () => {
    const probes = mountProbes()
    await flushPromises()
    api.deleteUptimeProbe.mockRejectedValue(serverError('suppression refusée'))
    await probes.confirmDeleteProbe(probe)
    expect(probes.error.value).toBe('suppression refusée')
  })
})
