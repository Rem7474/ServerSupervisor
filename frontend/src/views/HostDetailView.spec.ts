import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref, reactive, computed } from 'vue'
import { setLocale } from '../i18n'

const { addToast } = vi.hoisted(() => ({ addToast: vi.fn() }))
vi.mock('../composables/useGlobalToast', () => ({ addToast }))

// Mirrors useHostDetail.ts's own local AnyRecord — the real composable
// keeps these fields untyped (WS-fed JSON), so the mock does too.
type AnyRecord = Record<string, unknown>

function baseUseHostDetail() {
  return {
    auth: reactive({ isAdmin: false, username: 'alice' }),
    hostId: 'h1',
    canRunApt: ref(true),
    activeTab: ref('overview'),
    isEditing: ref(false),
    tasksCount: ref(0),
    customTasksCount: ref(0),
    aptCmdLoading: ref(null),
    agentUpdateLoading: ref(false),
    host: ref<AnyRecord | null>({ id: 'h1', name: 'web-01', hostname: 'web-01', os: 'Debian 12', ip_address: '10.0.0.5', status: 'online', agent_version: '1.2.3', last_seen: new Date().toISOString() }),
    containers: ref<AnyRecord[]>([]),
    metricsUpdatedAt: ref(0),
    versionComparisons: ref([]),
    aptStatus: ref({ pending_packages: 0, security_updates: 0 }),
    cmdHistory: ref<AnyRecord[]>([]),
    diskMetrics: ref(null),
    diskHealth: ref(null),
    networkFlows: ref(null),
    proxmoxLink: ref<AnyRecord | null>(null),
    linkSaving: ref(false),
    hostActiveIncidents: ref<AnyRecord[]>([]),
    incidentsLoading: ref(false),
    effectiveMetrics: ref(null),
    effectiveMetricsSource: ref('agent'),
    showLinkForm: ref(false),
    showLinkButton: ref(false),
    linkCandidates: ref<AnyRecord[]>([]),
    linkCandidatesLoading: ref(false),
    selectedCandidate: ref(''),
    liveCommand: ref(null),
    showConsole: ref(false),
    wsStatus: ref('connected'),
    wsError: ref(''),
    retryCount: ref(0),
    reconnect: vi.fn(),
    openCommand: vi.fn(),
    sendAptCmd: vi.fn(),
    sendAgentUpdate: vi.fn(),
    isAgentUpToDate: vi.fn(() => true),
    canUpdateAgent: computed(() => true),
    deleteHost: vi.fn(),
    loadCmdHistoryRefresh: vi.fn(),
    confirmLink: vi.fn(),
    ignoreLink: vi.fn(),
    changeMetricsSource: vi.fn(),
    deleteLink: vi.fn(),
    openLinkForm: vi.fn(),
    createManualLink: vi.fn(),
    closeConsoleAndStream: vi.fn(),
    clearConsoleOutput: vi.fn(),
    formatBytesLink: vi.fn((b: number) => `${b} B`),
    hostPerms: ref<AnyRecord[]>([]),
    permLoading: ref(false),
    addPermModal: ref(false),
    newPermUsername: ref(''),
    newPermLevel: ref('viewer'),
    permSaving: ref(false),
    permError: ref(''),
    availableUsers: ref<AnyRecord[]>([]),
    openAddPermission: vi.fn(),
    savePermission: vi.fn(),
    revokePermission: vi.fn(),
    uuStatus: ref(null),
    uuRuns: ref([]),
    uuForm: ref({}),
    uuLoading: ref(false),
    handleUUInstall: vi.fn(),
    handleUUConfigure: vi.fn(),
    handleUURunNow: vi.fn(),
    openUULog: vi.fn(),
  }
}

const useHostDetailMock = vi.hoisted(() => vi.fn())
vi.mock('../composables/useHostDetail', () => ({ useHostDetail: useHostDetailMock }))

import HostDetailView from './HostDetailView.vue'

// Every tab body pulls in its own data-fetching composable — stub them all so
// this spec exercises HostDetailView's own template logic (header, Proxmox
// link panel, overview KPIs, tab badges, permissions CRUD) without each leaf
// component making real API/WS calls.
const stubs = {
  RouterLink: { template: '<a><slot /></a>' },
  HostMetricsPanel: true,
  DiskMetricsCard: true,
  DiskHistoryChart: true,
  DiskHealthCard: true,
  ProxmoxHostDiskHealthCard: true,
  HostDockerTab: true,
  HostAptTab: true,
  HostBackupTab: true,
  NetworkFlowsTable: true,
  HostExposureTab: true,
  HostSystemTab: true,
  HostProcessesPanel: true,
  HostCustomTasksTab: true,
  HostTasksTab: true,
  HostTimelineTab: true,
  HostEditForm: true,
  CommandLogPanel: true,
}

beforeEach(() => {
  setLocale('fr')
  vi.clearAllMocks()
  useHostDetailMock.mockReturnValue(baseUseHostDetail())
})

