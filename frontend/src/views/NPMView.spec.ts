import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../i18n'

const { listAllProxyHosts } = vi.hoisted(() => ({ listAllProxyHosts: vi.fn() }))

vi.mock('../api/npm', () => ({
  npmApi: { listAllProxyHosts, setNPMEnabled: vi.fn(), updateProxyHost: vi.fn() },
}))

import NPMView from './NPMView.vue'

const mountOpts = {
  global: {
    stubs: { 'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' } },
  },
}

function host(overrides: Record<string, unknown> = {}) {
  return {
    id: 'h1', connection_name: 'main', domain_names: ['a.example.com'],
    forward_host: '10.0.0.1', forward_port: 8080, npm_enabled: true,
    uptime_monitoring_enabled: true, ssl_monitoring_enabled: true, ssl_enabled: true,
    ...overrides,
  }
}

describe('NPMView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    listAllProxyHosts.mockResolvedValue({ data: { proxy_hosts: [] } })
  })

  it('renders the translated header and empty state', async () => {
    const wrapper = mount(NPMView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('Dashboard')
    expect(wrapper.text()).toContain('Nginx Proxy Manager')
    expect(wrapper.text()).toContain('Proxy Hosts NPM')
    expect(wrapper.text()).toContain('Tous les proxy hosts')
    expect(wrapper.text()).toContain('Aucun proxy host trouvé')
    expect(wrapper.text()).toContain('Paramètres → Intégrations')
  })

  it('renders the translated table headers and expiring-certificate banner', async () => {
    listAllProxyHosts.mockResolvedValue({
      data: { proxy_hosts: [host({ ssl_certificate_id: 'c1', ssl_days_remaining: 5 })] },
    })
    const wrapper = mount(NPMView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('1 certificat expirant sous 30 jours')
    expect(wrapper.text()).toContain('Connexion')
    expect(wrapper.text()).toContain('Domaine')
    expect(wrapper.text()).toContain('Actif NPM')
    expect(wrapper.text()).toContain('5j')
  })

  it('pluralizes the expiring-certificates banner for 2+ certs', async () => {
    listAllProxyHosts.mockResolvedValue({
      data: {
        proxy_hosts: [
          host({ id: 'h1', ssl_certificate_id: 'c1', ssl_days_remaining: 5 }),
          host({ id: 'h2', ssl_certificate_id: 'c2', ssl_days_remaining: 10 }),
        ],
      },
    })
    const wrapper = mount(NPMView, mountOpts)
    await flushPromises()
    expect(wrapper.text()).toContain('2 certificats expirant sous 30 jours')
  })

  it('translates to English when the locale is switched', async () => {
    setLocale('en')
    const wrapper = mount(NPMView, mountOpts)
    await flushPromises()

    expect(wrapper.text()).toContain('NPM Proxy Hosts')
    expect(wrapper.text()).toContain('All proxy hosts')
    expect(wrapper.text()).toContain('No proxy host found')
  })
})
