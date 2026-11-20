import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { setLocale } from '../i18n'

const api = vi.hoisted(() => ({
  getSSLCertificates: vi.fn(),
  checkSSLCertificateNow: vi.fn(),
  createSSLCertificate: vi.fn(),
  updateSSLCertificate: vi.fn(),
  deleteSSLCertificate: vi.fn(),
}))
const npmApi = vi.hoisted(() => ({ updateProxyHost: vi.fn() }))
const confirm = vi.hoisted(() => vi.fn())

vi.mock('../api', () => ({ default: api }))
vi.mock('../api/npm', () => ({ npmApi }))
vi.mock('./useConfirmDialog', () => ({ useConfirmDialog: () => ({ confirm }) }))

import { useSslCertificates } from './useSslCertificates'

const serverError = (msg: string) => ({ isAxiosError: true, message: 'Request failed', response: { data: { error: msg } } })

let wrapper: VueWrapper | undefined
function mountCerts() {
  let result!: ReturnType<typeof useSslCertificates>
  wrapper = mount(defineComponent({
    setup() {
      result = useSslCertificates()
      return () => h('div')
    },
  }))
  return result
}

const cert = { id: 'c1', name: 'site', host: 'example.com', port: 443, enabled: true } as never
const npmCert = { ...(cert as object), npm_proxy_host_id: 7, npm_proxy_host_domain: 'example.com' } as never

describe('useSslCertificates', () => {
  beforeEach(() => {
    setLocale('fr')
    for (const fn of [...Object.values(api), npmApi.updateProxyHost, confirm]) fn.mockReset()
    api.getSSLCertificates.mockResolvedValue({ data: { certificates: [] } })
    confirm.mockResolvedValue(true)
  })

  afterEach(() => wrapper?.unmount())

  it('shows the server message when the list fails to load', async () => {
    api.getSSLCertificates.mockRejectedValue(serverError('base indisponible'))
    const certs = mountCerts()
    await flushPromises()
    expect(certs.error.value).toBe('base indisponible')
    expect(certs.loadingCerts.value).toBe(false)
  })

  it('reports a failed manual check and clears the in-flight marker', async () => {
    const certs = mountCerts()
    await flushPromises()
    api.checkSSLCertificateNow.mockRejectedValue(serverError('hôte injoignable'))
    await certs.checkCertNow(cert)
    expect(certs.error.value).toBe('hôte injoignable')
    expect(certs.checkingCertId.value).toBe('')
  })

  it('keeps the form open with the error when saving fails', async () => {
    const certs = mountCerts()
    await flushPromises()
    certs.openCreateCert()
    api.createSSLCertificate.mockRejectedValue(serverError('port invalide'))
    await certs.saveCert()
    expect(certs.certFormError.value).toBe('port invalide')
    expect(certs.certModalOpen.value).toBe(true)
    expect(certs.savingCert.value).toBe(false)
  })

  it('turns the NPM monitoring flag off before deleting an NPM-linked certificate', async () => {
    const certs = mountCerts()
    await flushPromises()
    await certs.confirmDeleteCert(npmCert)
    expect(npmApi.updateProxyHost).toHaveBeenCalledWith(7, { ssl_monitoring_enabled: false })
    expect(api.deleteSSLCertificate).toHaveBeenCalledWith('c1')
    expect(npmApi.updateProxyHost.mock.invocationCallOrder[0]).toBeLessThan(api.deleteSSLCertificate.mock.invocationCallOrder[0])
  })

  it('does nothing when the deletion is not confirmed', async () => {
    confirm.mockResolvedValue(false)
    const certs = mountCerts()
    await flushPromises()
    await certs.confirmDeleteCert(npmCert)
    expect(npmApi.updateProxyHost).not.toHaveBeenCalled()
    expect(api.deleteSSLCertificate).not.toHaveBeenCalled()
  })

  it('reports a failed deletion', async () => {
    const certs = mountCerts()
    await flushPromises()
    api.deleteSSLCertificate.mockRejectedValue(serverError('suppression refusée'))
    await certs.confirmDeleteCert(cert)
    expect(certs.error.value).toBe('suppression refusée')
  })
})
