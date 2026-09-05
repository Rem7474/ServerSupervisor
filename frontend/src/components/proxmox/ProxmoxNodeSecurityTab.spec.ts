import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'

const { getProxmoxNodeSyslog } = vi.hoisted(() => ({
  getProxmoxNodeSyslog: vi.fn(),
}))

vi.mock('../../api', () => ({
  default: { getProxmoxNodeSyslog },
}))

import ProxmoxNodeSecurityTab from './ProxmoxNodeSecurityTab.vue'

beforeEach(() => {
  setLocale('fr')
  vi.clearAllMocks()
})

describe('ProxmoxNodeSecurityTab', () => {
  it('renders translated toolbar and column headers', async () => {
    getProxmoxNodeSyslog.mockResolvedValue({ data: [] })
    const wrapper = mount(ProxmoxNodeSecurityTab, { props: { nodeId: 'n1', active: true } })
    await flushPromises()

    expect(wrapper.text()).toContain('Rotation (3 services)')
    expect(wrapper.text()).toContain('Tous les services')
    expect(wrapper.find('input').attributes('placeholder')).toBe('Filtre syslog (ex: failed, denied, apparmor)')
    expect(wrapper.text()).toContain('Rechercher')
  })

  it('shows the translated empty state when no events match', async () => {
    getProxmoxNodeSyslog.mockResolvedValue({ data: [] })
    const wrapper = mount(ProxmoxNodeSecurityTab, { props: { nodeId: 'n1', active: true } })
    await flushPromises()

    expect(wrapper.text()).toContain('Aucun événement de sécurité trouvé pour ce filtre.')
  })

  it('renders translated table headers and a syslog row once events load', async () => {
    getProxmoxNodeSyslog.mockResolvedValue({
      data: [{ id: '1', t: 'Jan 15 10:00:00 pve1 sshd[123]: Failed password for root' }],
    })
    const wrapper = mount(ProxmoxNodeSecurityTab, { props: { nodeId: 'n1', active: true } })
    await flushPromises()

    for (const label of ['Date', 'Niveau', 'Tag', 'Message']) {
      expect(wrapper.text()).toContain(label)
    }
    expect(wrapper.text()).toContain('sshd')
  })

  it('surfaces the translated "no syslog service accessible" error when every rotate call fails', async () => {
    getProxmoxNodeSyslog.mockRejectedValue(new Error('network error'))
    const wrapper = mount(ProxmoxNodeSecurityTab, { props: { nodeId: 'n1', active: true } })
    await flushPromises()

    expect(wrapper.text()).toContain('Aucun service syslog accessible (pveproxy, sshd, pvedaemon).')
  })
})
