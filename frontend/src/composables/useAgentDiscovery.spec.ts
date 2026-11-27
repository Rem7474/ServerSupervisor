import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setLocale } from '../i18n'
import { cidrHostCount, countPorts, JOB_POLL_INTERVAL_MS, useAgentDiscovery } from './useAgentDiscovery'

const api = vi.hoisted(() => ({
  getHosts: vi.fn(),
  getDiscoveryLimits: vi.fn(),
  startArpDiscovery: vi.fn(),
  startPortScan: vi.fn(),
  startSnmpProbe: vi.fn(),
  getDiscoveryJob: vi.fn(),
  cancelDiscoveryJob: vi.fn(),
}))
const stream = vi.hoisted(() => ({
  openCommandStream: vi.fn(),
  closeStream: vi.fn(),
}))

vi.mock('../api', () => ({ default: api }))
vi.mock('../api/client', () => ({
  getApiErrorMessage: (e: unknown, fallback: string) => (e instanceof Error ? e.message : fallback),
}))
vi.mock('./useCommandStream', () => ({ useCommandStream: () => stream }))

const limits = { default_ports: [22, 80, 443], max_ports: 10, min_prefix_len: 24, allowed_subnets: [] }
const hosts = [
  { id: 'old', name: 'old', ip_address: '10.0.0.1', collectors: {} },
  { id: 'a1', name: 'nas', ip_address: '10.0.0.2', collectors: { network_discovery: true } },
  { id: 'a2', name: 'pve', ip_address: '10.0.0.3', collectors: { network_discovery: true, network_scan: true } },
]

async function loaded() {
  api.getHosts.mockResolvedValue({ data: hosts })
  api.getDiscoveryLimits.mockResolvedValue({ data: limits })
  const d = useAgentDiscovery()
  await d.load()
  return d
}

describe('cidrHostCount / countPorts', () => {
  it('counts scanned addresses like the agent', () => {
    expect(cidrHostCount('192.168.1.0/24')).toBe(254)
    expect(cidrHostCount('192.168.1.4/30')).toBe(2)
    expect(cidrHostCount('192.168.1.4/31')).toBe(2)
    expect(cidrHostCount('192.168.1.4/32')).toBe(1)
    expect(cidrHostCount('192.168.1.0')).toBeNull()
    expect(cidrHostCount('192.168.300.0/24')).toBeNull()
    expect(cidrHostCount('192.168.1.0/33')).toBeNull()
  })

  it('counts distinct ports, 0 for the defaults, null when invalid', () => {
    expect(countPorts('')).toBe(0)
    expect(countPorts('22, 80-82,22,')).toBe(4)
    expect(countPorts('a')).toBeNull()
    expect(countPorts('0')).toBeNull()
    expect(countPorts('90-80')).toBeNull()
    expect(countPorts('1-70000')).toBeNull()
  })
})

