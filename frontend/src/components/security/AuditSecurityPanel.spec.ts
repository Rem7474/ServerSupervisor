import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import AuditSecurityPanel from './AuditSecurityPanel.vue'

function mountPanel(overrides: Record<string, unknown> = {}) {
  return mount(AuditSecurityPanel, {
    props: {
      security: { stats: { total: 10, failures: 2, unique_ips: 3 }, blocked_ips: [], top_failed_ips: [] },
      period: 24,
      periodLabel: '24h',
      periodOptions: [{ hours: 24, label: '24h' }],
      unblockingIp: '',
      ...overrides,
    },
  })
}

beforeEach(() => {
  setLocale('fr')
})

describe('AuditSecurityPanel', () => {
  it('renders the translated KPI labels with the period interpolated', () => {
    const wrapper = mountPanel()
    expect(wrapper.text()).toContain('Connexions (24h)')
    expect(wrapper.text()).toContain('Échecs (24h)')
    expect(wrapper.text()).toContain('IPs uniques (24h)')
  })

  it('shows the translated empty states when there is no blocked IP or failure', () => {
    const wrapper = mountPanel()
    expect(wrapper.text()).toContain('Aucune IP bloquée')
    expect(wrapper.text()).toContain('Aucun échec enregistré sur cette période')
  })

  it('shows a blocked IP with a translated badge, and unblocks it on click', async () => {
    const wrapper = mountPanel({ security: { stats: {}, blocked_ips: ['1.2.3.4'], top_failed_ips: [] } })
    expect(wrapper.text()).toContain('Bloquée')
    expect(wrapper.text()).toContain('Débloquer')

    await wrapper.find('button.btn-outline-success').trigger('click')
    expect(wrapper.emitted('unblock')?.[0]).toEqual(['1.2.3.4'])
  })

  it('shows a pluralized failure-count badge for the top failed IPs', () => {
    const wrapper = mountPanel({
      security: { stats: {}, blocked_ips: [], top_failed_ips: [{ ip_address: '5.6.7.8', fail_count: 1 }, { ip_address: '9.9.9.9', fail_count: 4 }] },
    })
    expect(wrapper.text()).toContain('1 échec')
    expect(wrapper.text()).toContain('4 échecs')
  })
})
