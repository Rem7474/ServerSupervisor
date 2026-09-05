import { describe, it, expect, beforeEach, vi } from 'vitest'
import { ref, computed } from 'vue'
import { mount } from '@vue/test-utils'
import { setLocale } from '../i18n'

const trafficDelta = ref({ rx: 0, tx: 0, intervalSec: 0 })

vi.mock('../composables/useNetwork', () => ({
  useNetwork: () => ({
    hosts: ref([{ id: 'h1' }]),
    containers: ref([{ id: 'c1' }]),
    proxmoxGuestIPs: ref([]),
    npmEntries: ref([]),
    ipInventoryLoading: ref(false),
    viewMode: ref('graph'),
    networkTab: ref('topology'),
    rootNodeName: ref(''),
    rootNodeIp: ref(''),
    autheliaLabel: ref(''),
    autheliaIp: ref(''),
    internetLabel: ref(''),
    internetIp: ref(''),
    networkServices: ref([]),
    hostPortConfig: ref({}),
    nodePositions: ref({}),
    topologyConfigLoaded: ref(true),
    saveStatus: ref('idle'),
    selectedNode: ref(null),
    rootHostId: ref(''),
    autheliaHostId: ref(''),
    rootPortId: ref(''),
    autheliaPortId: ref(''),
    filterInternetOnly: ref(false),
    filterHideInternal: ref(false),
    debouncedSave: vi.fn(),
    discoveredPortsByHost: ref({}),
    hostPortOverrides: ref({}),
    combinedServices: computed(() => [{ id: 's1' }]),
    guestNodes: ref([]),
    filteredGraphHosts: computed(() => []),
    filteredServices: computed(() => []),
    totalPorts: computed(() => 5),
    hostsOnline: computed(() => 1),
    containersRunning: computed(() => 1),
    trafficDelta,
    formatBytes: (n: number) => `${n} B`,
    onNodePositionsUpdate: vi.fn(),
    wsStatus: ref('connected'),
    wsError: ref(''),
    retryCount: ref(0),
    reconnect: vi.fn(),
  }),
}))

import NetworkView from './NetworkView.vue'

const mountOpts = {
  global: {
    stubs: {
      'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' },
      WsStatusBar: true,
      NetworkGraph: true,
      NetworkNodeDetail: true,
      NetworkPortList: true,
      NetworkTopologyConfig: true,
      ErrorBoundary: { template: '<div><slot /></div>' },
    },
  },
}

describe('NetworkView', () => {
  beforeEach(() => {
    setLocale('fr')
    trafficDelta.value = { rx: 0, tx: 0, intervalSec: 0 }
  })

  it('renders the translated header, KPI labels and topology header', () => {
    const wrapper = mount(NetworkView, mountOpts)

    expect(wrapper.text()).toContain('Dashboard')
    expect(wrapper.text()).toContain('Architecture réseau')
    expect(wrapper.text()).toContain('Architecture réseau logique')
    expect(wrapper.text()).toContain('Hôtes')
    expect(wrapper.text()).toContain('1 en ligne')
    expect(wrapper.text()).toContain('Conteneurs')
    expect(wrapper.text()).toContain('Trafic réseau')
    expect(wrapper.text()).toContain('En attente de données')
    expect(wrapper.text()).toContain('Topologie réseau')
    expect(wrapper.text()).toContain('Graphe')
    expect(wrapper.text()).toContain('Cartes')
    expect(wrapper.text()).toContain('Topologie')
    expect(wrapper.text()).toContain('Configuration')
    expect(wrapper.text()).toContain('Internet uniquement')
    expect(wrapper.text()).toContain('Masquer les ports internes')
  })

  it('renders the traffic interval label once data is flowing', () => {
    trafficDelta.value = { rx: 1024, tx: 2048, intervalSec: 30 }
    const wrapper = mount(NetworkView, mountOpts)
    expect(wrapper.text()).toContain('sur 30s')
    expect(wrapper.text()).not.toContain('En attente de données')
  })

  it('shows the ports-and-containers title in cards mode', async () => {
    const wrapper = mount(NetworkView, mountOpts)
    await wrapper.findAll('.btn-group button')[1].trigger('click')
    expect(wrapper.text()).toContain('Ports & conteneurs')
  })

  it('translates to English when the locale is switched', () => {
    setLocale('en')
    const wrapper = mount(NetworkView, mountOpts)

    expect(wrapper.text()).toContain('Network architecture')
    expect(wrapper.text()).toContain('Logical network architecture')
    expect(wrapper.text()).toContain('Hosts')
    expect(wrapper.text()).toContain('Network traffic')
  })
})
