import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

// Auto-unmount after each test so the view's onUnmounted clears its polling
// timers and no late-resolving async touches a torn-down component.
enableAutoUnmount(afterEach)

const { routeQuery } = vi.hoisted(() => ({ routeQuery: { current: {} as Record<string, string> } }))

vi.mock('../api', () => {
  const ok = (data: unknown = {}) => async () => ({ data })
  return {
    default: {
      getProxmoxNode: vi.fn(async () => ({
        data: {
          node_name: 'pve1',
          status: 'online',
          pending_updates: 0,
          storages: [],
          disks: [],
          tasks: [],
          guests: [],
        },
      })),
      // These endpoints return arrays in production — match the real shape so
      // the view's array operations (.map/.filter) don't throw.
      getProxmoxNodeSensorSourceCandidates: ok([]),
      getProxmoxLinks: ok([]),
      getProxmoxNodeStatus: ok(null),
      getProxmoxNodeSyslog: ok([]),
      getProxmoxNodeRRD: ok([]),
      getProxmoxNodeCpuTempHistory: ok([]),
      getProxmoxNodeFanRPMHistory: ok([]),
      getProxmoxNodes: ok([]),
      getProxmoxNodeGuestNetworks: vi.fn(ok({})),
      getProxmoxNodeGuestExposure: vi.fn(ok({})),
      getProxmoxNodeServices: ok([]),
    },
  }
})

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'node-1' }, query: routeQuery.current }),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}))

// Async children pull in Chart.js / chartTheme; replace them with trivial stubs
// so their lazy import chain never runs (avoids post-teardown import errors).
vi.mock('../components/proxmox/ProxmoxNodeChartsPanel.vue', () => ({
  default: { name: 'ProxmoxNodeChartsPanel', template: '<div />' },
}))
vi.mock('../components/host/CommandLogPanel.vue', () => ({
  default: { name: 'CommandLogPanel', template: '<div />' },
}))

import apiClient from '../api'
import ProxmoxNodeView from './ProxmoxNodeView.vue'

// Heavy/async children (Chart.js panel, command log) are verified separately;
// stub them so the happy-dom shell test stays focused and clean.
const mountOpts = {
  global: {
    stubs: {
      ProxmoxNodeChartsPanel: true,
      CommandLogPanel: true,
      'router-link': true,
      RouterLink: true,
    },
  },
}

describe('ProxmoxNodeView (characterization)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routeQuery.current = {}
    // ProxmoxNodeGuestsTab (mounted for the vms/lxc tabs) calls useAuthStore()
    // for its admin-only guest power-action buttons.
    setActivePinia(createPinia())
  })

  it('fetches the node on mount', async () => {
    mount(ProxmoxNodeView, mountOpts)
    await flushPromises()
    expect(apiClient.getProxmoxNode).toHaveBeenCalledWith('node-1')
  })

  it('renders the node header + all tab labels once loaded', async () => {
    const wrapper = mount(ProxmoxNodeView, mountOpts)
    await flushPromises()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('pve1')
    for (const label of ['VMs', 'LXC', 'Stockage', 'Disques', 'Tâches', 'Mises à jour', 'Services', 'Journaux sécurité']) {
      expect(text).toContain(label)
    }
  })

  it('switches the active tab on nav click', async () => {
    const wrapper = mount(ProxmoxNodeView, mountOpts)
    await flushPromises()
    await flushPromises()

    const disksBtn = wrapper.findAll('.proxmox-node-tabs .nav-link').find((b) => b.text().includes('Disques'))
    expect(disksBtn).toBeTruthy()
    await disksBtn!.trigger('click')
    expect(disksBtn!.classes()).toContain('active')
  })

  it('fetches guest networks/exposure (IP, domains) when landing directly on ?tab=lxc, without a click', async () => {
    // Regression test: a hard refresh / direct link restores the tab via
    // useProxmoxNode's load(), a different code path than the nav-click
    // handler that used to be the only place these two loaders were
    // triggered from — the IP/domains columns silently stayed empty.
    routeQuery.current = { tab: 'lxc' }
    mount(ProxmoxNodeView, mountOpts)
    await flushPromises()
    await flushPromises()

    expect(apiClient.getProxmoxNodeGuestNetworks).toHaveBeenCalledWith('node-1')
    expect(apiClient.getProxmoxNodeGuestExposure).toHaveBeenCalledWith('node-1')
  })

  it('does not fetch guest networks/exposure when landing on an unrelated tab', async () => {
    routeQuery.current = { tab: 'storage' }
    mount(ProxmoxNodeView, mountOpts)
    await flushPromises()
    await flushPromises()

    expect(apiClient.getProxmoxNodeGuestNetworks).not.toHaveBeenCalled()
    expect(apiClient.getProxmoxNodeGuestExposure).not.toHaveBeenCalled()
  })
})
