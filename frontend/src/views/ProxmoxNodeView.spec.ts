import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'

// Auto-unmount after each test so the view's onUnmounted clears its polling
// timers and no late-resolving async touches a torn-down component.
enableAutoUnmount(afterEach)

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
      getProxmoxNodeGuestNetworks: ok({}),
      getProxmoxNodeServices: ok([]),
    },
  }
})

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'node-1' }, query: {} }),
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
    for (const label of ['VMs', 'LXC', 'Stockage', 'Disques', 'Tâches', 'Mises à jour', 'Services', 'Sécurité']) {
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
})
