import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import HostTimelineTab from './HostTimelineTab.vue'

const { getHostTimeline } = vi.hoisted(() => ({ getHostTimeline: vi.fn() }))

vi.mock('../../api', () => ({
  default: { getHostTimeline },
  getApiErrorMessage: (e: unknown) => String(e),
}))

describe('HostTimelineTab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
  })

  it('shows the triggering user for an event that has one', async () => {
    getHostTimeline.mockResolvedValue({
      data: {
        events: [
          { id: 'cmd-1', type: 'command', timestamp: new Date().toISOString(), title: 'apt upgrade', status: 'completed', user: 'bob' },
        ],
      },
    })

    const wrapper = mount(HostTimelineTab, { props: { hostId: 'h1' } })
    await flushPromises()

    expect(wrapper.text()).toContain('bob')
  })

  it('renders a placeholder in the Utilisateur column when the event has none (e.g. an incident)', async () => {
    getHostTimeline.mockResolvedValue({
      data: {
        events: [
          { id: 'inc-1', type: 'incident', timestamp: new Date().toISOString(), title: 'CPU élevé', severity: 'warn' },
        ],
      },
    })

    const wrapper = mount(HostTimelineTab, { props: { hostId: 'h1' } })
    await flushPromises()

    expect(wrapper.find('tbody tr').text()).toContain('—')
  })

  it('renders events as table rows', async () => {
    getHostTimeline.mockResolvedValue({
      data: {
        events: [
          { id: 'cmd-1', type: 'command', timestamp: new Date().toISOString(), title: 'docker restart nginx', module: 'docker', status: 'completed', user: 'bob' },
        ],
      },
    })

    const wrapper = mount(HostTimelineTab, { props: { hostId: 'h1' } })
    await flushPromises()

    expect(wrapper.find('table.card-table').exists()).toBe(true)
    expect(wrapper.find('tbody tr').text()).toContain('docker restart nginx')
  })

  it('emits watch-command with the full event (including action/target/output) so it can be interpreted per type', async () => {
    const ev = {
      id: 'cmd-2',
      type: 'command',
      timestamp: new Date().toISOString(),
      title: 'systemd list',
      module: 'systemd',
      action: 'list',
      target: '',
      output: '[{"name":"nginx.service"}]',
      status: 'completed',
      user: 'bob',
    }
    getHostTimeline.mockResolvedValue({ data: { events: [ev] } })

    const wrapper = mount(HostTimelineTab, { props: { hostId: 'h1' } })
    await flushPromises()

    await wrapper.find('button[title="Voir les logs"]').trigger('click')

    expect(wrapper.emitted('watch-command')?.[0]?.[0]).toMatchObject(ev)
  })
})
