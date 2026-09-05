import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'

enableAutoUnmount(afterEach)

const baseGuest = {
  id: 'g1',
  connection_id: 'conn-1',
  node_name: 'pve1',
  guest_type: 'lxc',
  vmid: 101,
  name: 'web1',
  status: 'running',
  cpu_alloc: 2,
  cpu_usage: 0.1,
  mem_alloc: 1024,
  mem_usage: 512,
  disk_alloc: 0,
  disk_usage: 0,
  uptime: 3600,
}

interface GuestLinkFixture {
  id: string
  status: string
  host_id: string
  host_hostname?: string
  host_name?: string
}

const { getProxmoxInstance, getProxmoxGuestLink } = vi.hoisted(() => ({
  getProxmoxInstance: vi.fn(),
  getProxmoxGuestLink: vi.fn(async (): Promise<{ data: GuestLinkFixture | null }> => ({ data: null })),
}))

const qemuGuest = { ...baseGuest, id: 'g2', guest_type: 'qemu' }

const { guestsResponse } = vi.hoisted(() => ({ guestsResponse: { data: [] as unknown[] } }))

vi.mock('../api', () => {
  const ok = (data: unknown = {}) => async () => ({ data })
  return {
    default: {
      getProxmoxGuests: vi.fn(async () => guestsResponse),
      getProxmoxGuestLink,
      getProxmoxNodes: ok([{ id: 'node-1', node_name: 'pve1' }]),
      getProxmoxNodeGuestNetworks: ok({}),
      getProxmoxGuestMetrics: ok([]),
      getProxmoxInstance,
    },
  }
})

const { routeParamId } = vi.hoisted(() => ({ routeParamId: { value: 'g1' } }))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: routeParamId.value }, query: {} }),
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock('../components/proxmox/GuestExposureCard.vue', () => ({
  default: { name: 'GuestExposureCard', template: '<div />' },
}))

import ProxmoxGuestView from './ProxmoxGuestView.vue'
import { useAuthStore } from '../stores/auth'

// ProxmoxConsoleLazy is defineAsyncComponent(() => import('.../ProxmoxConsole.vue'))
// — vi.mock-ing that module confuses @vue/test-utils' Teleport introspection
// when combined with an async component wrapper. Stubbing by tag name
// instead avoids ever resolving the real (xterm.js-pulling) component.
function mountView() {
  setActivePinia(createPinia())
  const auth = useAuthStore()
  auth.setAuth({ role: 'admin', username: 'admin' } as never, 'admin')
  return mount(ProxmoxGuestView, {
    global: {
      stubs: {
        ProxmoxConsoleLazy: { props: ['guestId', 'guestName', 'show'], template: '<div class="proxmox-console-stub" />' },
      },
    },
  })
}

describe('ProxmoxGuestView — console entry point', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    routeParamId.value = 'g1'
    guestsResponse.data = [baseGuest]
  })

  it('warns and explains when the lxc guest\'s connection has no console credentials configured', async () => {
    getProxmoxInstance.mockResolvedValue({ data: { console_configured: false } })
    const wrapper = mountView()
    await flushPromises()
    await flushPromises()

    expect(getProxmoxInstance).toHaveBeenCalledWith('conn-1')
    const btn = wrapper.findAll('button').find((b) => b.text().includes('Console'))
    expect(btn!.attributes('title')).toContain('non configurée')
    expect(wrapper.find('[title*="Identifiants console PVE non configurés"]').exists()).toBe(true)
  })

  it('shows no warning and a normal tooltip when console credentials are configured', async () => {
    getProxmoxInstance.mockResolvedValue({ data: { console_configured: true } })
    const wrapper = mountView()
    await flushPromises()
    await flushPromises()

    const btn = wrapper.findAll('button').find((b) => b.text().includes('Console'))
    expect(btn!.attributes('title')).toBe('Ouvrir une console interactive')
    expect(wrapper.find('[title*="Identifiants console PVE non configurés"]').exists()).toBe(false)
  })

  it('does not mount the console component until "Console" is clicked, then keeps it mounted', async () => {
    getProxmoxInstance.mockResolvedValue({ data: { console_configured: true } })
    const wrapper = mountView()
    await flushPromises()
    await flushPromises()

    expect(wrapper.find('.proxmox-console-stub').exists()).toBe(false)

    const btn = wrapper.findAll('button').find((b) => b.text().includes('Console'))
    await btn!.trigger('click')
    await flushPromises()

    expect(wrapper.find('.proxmox-console-stub').exists()).toBe(true)
  })

  it('disables the Console button and skips the config check entirely for a QEMU guest', async () => {
    routeParamId.value = 'g2'
    guestsResponse.data = [qemuGuest]

    const wrapper = mountView()
    await flushPromises()
    await flushPromises()

    expect(getProxmoxInstance).not.toHaveBeenCalled()
    const btn = wrapper.findAll('button').find((b) => b.text().includes('Console'))
    expect(btn!.attributes('disabled')).toBeDefined()
    expect(btn!.attributes('title')).toBe('VM QEMU : bientôt disponible')
    // The "console not configured" warning icon is lxc-only.
    expect(wrapper.find('[title*="Identifiants console PVE non configurés"]').exists()).toBe(false)
  })

  it('shows the translated exposure hint and correlation link once the guest is confirmed-linked to a host', async () => {
    getProxmoxGuestLink.mockResolvedValue({
      data: { id: 'link-1', status: 'confirmed', host_id: 'host-1', host_hostname: 'web-01.local', host_name: 'web-01' },
    })
    routeParamId.value = 'g1'
    guestsResponse.data = [baseGuest]

    const wrapper = mountView()
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain("Ce guest est lié à l'hôte web-01.local")
    expect(wrapper.text()).toContain('Voir la corrélation domaine/IP')
    // GuestExposureCard is only rendered for a guest that is NOT confirmed-linked.
    expect(wrapper.findComponent({ name: 'GuestExposureCard' }).exists()).toBe(false)
  })
})
