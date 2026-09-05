import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'

const { getSSLCertificate, getSSLCertificateHistory } = vi.hoisted(() => ({
  getSSLCertificate: vi.fn(),
  getSSLCertificateHistory: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'cert-1' } }),
}))

vi.mock('../../api', () => ({
  default: { getSSLCertificate, getSSLCertificateHistory },
  isApiAbort: () => false,
}))

import SslDetailSection from './SslDetailSection.vue'

describe('SslDetailSection', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    setLocale('fr')
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.clearAllMocks()
  })

  it('renders the certificate details grid (subject/issuer/serial/SAN) once loaded', async () => {
    getSSLCertificate.mockResolvedValue({
      data: {
        id: 'cert-1',
        subject: 'CN=example.com,O=Example Corp',
        issuer: 'CN=Let\'s Encrypt R3,O=Let\'s Encrypt',
        serial_number: '03:AB:CD',
        dns_names: ['example.com', 'www.example.com'],
        days_remaining: 45,
        valid_from: '2026-07-01T00:00:00Z',
        valid_to: '2026-10-01T00:00:00Z',
        last_checked_at: '2026-08-24T10:00:00Z',
      },
    })
    getSSLCertificateHistory.mockResolvedValue({ data: { events: [] } })

    const wrapper = mount(SslDetailSection, { props: { certId: 'cert-1' } })
    await flushPromises()

    expect(wrapper.text()).toContain('example.com')
    expect(wrapper.text()).toContain('Let\'s Encrypt R3')
    expect(wrapper.text()).toContain('03:AB:CD')
    expect(wrapper.text()).toContain('www.example.com')
    // 45 days remaining -> "Valide" / text-success, per statusColor's thresholds.
    expect(wrapper.text()).toContain('Valide')
    expect(wrapper.text()).toContain('45 jours')
  })

  it('shows a dash for SAN/DNS when the certificate has no dns_names', async () => {
    getSSLCertificate.mockResolvedValue({
      data: { id: 'cert-1', subject: 'CN=example.com', issuer: 'CN=CA', dns_names: [], days_remaining: 3 },
    })
    getSSLCertificateHistory.mockResolvedValue({ data: { events: [] } })

    const wrapper = mount(SslDetailSection, { props: { certId: 'cert-1' } })
    await flushPromises()

    // 3 days remaining -> "Critique" (<=7).
    expect(wrapper.text()).toContain('Critique')
    const sanItem = wrapper.findAll('.datagrid-item').find((el) => el.text().includes('SAN / DNS'))
    expect(sanItem?.text()).toContain('—')
  })

  it('shows the verification-error card when the certificate has last_error set', async () => {
    getSSLCertificate.mockResolvedValue({
      data: { id: 'cert-1', subject: 'CN=x', issuer: 'CN=y', days_remaining: -2, last_error: 'connection refused' },
    })
    getSSLCertificateHistory.mockResolvedValue({ data: { events: [] } })

    const wrapper = mount(SslDetailSection, { props: { certId: 'cert-1' } })
    await flushPromises()

    // Expired (days_remaining < 0).
    expect(wrapper.text()).toContain('Expiré')
    expect(wrapper.text()).toContain('connection refused')
  })

  it('renders the renewal history table once events have loaded', async () => {
    getSSLCertificate.mockResolvedValue({
      data: { id: 'cert-1', subject: 'CN=x', issuer: 'CN=y', days_remaining: 10 },
    })
    getSSLCertificateHistory.mockResolvedValue({
      data: {
        events: [
          {
            id: 'ev-1',
            detected_at: '2026-08-20T00:00:00Z',
            valid_from: '2026-08-01T00:00:00Z',
            valid_to: '2027-08-01T00:00:00Z',
            issuer: 'CN=Let\'s Encrypt R3',
            serial_number: 'AA:BB',
          },
        ],
      },
    })

    const wrapper = mount(SslDetailSection, { props: { certId: 'cert-1' } })
    await flushPromises()

    expect(wrapper.text()).toContain('1 version détectée')
    expect(wrapper.text()).toContain('1 ans') // certDuration: 365 days -> "1 ans"
    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(1)
    expect(rows[0].text()).toContain('AA:BB')
  })

  it('shows the empty state when the certificate has no renewal history', async () => {
    getSSLCertificate.mockResolvedValue({
      data: { id: 'cert-1', subject: 'CN=x', issuer: 'CN=y', days_remaining: 10 },
    })
    getSSLCertificateHistory.mockResolvedValue({ data: { events: [] } })

    const wrapper = mount(SslDetailSection, { props: { certId: 'cert-1' } })
    await flushPromises()

    expect(wrapper.text()).toContain('Aucun renouvellement enregistré')
  })

  it('surfaces a fetch failure as the top-level error alert', async () => {
    getSSLCertificate.mockRejectedValue({ response: { data: { error: 'certificat introuvable' } } })
    getSSLCertificateHistory.mockResolvedValue({ data: { events: [] } })

    const wrapper = mount(SslDetailSection, { props: { certId: 'cert-1' } })
    await flushPromises()

    expect(wrapper.find('.alert-danger').exists()).toBe(true)
    expect(wrapper.text()).toContain('certificat introuvable')
  })

  it('re-emits the loaded certificate and exposes lastUpdatedAt for a merged refresh bar', async () => {
    getSSLCertificate.mockResolvedValue({
      data: { id: 'cert-1', subject: 'CN=x', issuer: 'CN=y', days_remaining: 10 },
    })
    getSSLCertificateHistory.mockResolvedValue({ data: { events: [] } })

    const wrapper = mount(SslDetailSection, {
      props: { certId: 'cert-1', hideRefreshBar: true, autoRefresh: true },
    })
    await flushPromises()

    // watch(cert, ..., { immediate: true }) fires once synchronously with the
    // still-null initial value, then again once the fetch resolves.
    const loadedEmits = wrapper.emitted('loaded')
    expect(loadedEmits?.[loadedEmits.length - 1]?.[0]).toMatchObject({ id: 'cert-1' })
    // hideRefreshBar suppresses this component's own PageRefreshBar — the
    // parent (MonitoringHostDetailView) renders a single merged one instead.
    expect(wrapper.find('.page-refresh-bar').exists()).toBe(false)
    expect((wrapper.vm as unknown as { lastUpdatedAt: Date | null }).lastUpdatedAt).toBeInstanceOf(Date)
  })
})
