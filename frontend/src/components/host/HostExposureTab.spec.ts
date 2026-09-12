import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'

const { getHostExposure } = vi.hoisted(() => ({ getHostExposure: vi.fn() }))
vi.mock('../../api', () => ({
  default: { getHostExposure },
}))

import HostExposureTab from './HostExposureTab.vue'

const exposure = {
  host_id: 'h1',
  ip_address: '10.0.0.1',
  since: new Date().toISOString(),
  domains: [{
    proxy_host_id: 'p1', connection_id: 'c1', connection_name: 'main', domain_name: 'app.example.com',
    forward_port: 443, ssl_enabled: true, npm_enabled: true, requests: 10, bytes: 1024,
    errors_4xx: 0, errors_5xx: 0, suspicious_requests: 0, blocked_requests: 0,
  }],
  total_requests: 10,
  total_suspicious_requests: 0,
  total_blocked_requests: 0,
}

beforeEach(() => {
  setLocale('fr')
  vi.clearAllMocks()
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
})

describe('HostExposureTab', () => {
  it('loads exposure data on mount and emits the domain count', async () => {
    getHostExposure.mockResolvedValue({ data: exposure })
    const wrapper = mount(HostExposureTab, { props: { hostId: 'h1' } })
    await flushPromises()

    expect(getHostExposure).toHaveBeenCalledWith('h1', '24h')
    expect(wrapper.emitted('loaded')?.[0]).toEqual([1])
  })

  it('shows the loading spinner while the request is in flight', async () => {
    let resolveFn: (v: unknown) => void = () => {}
    getHostExposure.mockReturnValue(new Promise((resolve) => { resolveFn = resolve }))
    const wrapper = mount(HostExposureTab, { props: { hostId: 'h1' } })
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.spinner-border').exists()).toBe(true)
    resolveFn({ data: exposure })
    await flushPromises()
    expect(wrapper.find('.spinner-border').exists()).toBe(false)
  })

  it('does not emit "loaded" and keeps exposure null when the request fails', async () => {
    getHostExposure.mockRejectedValue({ response: { data: {} } })
    const wrapper = mount(HostExposureTab, { props: { hostId: 'h1' } })
    await flushPromises()

    expect(wrapper.emitted('loaded')).toBeFalsy()
    expect(wrapper.find('.spinner-border').exists()).toBe(false)
  })

  it('reloads with the new period when a period button is clicked', async () => {
    getHostExposure.mockResolvedValue({ data: exposure })
    const wrapper = mount(HostExposureTab, { props: { hostId: 'h1' } })
    await flushPromises()

    const sevenDayButton = wrapper.findAll('button').find((b) => b.text() === '7j')
    await sevenDayButton!.trigger('click')
    await flushPromises()

    expect(getHostExposure).toHaveBeenLastCalledWith('h1', '168h')
  })

  it('auto-refreshes on the timer while autoRefresh stays enabled', async () => {
    getHostExposure.mockResolvedValue({ data: exposure })
    mount(HostExposureTab, { props: { hostId: 'h1' } })
    await flushPromises()
    expect(getHostExposure).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(60_000)
    expect(getHostExposure).toHaveBeenCalledTimes(2)
  })

  it('stops the refresh timer on unmount', async () => {
    getHostExposure.mockResolvedValue({ data: exposure })
    const wrapper = mount(HostExposureTab, { props: { hostId: 'h1' } })
    await flushPromises()
    wrapper.unmount()

    await vi.advanceTimersByTimeAsync(120_000)
    expect(getHostExposure).toHaveBeenCalledTimes(1)
  })
})
