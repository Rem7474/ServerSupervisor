import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../i18n'

const { registerHost, getIPInventory, getHost } = vi.hoisted(() => ({
  registerHost: vi.fn(),
  getIPInventory: vi.fn(),
  getHost: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { registerHost, getIPInventory, getHost },
  getApiErrorMessage: (e: unknown, fallback?: string) =>
    (e as { response?: { data?: { error?: string } } })?.response?.data?.error || fallback || String(e),
}))

const { push } = vi.hoisted(() => ({ push: vi.fn() }))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push }),
}))

import AddHostView from './AddHostView.vue'

const mountOpts = {
  global: {
    stubs: { 'router-link': true, RouterLink: true, NetworkDiscoveryPanel: true },
  },
}

beforeEach(() => {
  setLocale('fr')
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

describe('AddHostView — mode toggle', () => {
  it('shows the manual form by default, and NetworkDiscoveryPanel when switching to scan mode', async () => {
    getIPInventory.mockResolvedValue({ data: { proxmox_guests: [], npm_hosts: [] } })
    const wrapper = mount(AddHostView, mountOpts)
    expect(wrapper.find('#host-name').exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'NetworkDiscoveryPanel' }).exists()).toBe(false)

    const scanTab = wrapper.findAll('.nav-link').find((b) => b.text() === 'Scanner un sous-réseau')
    await scanTab!.trigger('click')

    expect(wrapper.find('#host-name').exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'NetworkDiscoveryPanel' }).exists()).toBe(true)
  })
})

describe('AddHostView — form validation', () => {
  it('shows a required-field message once the name is touched and left empty', async () => {
    getIPInventory.mockResolvedValue({ data: { proxmox_guests: [], npm_hosts: [] } })
    const wrapper = mount(AddHostView, mountOpts)
    await wrapper.find('#host-name').trigger('blur')
    expect(wrapper.text()).toContain('Ce champ est requis')
  })

  it('shows an invalid-IP message for a malformed address', async () => {
    getIPInventory.mockResolvedValue({ data: { proxmox_guests: [], npm_hosts: [] } })
    const wrapper = mount(AddHostView, mountOpts)
    await wrapper.get('#host-ip').setValue('not-an-ip')
    await wrapper.find('#host-ip').trigger('blur')
    expect(wrapper.text()).toContain('Adresse IPv4 invalide')
  })

  it('does not submit while the form is invalid', async () => {
    getIPInventory.mockResolvedValue({ data: { proxmox_guests: [], npm_hosts: [] } })
    const wrapper = mount(AddHostView, mountOpts)
    await wrapper.find('form').trigger('submit.prevent')
    expect(registerHost).not.toHaveBeenCalled()
  })
})

describe('AddHostView — submission', () => {
  async function fillValidForm(wrapper: ReturnType<typeof mount>) {
    await wrapper.get('#host-name').setValue('web-01')
    await wrapper.get('#host-ip').setValue('10.0.0.5')
  }

  it('registers the host and shows the success panel with the install command and API key', async () => {
    registerHost.mockResolvedValue({ data: { id: 'h1', api_key: 'secret-key' } })
    const wrapper = mount(AddHostView, mountOpts)
    await fillValidForm(wrapper)
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(registerHost).toHaveBeenCalledWith(expect.objectContaining({ name: 'web-01', ip_address: '10.0.0.5' }))
    expect(wrapper.text()).toContain('Hôte enregistré avec succès')
    expect(wrapper.text()).toContain('secret-key')
    expect(wrapper.text()).toContain('En attente du premier rapport agent')
  })

  it('shows a translated error and no success panel when registration fails', async () => {
    registerHost.mockRejectedValue({ response: { data: { error: 'IP déjà utilisée' } } })
    const wrapper = mount(AddHostView, mountOpts)
    await fillValidForm(wrapper)
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.text()).toContain('IP déjà utilisée')
    expect(wrapper.text()).not.toContain('Hôte enregistré avec succès')
  })

  it('shows the agent-connected state and navigates to the host page on "Terminé"', async () => {
    vi.useFakeTimers()
    registerHost.mockResolvedValue({ data: { id: 'h1', api_key: 'secret-key' } })
    getHost.mockResolvedValue({ data: { status: 'online' } })
    const wrapper = mount(AddHostView, mountOpts)
    await fillValidForm(wrapper)
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    await vi.advanceTimersByTimeAsync(3000)
    expect(wrapper.text()).toContain('Agent connecté')

    const doneBtn = wrapper.findAll('button').find((b) => b.text() === 'Terminé')
    await doneBtn!.trigger('click')
    expect(push).toHaveBeenCalledWith('/hosts/h1')
    vi.useRealTimers()
  })

  it('copies the install command, API key, and agent config, each with its own confirmation', async () => {
    vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } })
    registerHost.mockResolvedValue({ data: { id: 'h1', api_key: 'secret-key' } })
    const wrapper = mount(AddHostView, mountOpts)
    await fillValidForm(wrapper)
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    const copyButtons = wrapper.findAll('button').filter((b) => b.text() === 'Copier')
    expect(copyButtons.length).toBe(3)
    await copyButtons[1].trigger('click')
    await flushPromises()
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('secret-key')
    expect(wrapper.text()).toContain('Copié')
  })
})
