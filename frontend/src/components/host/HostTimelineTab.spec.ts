import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import HostTimelineTab from './HostTimelineTab.vue'

const { getHostTimeline } = vi.hoisted(() => ({ getHostTimeline: vi.fn() }))

vi.mock('../../api', () => ({
  default: { getHostTimeline },
  getApiErrorMessage: (e: unknown) => String(e),
}))

describe('HostTimelineTab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
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

  it('does not render a user separator when the event has none (e.g. an incident)', async () => {
    getHostTimeline.mockResolvedValue({
      data: {
        events: [
          { id: 'inc-1', type: 'incident', timestamp: new Date().toISOString(), title: 'CPU élevé', severity: 'warn' },
        ],
      },
    })

    const wrapper = mount(HostTimelineTab, { props: { hostId: 'h1' } })
    await flushPromises()

    expect(wrapper.text()).not.toContain('·')
  })
})