describe('HostDetailView — header', () => {
  it('shows the host name, status and agent version badge', () => {
    const wrapper = mount(HostDetailView, { global: { stubs } })
    expect(wrapper.text()).toContain('web-01')
    expect(wrapper.text()).toContain('En ligne')
    expect(wrapper.text()).toContain('Agent v1.2.3')
  })

  it('falls back to hostname/loading/not-connected placeholders when data is missing', () => {
    const api = baseUseHostDetail()
    api.host = ref(null)
    useHostDetailMock.mockReturnValue(api)
    const wrapper = mount(HostDetailView, { global: { stubs } })
    expect(wrapper.text()).toContain('Chargement...')
  })

  it('shows the outdated-agent title when the agent needs an update', () => {
    const api = baseUseHostDetail()
    api.isAgentUpToDate = vi.fn(() => false)
    useHostDetailMock.mockReturnValue(api)
    const wrapper = mount(HostDetailView, { global: { stubs } })
    expect(wrapper.find('[title="Mise à jour de l\'agent disponible"]').exists()).toBe(true)
  })

  it('shows the update-agent button only when canUpdateAgent is true', () => {
    const api = baseUseHostDetail()
    api.canUpdateAgent = computed(() => false)
    useHostDetailMock.mockReturnValue(api)
    const wrapper = mount(HostDetailView, { global: { stubs } })
    expect(wrapper.text()).not.toContain('Mettre à jour l\'agent')
  })

  it('shows the delete button only for admins', () => {
    const nonAdmin = mount(HostDetailView, { global: { stubs } })
    expect(nonAdmin.text()).not.toContain('Supprimer')

    const api = baseUseHostDetail()
    api.auth = reactive({ isAdmin: true, username: 'alice' })
    useHostDetailMock.mockReturnValue(api)
    const admin = mount(HostDetailView, { global: { stubs } })
    expect(admin.text()).toContain('Supprimer')
  })

  it('sets isEditing to true when clicking Modifier', async () => {
    const api = baseUseHostDetail()
    useHostDetailMock.mockReturnValue(api)
    const wrapper = mount(HostDetailView, { global: { stubs } })
    const editBtn = wrapper.findAll('button').find((b) => b.text().includes('Modifier'))
    await editBtn!.trigger('click')
    expect(api.isEditing.value).toBe(true)
  })
})

describe('HostDetailView — Proxmox link panel', () => {
  it('shows the suggested-link banner with confirm/ignore actions', async () => {
    const api = baseUseHostDetail()
    api.proxmoxLink = ref({ status: 'suggested', guest_name: 'vm-web', guest_id: 'g1', guest_type: 'lxc', node_name: 'pve1', vmid: 101 })
    useHostDetailMock.mockReturnValue(api)
    const wrapper = mount(HostDetailView, { global: { stubs } })

    expect(wrapper.text()).toContain('Suggestion')
    const confirmBtn = wrapper.findAll('button').find((b) => b.text() === 'Confirmer')
    await confirmBtn!.trigger('click')
    expect(api.confirmLink).toHaveBeenCalled()

    const ignoreBtn = wrapper.findAll('button').find((b) => b.text() === 'Ignorer')
    await ignoreBtn!.trigger('click')
    expect(api.ignoreLink).toHaveBeenCalled()
  })

  it('shows the metrics source selector and live metrics for a confirmed link', () => {
    const api = baseUseHostDetail()
    api.proxmoxLink = ref({
      status: 'confirmed', guest_name: 'vm-web', guest_id: 'g1', guest_type: 'lxc', node_name: 'pve1', vmid: 101,
      metrics_source: 'proxmox', cpu_usage: 0.42, mem_usage: 512, mem_alloc: 1024, disk_usage: 100, disk_alloc: 200,
    })
    useHostDetailMock.mockReturnValue(api)
    const wrapper = mount(HostDetailView, { global: { stubs } })

    expect(wrapper.text()).toContain('Lié')
    expect(wrapper.text()).toContain('Source métriques')
    expect(wrapper.text()).toContain('42.0%')
  })

  it('shows the "link to Proxmox" button when no link exists and showLinkButton is true', () => {
    const api = baseUseHostDetail()
    api.showLinkButton = ref(true)
    useHostDetailMock.mockReturnValue(api)
    const wrapper = mount(HostDetailView, { global: { stubs } })
    expect(wrapper.text()).toContain('Lier à Proxmox')
  })

  it('opens the manual link form and shows candidates', async () => {
    const api = baseUseHostDetail()
    api.showLinkButton = ref(true)
    api.showLinkForm = ref(true)
    api.linkCandidates = ref([{ id: 'g2', name: 'vm-db', vmid: 102, guest_type: 'qemu', node_name: 'pve1' }])
    useHostDetailMock.mockReturnValue(api)
    const wrapper = mount(HostDetailView, { global: { stubs } })

    expect(wrapper.text()).toContain('Lier cet hôte à un guest Proxmox')
    expect(wrapper.text()).toContain('vm-db')
  })

  it('shows the empty-candidates message and a loading state', () => {
    const api = baseUseHostDetail()
    api.showLinkForm = ref(true)
    api.linkCandidatesLoading = ref(true)
    useHostDetailMock.mockReturnValue(api)
    const wrapper = mount(HostDetailView, { global: { stubs } })
    expect(wrapper.text()).toContain('Chargement...')
  })
})

