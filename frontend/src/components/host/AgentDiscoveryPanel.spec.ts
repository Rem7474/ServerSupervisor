import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import AgentDiscoveryPanel from './AgentDiscoveryPanel.vue'
import type { AgentDiscoveryKind } from '../../types/discovery'

const api = vi.hoisted(() => ({
  getHosts: vi.fn(),
  getDiscoveryLimits: vi.fn(),
  startArpDiscovery: vi.fn(),
  startPortScan: vi.fn(),
  startSnmpProbe: vi.fn(),
  getDiscoveryJob: vi.fn(),
  cancelDiscoveryJob: vi.fn(),
}))
const confirm = vi.hoisted(() => vi.fn())
const openCommandStream = vi.hoisted(() => vi.fn())

vi.mock('../../api', () => ({ default: api }))
vi.mock('../../api/client', () => ({
  getApiErrorMessage: (e: unknown, fallback: string) => (e instanceof Error ? e.message : fallback),
}))
vi.mock('../../composables/useCommandStream', () => ({
  useCommandStream: () => ({ openCommandStream, closeStream: vi.fn() }),
}))
vi.mock('../../composables/useConfirmDialog', () => ({ useConfirmDialog: () => ({ confirm }) }))

const limits = { default_ports: [22, 80], max_ports: 1024, min_prefix_len: 24, allowed_subnets: ['10.0.0.0/16'] }
const scanner = { id: 'a2', name: 'pve', ip_address: '10.0.0.3', collectors: { network_discovery: true, network_scan: true } }
const passive = { id: 'a1', name: 'nas', ip_address: '10.0.0.2', collectors: { network_discovery: true } }

async function mountPanel(tool: AgentDiscoveryKind, hosts: unknown[] = [scanner, passive]) {
  api.getHosts.mockResolvedValue({ data: hosts })
  api.getDiscoveryLimits.mockResolvedValue({ data: limits })
  const wrapper = mount(AgentDiscoveryPanel, { props: { tool } })
  await flushPromises()
  return wrapper
}

