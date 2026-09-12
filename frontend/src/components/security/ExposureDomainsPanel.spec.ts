import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import ExposureDomainsPanel from './ExposureDomainsPanel.vue'
import type { HostExposure } from '../../types/host'
import type { UptimeProbe } from '../../types/uptime'
import type { SSLCertificate } from '../../types/ssl'

const { probesRef, certsRef } = vi.hoisted(() => ({
  probesRef: { value: [] as UptimeProbe[] },
  certsRef: { value: [] as SSLCertificate[] },
}))

vi.mock('../../composables/useUptimeProbes', () => ({
  useUptimeProbes: () => ({
    probes: probesRef,
    probeBadge: (p: UptimeProbe) => (p.last_status === 'up' ? 'bg-success-lt text-success' : 'bg-danger-lt text-danger'),
    probeStatusLabel: (p: UptimeProbe) => (p.last_status === 'up' ? 'UP' : 'DOWN'),
    probeHistory: {} as Record<string, { id: string | number; checked_at: string; success: boolean }[]>,
  }),
}))

vi.mock('../../composables/useSslCertificates', () => ({
  useSslCertificates: () => ({
    certs: certsRef,
    daysLabel: (d: number) => `${d}j`,
    daysBadge: () => 'bg-success-lt text-success',
  }),
}))

function makeExposure(domains: HostExposure['domains']): HostExposure {
  return {
    host_id: 'h1',
    ip_address: '10.0.0.5',
    since: new Date().toISOString(),
    domains,
    total_requests: 0,
    total_suspicious_requests: 0,
    total_blocked_requests: 0,
  }
}

// The default auto-stub for router-link doesn't render its slot content, so
// badges nested inside it (the whole point of this test) would never appear —
// stub it with a minimal component that does.
const RouterLinkStub = {
  props: ['to'],
  template: '<a :href="to"><slot /></a>',
}

function mountPanel(exposure: HostExposure | null) {
  return mount(ExposureDomainsPanel, {
    props: { exposure, loading: false, period: '24h', periodLabel: '24h', subjectLabel: 'cet hôte' },
    global: { stubs: { DomainDetailsModal: true, 'router-link': RouterLinkStub, RouterLink: RouterLinkStub } },
  })
}

describe('ExposureDomainsPanel — disponibilité column', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('shows a monitoring badge and detail link when the domain\'s NPM proxy host has a linked probe/cert', () => {
    probesRef.value = [{ id: 'p1', npm_proxy_host_id: 'proxy-1', last_status: 'up' } as UptimeProbe]
    certsRef.value = [{ id: 'c1', npm_proxy_host_id: 'proxy-1', days_remaining: 42 } as SSLCertificate]

    const wrapper = mountPanel(makeExposure([
      { proxy_host_id: 'proxy-1', connection_id: 'conn-1', connection_name: 'main', domain_name: 'app.example.com', forward_port: 8080, ssl_enabled: true, npm_enabled: true, requests: 10, bytes: 1000, errors_4xx: 0, errors_5xx: 0, suspicious_requests: 0, blocked_requests: 0 },
    ]))

    const link = wrapper.find('a[href="/monitoring/host/proxy-1"]')
    expect(link.exists()).toBe(true)
    expect(link.text()).toContain('UP')
    expect(link.text()).toContain('42j')
  })

  it('shows "—" for a domain with no linked probe or cert', () => {
    probesRef.value = []
    certsRef.value = []

    const wrapper = mountPanel(makeExposure([
      { proxy_host_id: 'proxy-2', connection_id: 'conn-1', connection_name: 'main', domain_name: 'unmonitored.example.com', forward_port: 80, ssl_enabled: false, npm_enabled: true, requests: 5, bytes: 500, errors_4xx: 0, errors_5xx: 0, suspicious_requests: 0, blocked_requests: 0 },
    ]))

    expect(wrapper.find('a[href^="/monitoring/host/"]').exists()).toBe(false)
    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(1)
    expect(rows[0].text()).toContain('—')
  })

  it('renders the translated KPI labels and table headers, and interpolates the subject in the empty state', () => {
    const wrapper = mountPanel(makeExposure([]))

    expect(wrapper.text()).toContain('Adresse IP')
    expect(wrapper.text()).toContain('Requêtes (24h)')
    expect(wrapper.text()).toContain('Suspectes')
    expect(wrapper.text()).toContain('Bloquées')
    expect(wrapper.text()).toContain('Aucun domaine NPM ne route vers cet hôte.')
  })

  it('shows the translated "Désactivé" badge for a domain whose NPM proxy host is disabled', () => {
    const wrapper = mountPanel(makeExposure([
      { proxy_host_id: 'proxy-3', connection_id: 'conn-1', connection_name: 'main', domain_name: 'off.example.com', forward_port: 80, ssl_enabled: false, npm_enabled: false, requests: 0, bytes: 0, errors_4xx: 0, errors_5xx: 0, suspicious_requests: 0, blocked_requests: 0 },
    ]))

    expect(wrapper.text()).toContain('Désactivé')
    for (const header of ['Domaine', 'Connexion NPM', 'Cible', 'Disponibilité', 'Volume', 'Erreurs 4xx/5xx']) {
      expect(wrapper.text()).toContain(header)
    }
  })
})
