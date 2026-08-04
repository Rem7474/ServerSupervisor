import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const { registerHost, getIPInventory } = vi.hoisted(() => ({
  registerHost: vi.fn(),
  getIPInventory: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { registerHost, getIPInventory },
  getApiErrorMessage: (e: unknown) => String(e),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

import AddHostView from './AddHostView.vue'

const mountOpts = {
  global: {
    stubs: { 'router-link': true, RouterLink: true },
  },
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('AddHostView — Proxmox guest IP picker', () => {
  it('fills the IP (and empty name) field when a guest IP is picked', async () => {
    getIPInventory.mockResolvedValue({
      data: {
        proxmox_guests: [
          { guest_id: 'g1', name: 'web-01', node: 'pve1', guest_type: 'lxc', vmid: 101, status: 'running', ip_addresses: ['10.0.0.5'] },
        ],
        npm_hosts: [],
      },
    })
    const wrapper = mount(AddHostView, mountOpts)

    await wrapper.find('#host-ip + .input-group button, .input-group button').trigger('click')
    await flushPromises()

    expect(getIPInventory).toHaveBeenCalledTimes(1)
    const item = wrapper.find('.guest-picker-item')
    expect(item.text()).toContain('web-01')
    expect(item.text()).toContain('10.0.0.5')

    await item.trigger('click')

    const ipInput = wrapper.get<HTMLInputElement>('#host-ip')
    const nameInput = wrapper.get<HTMLInputElement>('#host-name')
    expect(ipInput.element.value).toBe('10.0.0.5')
    expect(nameInput.element.value).toBe('web-01')
    expect(wrapper.find('.guest-picker').exists()).toBe(false)
  })

  it('excludes guests already linked to an existing host', async () => {
    getIPInventory.mockResolvedValue({
      data: {
        proxmox_guests: [
          { guest_id: 'g1', name: 'linked', node: 'pve1', guest_type: 'vm', vmid: 100, status: 'running', ip_addresses: ['10.0.0.1'], host_id: 'h1', host_name: 'existing-host' },
          { guest_id: 'g2', name: 'free', node: 'pve1', guest_type: 'vm', vmid: 102, status: 'running', ip_addresses: ['10.0.0.2'] },
        ],
        npm_hosts: [],
      },
    })
    const wrapper = mount(AddHostView, mountOpts)

    await wrapper.find('.input-group button').trigger('click')
    await flushPromises()

    const items = wrapper.findAll('.guest-picker-item')
    expect(items).toHaveLength(1)
    expect(items[0].text()).toContain('free')
  })

  it('does not overwrite an already-typed name when picking a guest IP', async () => {
    getIPInventory.mockResolvedValue({
      data: {
        proxmox_guests: [
          { guest_id: 'g1', name: 'web-01', node: 'pve1', guest_type: 'lxc', vmid: 101, status: 'running', ip_addresses: ['10.0.0.5'] },
        ],
        npm_hosts: [],
      },
    })
    const wrapper = mount(AddHostView, mountOpts)
    await wrapper.get('#host-name').setValue('Custom name')

    await wrapper.find('.input-group button').trigger('click')
    await flushPromises()
    await wrapper.find('.guest-picker-item').trigger('click')

    expect(wrapper.get<HTMLInputElement>('#host-name').element.value).toBe('Custom name')
  })
})
