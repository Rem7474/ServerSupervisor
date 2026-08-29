import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const {
  getProxmoxNode, getProxmoxNodeSensorSourceCandidates, getProxmoxNodeStatus,
  getProxmoxNodeRRD, getProxmoxNodeCpuTempHistory, getProxmoxNodeFanRPMHistory,
  getProxmoxNodes, getProxmoxNodeGuestNetworks, getProxmoxNodeGuestExposure,
  getProxmoxBackupJobs, getProxmoxBackupRuns,
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
    getProxmoxBackupJobs, getProxmoxBackupRuns,
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
})
