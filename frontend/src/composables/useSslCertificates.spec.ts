import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { setLocale } from '../i18n'
import { useConfirmDialog } from './useConfirmDialog'

const {
  getSSLCertificates, checkSSLCertificateNow, createSSLCertificate,
  updateSSLCertificate, deleteSSLCertificate,
} = vi.hoisted(() => ({
  getSSLCertificates: vi.fn(),
  checkSSLCertificateNow: vi.fn(),
  createSSLCertificate: vi.fn(),
  updateSSLCertificate: vi.fn(),
  deleteSSLCertificate: vi.fn(),
}))

vi.mock('../api', () => ({
  default: {
    getSSLCertificates, checkSSLCertificateNow, createSSLCertificate,
    updateSSLCertificate, deleteSSLCertificate,
  },
}))

vi.mock('../api/npm', () => ({
  npmApi: { updateProxyHost: vi.fn() },
}))

import { useSslCertificates } from './useSslCertificates'

const cert = {
  id: 'c1', name: 'api.example.com', host: 'api.example.com', port: 443,
  enabled: true, days_remaining: 42,
}

// useI18n()/useConfirmDialog() both need an active component instance.
function mountHost() {
  let api: ReturnType<typeof useSslCertificates> | undefined
  const wrapper = mount(defineComponent({
    setup() {
      api = useSslCertificates()
      return () => h('div')
    },
  }))
  return { wrapper, api: api! }
}

describe('useSslCertificates', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    getSSLCertificates.mockResolvedValue({ data: { certificates: [cert] } })
  })

  it('falls back to the translated "unknown" label when days_remaining is null', async () => {
    const { api } = mountHost()
    await vi.waitFor(() => expect(api.certs.value).toHaveLength(1))
    expect(api.daysLabel(null)).toBe('Inconnu')
  })

  it('shows the translated "expired" label with the elapsed day count for a negative days_remaining', async () => {
    const { api } = mountHost()
    await vi.waitFor(() => expect(api.certs.value).toHaveLength(1))
    expect(api.daysLabel(-5)).toBe('Expiré (5j)')
  })

  it('surfaces the translated fallback error when the initial load fails', async () => {
    getSSLCertificates.mockRejectedValue(new Error('boom'))
    const { api } = mountHost()
    await vi.waitFor(() => expect(api.loadingCerts.value).toBe(false))
    expect(api.error.value).toBe('Impossible de charger les certificats')
  })

  it('surfaces the translated fallback error when a manual check fails', async () => {
    const { api } = mountHost()
    await vi.waitFor(() => expect(api.certs.value).toHaveLength(1))
    checkSSLCertificateNow.mockRejectedValue(new Error('boom'))

    await api.checkCertNow(cert as never)

    expect(api.error.value).toBe('Échec de la vérification')
  })

  it('surfaces the translated fallback error when saving a certificate fails', async () => {
    const { api } = mountHost()
    await vi.waitFor(() => expect(api.certs.value).toHaveLength(1))
    createSSLCertificate.mockRejectedValue(new Error('boom'))
    api.openCreateCert()

    await api.saveCert()

    expect(api.certFormError.value).toBe("Erreur lors de l'enregistrement")
    expect(api.savingCert.value).toBe(false)
  })

  it('surfaces the translated fallback error when deleting a certificate fails, after confirming', async () => {
    const { api } = mountHost()
    await vi.waitFor(() => expect(api.certs.value).toHaveLength(1))
    deleteSSLCertificate.mockRejectedValue(new Error('boom'))
    const dialog = useConfirmDialog()

    const deletePromise = api.confirmDeleteCert(cert as never)
    await vi.waitFor(() => expect(dialog.isOpen.value).toBe(true))
    dialog.onConfirm()
    await deletePromise

    expect(api.error.value).toBe('Suppression impossible')
  })
})
