import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../i18n'

const {
  getProxmoxNode, getProxmoxNodeSensorSourceCandidates, getProxmoxNodeStatus,
  getProxmoxNodeRRD, getProxmoxNodeCpuTempHistory, getProxmoxNodeFanRPMHistory,
  getProxmoxNodes, getProxmoxNodeGuestNetworks, getProxmoxNodeGuestExposure,
  getProxmoxBackupJobs, getProxmoxBackupRuns, setProxmoxNodeSensorSource,
  refreshProxmoxNodeApt, getProxmoxNodeServices, proxmoxNodeServiceAction,
  migrateProxmoxGuest, getProxmoxTaskLog,
} = vi.hoisted(() => ({
  getProxmoxNode: vi.fn(),
  getProxmoxNodeSensorSourceCandidates: vi.fn(),
  getProxmoxNodeStatus: vi.fn(),
  getProxmoxNodeRRD: vi.fn(),
  getProxmoxNodeCpuTempHistory: vi.fn(),
  getProxmoxNodeFanRPMHistory: vi.fn(),
  getProxmoxNodes: vi.fn(),
  getProxmoxNodeGuestNetworks: vi.fn(),
  getProxmoxNodeGuestExposure: vi.fn(),
  getProxmoxBackupJobs: vi.fn(),
  getProxmoxBackupRuns: vi.fn(),
  setProxmoxNodeSensorSource: vi.fn(),
  refreshProxmoxNodeApt: vi.fn(),
  getProxmoxNodeServices: vi.fn(),
  proxmoxNodeServiceAction: vi.fn(),
  migrateProxmoxGuest: vi.fn(),
  getProxmoxTaskLog: vi.fn(async () => ({ data: [{ t: 'TASK OK' }] })),
}))

let routeQuery: Record<string, string> = {}

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'node-1' }, query: routeQuery }),
  useRouter: () => ({ replace: vi.fn() }),
}))

vi.mock('../api', () => ({
  default: {
    getProxmoxNode, getProxmoxNodeSensorSourceCandidates, getProxmoxNodeStatus,
    getProxmoxNodeRRD, getProxmoxNodeCpuTempHistory, getProxmoxNodeFanRPMHistory,
    getProxmoxNodes, getProxmoxNodeGuestNetworks, getProxmoxNodeGuestExposure,
    getProxmoxBackupJobs, getProxmoxBackupRuns, setProxmoxNodeSensorSource,
    refreshProxmoxNodeApt, getProxmoxNodeServices, proxmoxNodeServiceAction,
    migrateProxmoxGuest, getProxmoxTaskLog,
  },
}))

vi.mock('../api/client', () => ({
  getApiErrorMessage: (e: unknown, fallback: string) => (e instanceof Error ? e.message : fallback),
}))

import { useProxmoxNode } from './useProxmoxNode'

function mountUseProxmoxNode() {
  let api!: ReturnType<typeof useProxmoxNode>
  const wrapper = mount({
    setup() {
      api = useProxmoxNode()
      return () => null
    },
  })
  return { wrapper, api: api! }
}

const node = {
  id: 'node-1',
  connection_id: 'conn-1',
  node_name: 'pve-a',
  cpu_temp_source_host_id: 'host-sensor',
  fan_rpm_source_host_id: 'host-sensor',
  guests: [],
  tasks: [],
}

