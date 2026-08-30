import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { ref, computed } from 'vue'

const cveSummary = ref<{ critical_count?: number; hosts_with_critical?: number } | null>(null)
const proxmoxSummary = ref<{
  nodes_down?: number; storage_near_full?: number; storage_offline?: number; recent_failed_tasks?: number
} | null>(null)
const attentionItems = ref<Array<{ key: string; label: string; to: string; severity: string; count?: number }>>([])

vi.mock('../composables/useDashboard', () => ({
  useDashboard: () => ({
    hosts: ref([]),
    versionComparisons: ref([]),
    proxmoxSummary,
    hasProxmox: computed(() => false),
    cveSummary,
    proxmoxNodes: ref([]),
    proxmoxLinks: ref([]),
    hostMetrics: ref({}),
    aptPendingHosts: ref({}),
    diskUsage: ref({}),
    loading: ref(false),
    searchQuery: ref(''),
    statusFilter: ref('all'),
    tagFilter: ref('all'),
    allTags: ref([]),
    sortKey: ref('name'),
    sortDir: ref('asc'),
    selectedHostIds: ref([]),
    aptLoading: ref(''),
    summaryHours: ref(24),
    summaryChartSeries: ref(null),
    summaryLoading: ref(false),
    chartSource: ref('agents'),
    chartSources: computed(() => [{ key: 'agents', label: 'Agents hôtes' }, { key: 'proxmox', label: 'Nœuds Proxmox' }]),
    selectedCount: computed(() => 0),
    canRunApt: computed(() => false),
    metricsReady: ref(true),
    wsStatus: ref('connected'),
    wsError: ref(''),
    retryCount: ref(0),
    dataStaleAlert: ref(false),
    reconnect: vi.fn(),
    effectiveMetricsByHost: computed(() => ({})),
    sortedHosts: computed(() => []),
    summaryChartOptions: computed(() => ({})),
    fetchSummary: vi.fn(),
    changeSummaryRange: vi.fn(),
    clearSelection: vi.fn(),
    sendBulkApt: vi.fn(),
    formatUptime: (s: number) => `${s}s`,
    cpuColor: () => '',
    memColor: () => '',
    diskColor: () => '',
  }),
}))

vi.mock('../composables/useAttentionCenter', () => ({
  useAttentionCenter: () => ({ items: attentionItems }),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ path: '/' }),
}))

import DashboardView from './DashboardView.vue'
import { setLocale } from '../i18n'

function mountView() {
  return mount(DashboardView, {
    global: {
      stubs: {
        WsStatusBar: true,
        DashboardKPIs: true,
        ProxmoxClusterCard: true,
        DashboardDockerVersions: true,
        LoadingSkeleton: true,
        PaginationNav: true,
        BulkActionBar: true,
        RelativeTime: true,
        'router-link': { template: '<a><slot /></a>' },
      },
    },
  })
}

beforeEach(() => {
  setActivePinia(createPinia())
  setLocale('fr')
  cveSummary.value = null
  proxmoxSummary.value = null
  attentionItems.value = []
})

describe('DashboardView — banner items', () => {
  it('renders nothing when there is nothing to report', () => {
    const wrapper = mountView()
    expect(wrapper.text()).not.toContain('Points d\'attention')
  })

  it('shows a singular CVE banner for exactly one critical CVE with no host breakdown', () => {
    cveSummary.value = { critical_count: 1, hosts_with_critical: 0 }
    const wrapper = mountView()
    const banner = wrapper.find('.list-group')
    expect(banner.text()).toBe('11 CVE critique')
  })

  it('pluralizes both the CVE count and the host count, joined by "sur"', () => {
    cveSummary.value = { critical_count: 5, hosts_with_critical: 2 }
    const wrapper = mountView()
    expect(wrapper.text()).toContain('5 CVE critiques sur 2 hôtes')
  })

  it('translates the combined CVE/host banner in English', () => {
    setLocale('en')
    cveSummary.value = { critical_count: 5, hosts_with_critical: 2 }
    const wrapper = mountView()
    expect(wrapper.text()).toContain('5 critical CVEs across 2 hosts')
  })

  it('combines multiple Proxmox health parts, each independently pluralized', () => {
    proxmoxSummary.value = { nodes_down: 1, storage_near_full: 2, storage_offline: 0, recent_failed_tasks: 3 }
    const wrapper = mountView()
    expect(wrapper.text()).toContain('1 nœud hors ligne')
    expect(wrapper.text()).toContain('2 stockages presque pleins')
    expect(wrapper.text()).toContain('3 tâches échouées (24h)')
  })

  it('shows attention-center items passed through as-is', () => {
    attentionItems.value = [{ key: 'proxmox-links', label: 'Liaisons suggérées', to: '/proxmox', severity: 'info', count: 2 }]
    const wrapper = mountView()
    expect(wrapper.text()).toContain('Liaisons suggérées')
  })
})