describe('HostDetailView — overview KPIs and alerts', () => {
  it('shows APT/Docker/tasks/commands KPI counts and switches tabs via the "voir" links', async () => {
    const api = baseUseHostDetail()
    api.aptStatus = ref({ pending_packages: 3, security_updates: 1 })
    api.containers = ref([{ state: 'running' }, { state: 'exited' }] as never)
    api.cmdHistory = ref([{ id: 'c1' }] as never)
    useHostDetailMock.mockReturnValue(api)
    const wrapper = mount(HostDetailView, { global: { stubs } })

    expect(wrapper.text()).toContain('3')
    expect(wrapper.text()).toContain('1 sécurité')

    const voirLinks = wrapper.findAll('a').filter((a) => a.text() === 'voir')
    await voirLinks[0].trigger('click')
    expect(api.activeTab.value).toBe('apt')
  })

  it('shows the no-active-alerts empty state, then a real incident row', () => {
    const empty = mount(HostDetailView, { global: { stubs } })
    expect(empty.text()).toContain('Aucune alerte active sur cet hôte.')

    const api = baseUseHostDetail()
    api.hostActiveIncidents = ref([{ id: 'i1', severity: 'crit', rule_name: 'CPU high', triggered_at: new Date().toISOString() }] as never)
    useHostDetailMock.mockReturnValue(api)
    const wrapper = mount(HostDetailView, { global: { stubs } })
    expect(wrapper.text()).toContain('CPU high')
  })

  it('shows the agent-offline warning on the metrics tab when the host is not online', async () => {
    const api = baseUseHostDetail()
    api.host = ref({ id: 'h1', name: 'web-01', status: 'offline' } as never)
    api.activeTab = ref('metrics')
    useHostDetailMock.mockReturnValue(api)
    const wrapper = mount(HostDetailView, { global: { stubs } })
    expect(wrapper.text()).toContain('Agent hors ligne')
  })
})

describe('HostDetailView — tabs', () => {
  it('shows badges for incidents, docker containers, apt security updates, custom/scheduled tasks', () => {
    const api = baseUseHostDetail()
    api.hostActiveIncidents = ref([{ id: 'i1' }] as never)
    api.containers = ref([{ state: 'running' }] as never)
    api.aptStatus = ref({ pending_packages: 2, security_updates: 5 })
    api.customTasksCount = ref(4)
    api.tasksCount = ref(7)
    useHostDetailMock.mockReturnValue(api)
    const wrapper = mount(HostDetailView, { global: { stubs } })
    const navText = wrapper.find('.nav-tabs').text()
    expect(navText).toContain('1')
    expect(navText).toContain('5')
    expect(navText).toContain('4')
    expect(navText).toContain('7')
  })

  it('shows the Système/Processus tabs only when canRunApt is true', () => {
    const api = baseUseHostDetail()
    api.canRunApt = ref(false)
    useHostDetailMock.mockReturnValue(api)
    const wrapper = mount(HostDetailView, { global: { stubs } })
    expect(wrapper.find('.nav-tabs').text()).not.toContain('Système')
  })
})

describe('HostDetailView — permissions tab', () => {
  it('shows the permissions table for an admin and revokes a permission', async () => {
    const api = baseUseHostDetail()
    api.auth = reactive({ isAdmin: true, username: 'alice' })
    api.activeTab = ref('securite')
    api.hostPerms = ref([{ username: 'bob', level: 'operator' }] as never)
    useHostDetailMock.mockReturnValue(api)
    const wrapper = mount(HostDetailView, { global: { stubs } })

    expect(wrapper.text()).toContain('bob')
    await wrapper.find('[title="Révoquer"]').trigger('click')
    expect(api.revokePermission).toHaveBeenCalledWith('bob')
  })

  it('shows the no-restrictions empty state when there are no permissions', () => {
    const api = baseUseHostDetail()
    api.auth = reactive({ isAdmin: true, username: 'alice' })
    api.activeTab = ref('securite')
    useHostDetailMock.mockReturnValue(api)
    const wrapper = mount(HostDetailView, { global: { stubs } })
    expect(wrapper.text()).toContain('Aucune restriction')
  })

  it('opens the add-permission modal and saves a new permission', async () => {
    const api = baseUseHostDetail()
    api.auth = reactive({ isAdmin: true, username: 'alice' })
    api.activeTab = ref('securite')
    api.addPermModal = ref(true)
    api.newPermUsername = ref('carol')
    api.availableUsers = ref([{ username: 'carol' }] as never)
    useHostDetailMock.mockReturnValue(api)
    const wrapper = mount(HostDetailView, { global: { stubs } })

    expect(wrapper.text()).toContain('Ajouter une permission')
    await wrapper.find('.modal-footer .btn-primary').trigger('click')
    expect(api.savePermission).toHaveBeenCalled()
  })
})