describe('useProxmoxNode — RRD/temperature/fan chart building', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    routeQuery = {}
    getProxmoxNode.mockResolvedValue({ data: node })
    getProxmoxNodeSensorSourceCandidates.mockResolvedValue({ data: [] })
    getProxmoxNodeStatus.mockResolvedValue({ data: {} })
    getProxmoxNodeRRD.mockResolvedValue({ data: [] })
    getProxmoxNodeCpuTempHistory.mockResolvedValue({ data: [] })
    getProxmoxNodeFanRPMHistory.mockResolvedValue({ data: [] })
    getProxmoxNodes.mockResolvedValue({ data: [] })
    getProxmoxNodeGuestNetworks.mockResolvedValue({ data: {} })
    getProxmoxNodeGuestExposure.mockResolvedValue({ data: {} })
    getProxmoxBackupJobs.mockResolvedValue({ data: [] })
    getProxmoxBackupRuns.mockResolvedValue({ data: [] })
    setProxmoxNodeSensorSource.mockResolvedValue({ data: {} })
    refreshProxmoxNodeApt.mockResolvedValue({ data: {} })
    getProxmoxNodeServices.mockResolvedValue({ data: [] })
    proxmoxNodeServiceAction.mockResolvedValue({ data: {} })
    migrateProxmoxGuest.mockResolvedValue({ data: {} })
    getProxmoxTaskLog.mockResolvedValue({ data: [{ t: 'TASK OK' }] })
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('builds one series per RRD metric present in the response, keyed on time*1000', async () => {
    getProxmoxNodeRRD.mockResolvedValue({
      data: [
        { time: 1000, cpu: 0.25, memused: 512, memtotal: 1024, iowait: 0.02, netin: 100, netout: 50 },
        { time: 1060, cpu: 0.5, memused: 768, memtotal: 1024, iowait: null, netin: 200, netout: null },
      ],
    })

    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()

    expect(api.rrdCpuChart.value).toEqual([
      { name: 'CPU', data: [{ x: 1000000, y: 25 }, { x: 1060000, y: 50 }], color: expect.any(String) },
    ])
    expect(api.rrdRamChart.value).toEqual([
      { name: 'RAM', data: [{ x: 1000000, y: 50 }, { x: 1060000, y: 75 }], color: expect.any(String) },
    ])
    // iowait only present on the first point.
    expect(api.rrdIowaitChart.value).toEqual([
      { name: 'IO Wait', data: [{ x: 1000000, y: 2 }], color: expect.any(String) },
    ])
    // netin has 2 points, netout only 1 (null on the second) — both series
    // still get built since at least one side has data.
    expect(api.rrdNetChart.value).toEqual([
      { name: 'Entrante', data: [{ x: 1000000, y: 100 }, { x: 1060000, y: 200 }], color: expect.any(String) },
      { name: 'Sortante', data: [{ x: 1000000, y: 50 }], color: expect.any(String) },
    ])
  })

  it('leaves RAM/IOWait/Net charts null when the RRD response carries none of that data', async () => {
    getProxmoxNodeRRD.mockResolvedValue({ data: [{ time: 1000, cpu: 0.1 }] })

    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()

    expect(api.rrdCpuChart.value).not.toBeNull()
    expect(api.rrdRamChart.value).toBeNull()
    expect(api.rrdIowaitChart.value).toBeNull()
    expect(api.rrdNetChart.value).toBeNull()
  })

  it('resets every RRD chart to null when the RRD fetch fails', async () => {
    getProxmoxNodeRRD.mockRejectedValue(new Error('rrd down'))

    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()

    expect(api.rrdCpuChart.value).toBeNull()
    expect(api.rrdError.value).toBe('rrd down')
  })

  it('builds the CPU temperature series and current-value KPI from history points, filtering non-positive readings', async () => {
    getProxmoxNodeCpuTempHistory.mockResolvedValue({
      data: [
        { timestamp: '2026-08-24T10:00:00Z', cpu_temperature: 0 }, // filtered out (not > 0)
        { timestamp: '2026-08-24T11:00:00Z', cpu_temperature: 45.5 },
        { timestamp: '2026-08-24T12:00:00Z', cpu_temperature: 48 },
      ],
    })

    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()

    expect(api.nodeTempChart.value).toHaveLength(1)
    expect((api.nodeTempChart.value?.[0] as { data: unknown[] } | undefined)?.data).toHaveLength(2)
    expect(api.nodeCpuTempCurrent.value).toBe(48)
  })

  it('skips the temperature/fan fetch entirely when no sensor source host is configured', async () => {
    getProxmoxNode.mockResolvedValue({ data: { ...node, cpu_temp_source_host_id: '', fan_rpm_source_host_id: '' } })

    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()

    expect(getProxmoxNodeCpuTempHistory).not.toHaveBeenCalled()
    expect(getProxmoxNodeFanRPMHistory).not.toHaveBeenCalled()
    expect(api.nodeTempChart.value).toBeNull()
    expect(api.nodeFanChart.value).toBeNull()
  })

  it('builds the fan RPM series and current-value KPI the same way as CPU temperature', async () => {
    getProxmoxNodeFanRPMHistory.mockResolvedValue({
      data: [
        { timestamp: '2026-08-24T11:00:00Z', fan_rpm: 1200 },
        { timestamp: '2026-08-24T12:00:00Z', fan_rpm: 1500 },
      ],
    })

    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()

    expect(api.nodeFanChart.value).toHaveLength(1)
    expect(api.nodeFanRPMCurrent.value).toBe(1500)
  })

  it('re-fetches RRD/temp/fan for a new timeframe via loadRRD()', async () => {
    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()
    vi.clearAllMocks()
    getProxmoxNodeRRD.mockResolvedValue({ data: [] })
    getProxmoxNodeCpuTempHistory.mockResolvedValue({ data: [] })
    getProxmoxNodeFanRPMHistory.mockResolvedValue({ data: [] })

    await api.loadRRD('week')

    expect(api.rrdTimeframe.value).toBe('week')
    expect(getProxmoxNodeRRD).toHaveBeenCalledWith('node-1', 'week')
    expect(getProxmoxNodeCpuTempHistory).toHaveBeenCalledWith('node-1', 24 * 7)
    expect(getProxmoxNodeFanRPMHistory).toHaveBeenCalledWith('node-1', 24 * 7)
  })

  it('counts a stopped task with a non-OK exit status as failed, and excludes a successful one', async () => {
    getProxmoxNode.mockResolvedValue({
      data: {
        ...node,
        tasks: [
          { upid: 't1', status: 'stopped', exit_status: 'OK' },
          { upid: 't2', status: 'stopped', exit_status: 'unable to parse' },
          { upid: 't3', status: 'running', exit_status: '' },
        ],
      },
    })

    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()

    expect(api.failedTaskCount.value).toBe(1)
  })

  it('surfaces the translated fallback error when the node itself fails to load', async () => {
    getProxmoxNode.mockRejectedValue('network down')

    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()

    expect(api.error.value).toBe('Erreur lors du chargement.')
  })

  it('surfaces the translated fallback error when the CPU temperature history fetch fails', async () => {
    getProxmoxNodeCpuTempHistory.mockRejectedValue('boom')

    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()

    expect(api.nodeTempError.value).toBe('Erreur lors du chargement de la température CPU.')
  })

  it('surfaces the translated fallback error when the fan RPM history fetch fails', async () => {
    getProxmoxNodeFanRPMHistory.mockRejectedValue('boom')

    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()

    expect(api.nodeFanError.value).toBe('Erreur lors du chargement des RPM ventilateurs.')
  })

  it('shows a translated success message and refreshes temp/fan history when saving the sensor source succeeds', async () => {
    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()
    getProxmoxNodeCpuTempHistory.mockClear()
    getProxmoxNodeFanRPMHistory.mockClear()
    setProxmoxNodeSensorSource.mockResolvedValue({ data: { cpu_temp_source_host_id: 'host-sensor-2', cpu_temp_source_host_name: 'sensor-host-2' } })

    await api.saveSensorSource()

    expect(api.sensorSourceMsg.value).toBe('Source capteurs mise à jour (CPU + ventilateurs).')
    expect(api.sensorSourceOk.value).toBe(true)
    expect(getProxmoxNodeCpuTempHistory).toHaveBeenCalled()
    expect(getProxmoxNodeFanRPMHistory).toHaveBeenCalled()
  })

  it('surfaces the translated fallback error when saving the sensor source fails', async () => {
    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()
    setProxmoxNodeSensorSource.mockRejectedValue('boom')

    await api.saveSensorSource()

    expect(api.sensorSourceMsg.value).toBe("Erreur lors de la mise à jour.")
    expect(api.sensorSourceOk.value).toBe(false)
  })

  it('surfaces the server-provided message when the live status fetch fails with a response error', async () => {
    getProxmoxNodeStatus.mockRejectedValue({ response: { data: { error: 'node unreachable' }, status: 502 } })

    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()

    expect(api.liveStatusError.value).toBe('node unreachable')
  })

  it('falls back to the translated connectivity-check message when the live status fetch fails without a response body', async () => {
    getProxmoxNodeStatus.mockRejectedValue({})

    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()

    expect(api.liveStatusError.value).toContain('vérifiez la connectivité au nœud.')
  })

  it('shows the translated "task launched" message (no logs) when apt update dispatches without a upid', async () => {
    refreshProxmoxNodeApt.mockResolvedValue({ data: {} })
    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()

    await api.triggerAptRefresh()

    expect(api.aptRefreshMsg.value).toBe('Tâche lancée.')
    expect(api.aptRefreshOk.value).toBe(true)
  })

  it('shows the translated "logs in progress" message when apt update dispatches with a upid', async () => {
    refreshProxmoxNodeApt.mockResolvedValue({ data: { upid: 'UPID:apt' } })
    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()

    await api.triggerAptRefresh()

    expect(api.aptRefreshMsg.value).toBe('Tâche lancée — logs en cours…')
  })

  it('surfaces the translated fallback error when triggering apt update fails', async () => {
    refreshProxmoxNodeApt.mockRejectedValue('boom')
    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()

    await api.triggerAptRefresh()

    expect(api.aptRefreshMsg.value).toBe("Erreur lors du lancement de apt update.")
    expect(api.aptRefreshOk.value).toBe(false)
  })

  it('surfaces the translated fallback error when the services list fails to load', async () => {
    getProxmoxNodeServices.mockRejectedValue('boom')
    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()
    routeQuery = { tab: 'services' }

    await api.loadServices()

    expect(api.servicesError.value).toBe('Erreur lors du chargement des services.')
  })

  it('surfaces the translated fallback error when backups fail to load', async () => {
    getProxmoxBackupJobs.mockRejectedValue('boom')
    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()

    await api.loadBackups()

    expect(api.backupsError.value).toBe('Erreur lors du chargement des sauvegardes.')
  })

  it('shows the translated "launched" message (no logs) when a service action dispatches without a upid', async () => {
    proxmoxNodeServiceAction.mockResolvedValue({ data: {} })
    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()

    await api.svcAction('sshd', 'restart')

    expect(api.svcActionMsg.value).toBe('restart sshd lancé.')
    expect(api.svcActionOk.value).toBe(true)
  })

  it('shows the translated "logs in progress" message when a service action dispatches with a upid', async () => {
    proxmoxNodeServiceAction.mockResolvedValue({ data: { upid: 'UPID:svc' } })
    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()

    await api.svcAction('sshd', 'restart')

    expect(api.svcActionMsg.value).toBe('restart sshd lancé — logs en cours…')
  })

  it('surfaces the translated fallback error when a service action fails', async () => {
    proxmoxNodeServiceAction.mockRejectedValue('boom')
    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()

    await api.svcAction('sshd', 'restart')

    expect(api.svcActionMsg.value).toBe('Erreur lors de restart sshd.')
    expect(api.svcActionOk.value).toBe(false)
  })

  it('surfaces the translated fallback error when submitting a guest migration fails', async () => {
    migrateProxmoxGuest.mockRejectedValue('boom')
    const { api } = mountUseProxmoxNode()
    await flushPromises()
    await flushPromises()
    api.openMigrateModal({ vmid: 100, name: 'web-01' }, 'vm')
    api.migrateModal.value.target = 'pve-b'

    await api.submitMigration()

    expect(api.migrateModal.value.error).toBe('Erreur lors du lancement de la migration.')
    expect(api.migrateModal.value.loading).toBe(false)
  })
})