describe('AgentDiscoveryPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
  })

  it('explains when no agent supports discovery', async () => {
    const w = await mountPanel('arp', [{ id: 'x', name: 'old', ip_address: '1', collectors: {} }])
    expect(w.text()).toContain('Aucun agent compatible')
  })

  it('shows a load error', async () => {
    api.getHosts.mockRejectedValue(new Error('réseau'))
    api.getDiscoveryLimits.mockResolvedValue({ data: limits })
    const w = mount(AgentDiscoveryPanel, { props: { tool: 'arp' } })
    await flushPromises()
    expect(w.find('[role="alert"]').text()).toBe('réseau')
  })

  it('labels the agent picker and marks passive-only agents', async () => {
    const w = await mountPanel('arp')
    const select = w.find('select')
    expect(w.find(`label[for="${select.attributes('id')}"]`).exists()).toBe(true)
    expect(w.findAll('option').map((o) => o.text())).toEqual(['pve (10.0.0.3)', 'nas (10.0.0.2) — ARP uniquement'])
  })

  it('reads the ARP table and shows the result', async () => {
    api.startArpDiscovery.mockResolvedValue({ data: { job_id: 'j1' } })
    api.getDiscoveryJob.mockResolvedValue({ data: { id: 'j1', kind: 'arp', status: 'completed', arp: { entries: [] } } })
    const w = await mountPanel('arp')
    await w.find('button.btn-primary').trigger('click')
    await flushPromises()
    expect(api.startArpDiscovery).toHaveBeenCalledWith('a2')
    expect(w.text()).toContain('Table ARP vide')
  })

  it('asks for confirmation before a port scan and shows progress with a cancel button', async () => {
    const w = await mountPanel('port_scan')
    expect(w.text()).toContain('Sous-réseaux autorisés : 10.0.0.0/16.')
    const submit = w.find('button[type="submit"]')
    expect(submit.attributes('disabled')).toBeDefined()

    await w.find('input.font-monospace').setValue('10.0.0.0/23')
    expect(w.text()).toContain('Plage trop large : /24 au plus.')
    await w.find('form').trigger('submit')
    expect(confirm).not.toHaveBeenCalled()

    await w.find('input.font-monospace').setValue('10.0.0.0/24')
    confirm.mockResolvedValueOnce(false)
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(confirm).toHaveBeenCalledWith(expect.objectContaining({ message: expect.stringContaining('254 adresses × 2 ports dans 10.0.0.0/24, depuis l\'agent pve') }))
    expect(api.startPortScan).not.toHaveBeenCalled()

    confirm.mockResolvedValueOnce(true)
    api.startPortScan.mockResolvedValue({ data: { job_id: 'j2' } })
    api.getDiscoveryJob.mockResolvedValue({ data: { id: 'j2', kind: 'port_scan', status: 'pending' } })
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(api.startPortScan).toHaveBeenCalledWith({ host_id: 'a2', cidr: '10.0.0.0/24', ports: '' })
    expect(w.text()).toContain('En attente de l\'agent')
    expect(w.find('select').attributes('disabled')).toBeDefined()

    api.cancelDiscoveryJob.mockResolvedValue({ data: { id: 'j2', kind: 'port_scan', status: 'failed', error: 'cancelled' } })
    await w.find('button.btn-outline-danger').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Découverte annulée')
    w.unmount()
  })

  it('shows the streamed port-scan progress', async () => {
    const w = await mountPanel('port_scan')
    await w.find('input.font-monospace').setValue('10.0.0.0/24')
    confirm.mockResolvedValueOnce(true)
    api.startPortScan.mockResolvedValue({ data: { job_id: 'j9' } })
    api.getDiscoveryJob.mockResolvedValue({ data: { id: 'j9', kind: 'port_scan', status: 'running' } })
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(w.text()).toContain('En cours sur l\'agent')
    openCommandStream.mock.calls[0][1].onChunk({ chunk: '{"done":127,"total":508,"open":3}' })
    await flushPromises()
    expect(w.text()).toContain('127 / 508 sondes — 3 ports ouverts')
    const bar = w.find('[role="progressbar"]')
    expect(bar.attributes('aria-valuenow')).toBe('127')
    expect(bar.find('.progress-bar').attributes('style')).toContain('width: 25%')
    w.unmount()
  })

  it('disables active tools on an agent without the opt-in', async () => {
    const w = await mountPanel('snmp', [passive])
    expect(w.text()).toContain('Le scan actif est désactivé sur cet agent')
    expect(w.find('code').text()).toBe('allow_network_scan: true')
    expect(w.find('fieldset').attributes('disabled')).toBeDefined()
  })

  it('sends an SNMP v3 probe with write-only credentials', async () => {
    api.startSnmpProbe.mockResolvedValue({ data: { job_id: 'j3' } })
    api.getDiscoveryJob.mockResolvedValue({ data: { id: 'j3', kind: 'snmp', status: 'running' } })
    const w = await mountPanel('snmp')
    expect(w.find('input[type="password"]').exists()).toBe(true)

    const selects = w.findAll('select')
    await selects[1].setValue('3')
    await w.find('input[autocomplete="off"][type="text"]:not(.font-monospace)').setValue('mon')
    const secrets = w.findAll('input[type="password"]')
    expect(secrets).toHaveLength(2)
    await secrets[0].setValue('pass-one')
    await secrets[1].setValue('pass-two')
    await w.find('input.font-monospace').setValue('10.0.0.9')
    await w.find('form').trigger('submit')
    await flushPromises()

    expect(api.startSnmpProbe).toHaveBeenCalledWith(expect.objectContaining({ version: '3', username: 'mon', auth_passphrase: 'pass-one', priv_passphrase: 'pass-two' }))
    for (const input of w.findAll('input[type="password"]')) {
      expect((input.element as HTMLInputElement).value).toBe('')
    }
    expect(w.text()).toContain('En cours sur l\'agent')

    // authNoPriv hides the privacy passphrase; noAuthNoPriv hides both.
    const level = w.findAll('select')[2]
    await level.setValue('authNoPriv')
    expect(w.findAll('input[type="password"]')).toHaveLength(1)
    await level.setValue('noAuthNoPriv')
    expect(w.findAll('input[type="password"]')).toHaveLength(0)
    w.unmount()
  })

  it('shows a start error', async () => {
    api.startArpDiscovery.mockRejectedValue(new Error('Agent hors ligne'))
    const w = await mountPanel('arp')
    await w.find('button.btn-primary').trigger('click')
    await flushPromises()
    expect(w.find('.alert-danger').text()).toBe('Agent hors ligne')
  })
})

