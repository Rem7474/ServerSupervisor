import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import IPTimelineModal from './IPTimelineModal.vue'
import type { WebLogIPTimelineRow } from '../../types/security'

const rows: WebLogIPTimelineRow[] = [
  { timestamp: '2026-01-01T10:00:00Z', ip: '1.2.3.4', host_id: 'host-1', category: 'weblog', method: 'GET', path: '/admin', status: 404, bytes: 0, domain: 'app.example.com', host_name: 'web-01', user_agent: 'curl/8.0', blocked: false, source: 'weblog' },
  { timestamp: '2026-01-01T10:00:05Z', ip: '1.2.3.4', host_id: 'host-1', category: 'weblog', method: 'GET', path: '/wp-login.php', status: 200, bytes: 512, domain: 'app.example.com', host_name: 'web-01', user_agent: 'curl/8.0', blocked: true, blocked_source: 'crowdsec', blocked_reason: 'scanning', blocked_until: '2026-01-01T14:00:00Z', source: 'weblog' },
]

function mountModal(overrides: Record<string, unknown> = {}) {
  return mount(IPTimelineModal, {
    props: {
      show: true,
      ip: '1.2.3.4',
      timeline: rows,
      loading: false,
      blocked: false,
      banLoading: false,
      banError: false,
      hostId: 'host-1',
      readOnly: false,
      ...overrides,
    },
  })
}

beforeEach(() => {
  setLocale('fr')
})

describe('IPTimelineModal', () => {
  it('renders the translated title, subtitle and ban button', () => {
    const wrapper = mountModal()
    expect(wrapper.text()).toContain('Chronologie IP:')
    expect(wrapper.text()).toContain('1.2.3.4')
    expect(wrapper.text()).toContain('Chronologie des requêtes suspectes')
    expect(wrapper.text()).toContain('Bloquer (CrowdSec)')
    expect(wrapper.text()).toContain('Fermer')
  })

  it('shows the translated "blocked by CrowdSec" badge instead of the ban control once blocked', () => {
    const wrapper = mountModal({ blocked: true })
    expect(wrapper.text()).toContain('IP bloquée par CrowdSec')
    expect(wrapper.find('button.btn-outline-danger').exists()).toBe(false)
  })

  it('disables the ban button with a translated tooltip when no host is determined', () => {
    const wrapper = mountModal({ hostId: '' })
    const btn = wrapper.findAll('button').find((b) => b.text().includes('Bloquer'))
    expect(btn!.attributes('title')).toBe('Hôte non déterminé — renseigne le filtre Hôte')
  })

  it('shows the translated empty state when there is no request', () => {
    const wrapper = mountModal({ timeline: [] })
    expect(wrapper.text()).toContain('Aucune requête')
  })

  it('renders the translated statistics panel and group/status-family labels', () => {
    const wrapper = mountModal()
    expect(wrapper.text()).toContain('Statistiques')
    expect(wrapper.text()).toContain('Requêtes affichées')
    expect(wrapper.text()).toContain('Chemins uniques')
    expect(wrapper.text()).toContain('Domaines cibles uniques')
    expect(wrapper.text()).toContain('2xx Succès')
    expect(wrapper.text()).toContain('4xx Client')
  })

  it('renders the translated event-card meta labels', () => {
    const wrapper = mountModal()
    expect(wrapper.text()).toContain('Domaine:')
    expect(wrapper.text()).toContain('Hôte:')
    expect(wrapper.text()).toContain('Blocage:')
    expect(wrapper.text()).toContain('Expire:')
  })

  it('translates to English when the locale is switched', () => {
    setLocale('en')
    const wrapper = mountModal()
    expect(wrapper.text()).toContain('IP timeline:')
    expect(wrapper.text()).toContain('Block (CrowdSec)')
    expect(wrapper.text()).toContain('Close')
  })
})
