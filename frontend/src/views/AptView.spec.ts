import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref, reactive } from 'vue'
import { setLocale } from '../i18n'

const { addToast } = vi.hoisted(() => ({ addToast: vi.fn() }))
vi.mock('../composables/useGlobalToast', () => ({ addToast }))

function baseUseApt() {
  return {
    hosts: ref([]),
    selectedHosts: ref([]),
    hostExpanded: reactive({}),
    aptStatuses: reactive({}),
    aptHistories: reactive({}),
    uuStatuses: reactive({}),
    latestAgentVersion: ref(''),
    hostCmdLoading: reactive({}),
    enrichingHosts: reactive({}),
    canRunApt: ref(true),
    selectAll: ref(false),
    toggleSelected: vi.fn(),
    scheduleHost: ref(null),
    openScheduleModal: vi.fn(),
    showConsole: ref(false),
    liveCommand: ref(null),
    aptBulkLoading: ref(null),
    hostSearch: ref(''),
    hostQuickFilter: ref('all'),
    hostSortKey: ref('name'),
    hostSortDir: ref('asc'),
    hostFilterOptions: ref([]),
    filteredHosts: ref([]),
    isAgentOutdated: vi.fn(() => false),
    outdatedSelectedHosts: ref([]),
    bulkAgentUpdateLoading: ref(false),
    watchCommand: vi.fn(),
    closeLiveConsole: vi.fn(),
    runAptCmdForHost: vi.fn(),
    bulkAptCmd: vi.fn(),
    bulkAgentUpdate: vi.fn(),
    wsStatus: ref('connected'),
    wsError: ref(''),
    retryCount: ref(0),
    dataStaleAlert: ref(false),
    reconnect: vi.fn(),
  }
}

const useAptMock = vi.hoisted(() => vi.fn())
vi.mock('../composables/useApt', () => ({ useApt: useAptMock }))

import AptView from './AptView.vue'

const stubs = { RouterLink: { template: '<a><slot /></a>' } }

beforeEach(() => {
  setLocale('fr')
  vi.clearAllMocks()
  useAptMock.mockReturnValue(baseUseApt())
})

describe('AptView', () => {
  it('renders the page title and description in French', () => {
    const wrapper = mount(AptView, { global: { stubs } })
    expect(wrapper.text()).toContain('APT — Mises à jour système')
    expect(wrapper.text()).toContain('Gérer les mises à jour APT sur tous les hôtes')
  })

  it('shows the empty state when no host matches the current filters', () => {
    useAptMock.mockReturnValue({ ...baseUseApt(), wsStatus: ref('connected'), filteredHosts: ref([]) })
    const wrapper = mount(AptView, { global: { stubs } })
    expect(wrapper.text()).toContain('Aucun hôte ne correspond aux filtres.')
  })

  it('shows loading skeletons instead of the empty state while the WS is still connecting with no hosts yet', () => {
    useAptMock.mockReturnValue({ ...baseUseApt(), wsStatus: ref('connecting'), hosts: ref([]), filteredHosts: ref([]) })
    const wrapper = mount(AptView, { global: { stubs } })
    expect(wrapper.text()).not.toContain('Aucun hôte ne correspond aux filtres.')
  })

  it('renders an AptHostCard per filtered host', () => {
    const host = { id: 'h1', name: 'web-01' }
    useAptMock.mockReturnValue({ ...baseUseApt(), filteredHosts: ref([host]) })
    const wrapper = mount(AptView, { global: { stubs } })
    expect(wrapper.text()).toContain('web-01')
  })

  it('shows a translated success toast when a scheduled task is created', async () => {
    useAptMock.mockReturnValue(baseUseApt())
    const wrapper = mount(AptView, { global: { stubs } })

    await wrapper.findComponent({ name: 'AptScheduleModal' }).vm.$emit('created')

    expect(addToast).toHaveBeenCalledWith('Tâche planifiée créée', 'success')
  })
})