describe('useAgentDiscovery', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    setLocale('en')
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('lists only agents that support discovery and selects the first one', async () => {
    const d = await loaded()
    expect(d.agents.value.map((h) => h.id)).toEqual(['a1', 'a2'])
    expect(d.hostId.value).toBe('a1')
    expect(d.scanAllowed.value).toBe(false)
    d.hostId.value = 'a2'
    expect(d.scanAllowed.value).toBe(true)
  })

  it('reports a load failure', async () => {
    api.getHosts.mockRejectedValue(new Error('boom'))
    api.getDiscoveryLimits.mockResolvedValue({ data: limits })
    const d = useAgentDiscovery()
    await d.load()
    expect(d.loadError.value).toBe('boom')
    expect(d.hostsLoaded.value).toBe(true)
  })

  it('validates the port-scan form against the limits', async () => {
    const d = await loaded()
    expect(d.portScanProblem.value).toBe('')
    d.cidr.value = 'nope'
    expect(d.portScanProblem.value).toMatch(/Invalid IPv4 CIDR/)
    d.cidr.value = '10.0.0.0/23'
    expect(d.portScanProblem.value).toMatch(/\/24 at most/)
    d.cidr.value = '10.0.0.0/24'
    d.ports.value = 'x'
    expect(d.portScanProblem.value).toMatch(/Invalid port list/)
    d.ports.value = '1-11'
    expect(d.portScanProblem.value).toMatch(/10 at most/)
    d.ports.value = ''
    expect(d.effectivePortCount.value).toBe(3)
    d.ports.value = '22,80'
    expect(d.effectivePortCount.value).toBe(2)
    expect(d.addressCount.value).toBe(254)
  })

  it('follows an ARP job until it completes', async () => {
    const d = await loaded()
    api.startArpDiscovery.mockResolvedValue({ data: { job_id: 'j1' } })
    api.getDiscoveryJob
      .mockResolvedValueOnce({ data: { id: 'j1', kind: 'arp', status: 'running' } })
      .mockResolvedValueOnce({ data: { id: 'j1', kind: 'arp', status: 'completed', arp: { entries: [] } } })

    await d.startArp()
    expect(api.startArpDiscovery).toHaveBeenCalledWith('a1')
    await vi.waitFor(() => expect(d.jobs.arp?.status).toBe('running'))
    expect(d.isActive('arp')).toBe(true)
    expect(stream.openCommandStream).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(JOB_POLL_INTERVAL_MS)
    expect(d.jobs.arp?.status).toBe('completed')
    expect(d.isActive('arp')).toBe(false)
    await vi.advanceTimersByTimeAsync(JOB_POLL_INTERVAL_MS * 3)
    expect(api.getDiscoveryJob).toHaveBeenCalledTimes(2)
  })

  it('streams port-scan progress and cancels a running scan', async () => {
    const d = await loaded()
    d.hostId.value = 'a2'
    d.cidr.value = ' 10.0.0.0/24 '
    d.ports.value = '22'
    api.startPortScan.mockResolvedValue({ data: { job_id: 'j2' } })
    api.getDiscoveryJob.mockResolvedValue({ data: { id: 'j2', kind: 'port_scan', status: 'running' } })

    await d.startPortScan()
    expect(api.startPortScan).toHaveBeenCalledWith({ host_id: 'a2', cidr: '10.0.0.0/24', ports: '22' })
    const onChunk = stream.openCommandStream.mock.calls[0][1].onChunk
    onChunk({ chunk: '{"done":10,"total":254,"open":1}' })
    expect(d.progress.value).toEqual({ done: 10, total: 254, open: 1 })
    onChunk({ chunk: 'not json' })
    onChunk({ chunk: '{"other":1}' })
    expect(d.progress.value?.done).toBe(10)

    // A second start while running is ignored.
    await d.startPortScan()
    expect(api.startPortScan).toHaveBeenCalledTimes(1)

    stream.closeStream.mockClear()
    api.cancelDiscoveryJob.mockResolvedValue({ data: { id: 'j2', kind: 'port_scan', status: 'running' } })
    await d.cancel('port_scan')
    expect(api.cancelDiscoveryJob).toHaveBeenCalledWith('j2')
    expect(stream.closeStream).not.toHaveBeenCalled()

    api.cancelDiscoveryJob.mockResolvedValue({ data: { id: 'j2', kind: 'port_scan', status: 'failed', error: 'cancelled' } })
    await d.cancel('port_scan')
    expect(stream.closeStream).toHaveBeenCalled()
    expect(d.isActive('port_scan')).toBe(false)
    await d.cancel('port_scan') // finished: no-op
    expect(api.cancelDiscoveryJob).toHaveBeenCalledTimes(2)
  })

  it('reports start, poll and cancel errors', async () => {
    const d = await loaded()
    api.startArpDiscovery.mockRejectedValue(new Error('denied'))
    await d.startArp()
    expect(d.errors.arp).toBe('denied')
    expect(d.jobs.arp).toBeNull()

    api.startArpDiscovery.mockResolvedValue({ data: { job_id: 'j3' } })
    api.getDiscoveryJob.mockRejectedValue(new Error('gone'))
    await d.startArp()
    await vi.waitFor(() => expect(d.errors.arp).toBe('gone'))

    d.jobs.snmp = { id: 'j4', host_id: 'a1', kind: 'snmp', status: 'running', created_at: '' }
    api.cancelDiscoveryJob.mockRejectedValue(new Error('nope'))
    await d.cancel('snmp')
    expect(d.errors.snmp).toBe('nope')
    expect(d.cancelling.snmp).toBe(false)
  })

  it('does nothing without an agent', async () => {
    const d = useAgentDiscovery()
    await d.startArp()
    expect(api.startArpDiscovery).not.toHaveBeenCalled()
  })

  it('sends SNMP v2c credentials once and clears them', async () => {
    const d = await loaded()
    api.startSnmpProbe.mockResolvedValue({ data: { job_id: 'j5' } })
    api.getDiscoveryJob.mockResolvedValue({ data: { id: 'j5', kind: 'snmp', status: 'pending' } })
    Object.assign(d.snmp, { target: ' 10.0.0.9 ', port: 0, community: 'placeholder-value' })

    await d.startSnmp()
    expect(api.startSnmpProbe).toHaveBeenCalledWith(expect.objectContaining({
      host_id: 'a1', target: '10.0.0.9', port: 161, version: '2c', community: 'placeholder-value',
      username: '', security_level: '', auth_passphrase: '', priv_passphrase: '',
    }))
    expect(d.snmp.community).toBe('')
    d.dispose()
  })

  it('builds SNMP v3 requests per security level', async () => {
    const d = await loaded()
    api.startSnmpProbe.mockRejectedValue(new Error('x'))
    Object.assign(d.snmp, { target: '10.0.0.9', version: '3', username: ' mon ', community: 'ignored', authPassphrase: 'pass-one', privPassphrase: 'pass-two' })

    await d.startSnmp()
    expect(api.startSnmpProbe).toHaveBeenLastCalledWith(expect.objectContaining({
      community: '', username: 'mon', security_level: 'authPriv', auth_protocol: 'SHA256',
      auth_passphrase: 'pass-one', priv_protocol: 'AES', priv_passphrase: 'pass-two',
    }))
    // Kept after a refused start, so the form can be corrected and resent.
    expect(d.snmp.authPassphrase).toBe('pass-one')
    expect(d.snmp.privPassphrase).toBe('pass-two')

    Object.assign(d.snmp, { securityLevel: 'authNoPriv', authPassphrase: 'pass-one', privPassphrase: 'pass-two' })
    await d.startSnmp()
    expect(api.startSnmpProbe).toHaveBeenLastCalledWith(expect.objectContaining({
      security_level: 'authNoPriv', auth_passphrase: 'pass-one', priv_protocol: '', priv_passphrase: '',
    }))

    Object.assign(d.snmp, { securityLevel: 'noAuthNoPriv', authPassphrase: 'pass-one' })
    await d.startSnmp()
    expect(api.startSnmpProbe).toHaveBeenLastCalledWith(expect.objectContaining({
      security_level: 'noAuthNoPriv', auth_protocol: '', auth_passphrase: '',
    }))
  })

  it('ignores a stale poll answer for a replaced job', async () => {
    const d = await loaded()
    let resolveFirst: (v: unknown) => void = () => {}
    api.startArpDiscovery.mockResolvedValueOnce({ data: { job_id: 'old' } }).mockResolvedValueOnce({ data: { job_id: 'new' } })
    api.getDiscoveryJob
      .mockReturnValueOnce(new Promise((r) => { resolveFirst = r }))
      .mockResolvedValue({ data: { id: 'new', kind: 'arp', status: 'completed', arp: { entries: [] } } })

    await d.startArp()
    d.jobs.arp!.status = 'failed' // let a new job start
    await d.startArp()
    await vi.waitFor(() => expect(d.jobs.arp?.id).toBe('new'))
    resolveFirst({ data: { id: 'old', kind: 'arp', status: 'running' } })
    await vi.advanceTimersByTimeAsync(0)
    expect(d.jobs.arp?.id).toBe('new')
    expect(d.jobs.arp?.status).toBe('completed')
  })
})
