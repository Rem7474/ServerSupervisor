import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import NetworkDiscoveryPanel from './NetworkDiscoveryPanel.vue'
import AgentDiscoveryPanel from './AgentDiscoveryPanel.vue'

const { discoverHosts, registerHostsBulk } = vi.hoisted(() => ({
  discoverHosts: vi.fn(),
  registerHostsBulk: vi.fn(),
}))

vi.mock('../../api', () => ({
  default: { discoverHosts, registerHostsBulk },
}))
vi.mock('../../api/client', () => ({
  getApiErrorMessage: (e: unknown, fallback: string) => (e instanceof Error ? e.message : fallback),
}))

async function scan(wrapper: ReturnType<typeof mount>): Promise<void> {
  await wrapper.find('#scan-cidr').setValue('10.0.0.0/29')
  await wrapper.find('form').trigger('submit')
  await flushPromises()
}

describe('NetworkDiscoveryPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } })
  })

  it('shows an empty state when the scan finds nothing', async () => {
    discoverHosts.mockResolvedValue({ data: { results: [] } })
    const wrapper = mount(NetworkDiscoveryPanel)

    await scan(wrapper)

    expect(discoverHosts).toHaveBeenCalledWith('10.0.0.0/29')
    expect(wrapper.text()).toContain('Aucune adresse trouvée')
  })

  it('surfaces a scan error', async () => {
    discoverHosts.mockRejectedValue(new Error('scan failed'))
    const wrapper = mount(NetworkDiscoveryPanel)

    await scan(wrapper)

    expect(wrapper.find('.alert-danger').text()).toBe('scan failed')
  })

  it('renders responded, already-registered and unreachable rows with the right badges', async () => {
    discoverHosts.mockResolvedValue({
      data: {
        results: [
          { ip_address: '10.0.0.1', responded: true, latency_ms: 4, already_registered: false },
          { ip_address: '10.0.0.2', responded: true, already_registered: true, existing_host_name: 'web' },
          { ip_address: '10.0.0.3', responded: false, already_registered: false },
        ],
      },
    })
    const wrapper = mount(NetworkDiscoveryPanel)

    await scan(wrapper)

    expect(wrapper.text()).toContain('Répond (4 ms)')
    expect(wrapper.text()).toContain('Déjà enregistré — web')
    expect(wrapper.text()).toContain('Ne répond pas')
    expect(wrapper.text()).toContain('3 adresses scannées, 1 nouvelle disponible')
  })

  it('selects all candidates, deselects one, and submits the bulk add with edited names', async () => {
    discoverHosts.mockResolvedValue({
      data: {
        results: [
          { ip_address: '10.0.0.1', responded: true, already_registered: false },
          { ip_address: '10.0.0.2', responded: true, already_registered: false },
        ],
      },
    })
    registerHostsBulk.mockResolvedValue({
      data: {
        results: [
          { name: 'switch-1', ip_address: '10.0.0.1', created: true, api_key: 'secret-key' },
        ],
      },
    })
    const wrapper = mount(NetworkDiscoveryPanel)
    await scan(wrapper)

    await wrapper.find('button.btn-ghost-secondary').trigger('click')
    expect(wrapper.text()).toContain('Tout désélectionner')

    const nameInputs = wrapper.findAll('input.form-control-sm')
    await nameInputs[1].setValue('')
    await wrapper.find('input[aria-label="Sélectionner 10.0.0.2"]').trigger('change')

    await nameInputs[0].setValue('switch-1')

    const addButton = wrapper.findAll('button').find((b) => b.text().includes('Ajouter'))
    expect(addButton?.text()).toContain('Ajouter 1 hôte sélectionné')
    await addButton?.trigger('click')
    await flushPromises()

    expect(registerHostsBulk).toHaveBeenCalledWith([{ name: 'switch-1', ip_address: '10.0.0.1' }])
    expect(wrapper.text()).toContain('1 hôte ajouté')
    expect(wrapper.text()).toContain('secret-key')
  })

  it('copies an API key, toggling the button label, and emits done on finish', async () => {
    discoverHosts.mockResolvedValue({
      data: { results: [{ ip_address: '10.0.0.1', responded: true, already_registered: false }] },
    })
    registerHostsBulk.mockResolvedValue({
      data: { results: [{ name: '10.0.0.1', ip_address: '10.0.0.1', created: true, api_key: 'secret-key' }] },
    })
    const wrapper = mount(NetworkDiscoveryPanel)
    await scan(wrapper)
    await wrapper.find('input[aria-label="Sélectionner 10.0.0.1"]').trigger('change')
    const addButton = wrapper.findAll('button').find((b) => b.text().includes('Ajouter'))
    await addButton?.trigger('click')
    await flushPromises()

    const copyButton = wrapper.findAll('button').find((b) => b.text() === 'Copier')
    expect(copyButton).toBeTruthy()
    await copyButton?.trigger('click')
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('secret-key')

    await wrapper.find('button.btn-success').trigger('click')
    expect(wrapper.emitted('done')).toBeTruthy()
  })

  it('shows a per-item error for a failed bulk creation', async () => {
    discoverHosts.mockResolvedValue({
      data: { results: [{ ip_address: '10.0.0.1', responded: true, already_registered: false }] },
    })
    registerHostsBulk.mockResolvedValue({
      data: { results: [{ name: '10.0.0.1', ip_address: '10.0.0.1', created: false, error: 'IP already exists' }] },
    })
    const wrapper = mount(NetworkDiscoveryPanel)
    await scan(wrapper)
    await wrapper.find('input[aria-label="Sélectionner 10.0.0.1"]').trigger('change')
    const addButton = wrapper.findAll('button').find((b) => b.text().includes('Ajouter'))
    await addButton?.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('IP already exists')
  })

  it('switches between the ping sweep and the agent tools with tabs', async () => {
    const wrapper = mount(NetworkDiscoveryPanel, { global: { stubs: { AgentDiscoveryPanel: true } } })
    const tabs = wrapper.findAll('[role="tab"]')
    expect(tabs.map((t) => t.text())).toEqual(['Ping (serveur)', 'Table ARP', 'Scan de ports', 'SNMP'])
    expect(tabs[0].attributes('aria-selected')).toBe('true')
    // The agent panel is only mounted once an agent tool is opened.
    expect(wrapper.findComponent(AgentDiscoveryPanel).exists()).toBe(false)

    await tabs[2].trigger('click')
    const agent = wrapper.findComponent(AgentDiscoveryPanel)
    expect(agent.props('tool')).toBe('port_scan')
    const panels = wrapper.findAll('[role="tabpanel"]')
    expect(panels[1].attributes('style')).toContain('display: none')
    expect(panels[0].attributes('aria-labelledby')).toBe(tabs[2].attributes('id'))

    await tabs[2].trigger('keydown', { key: 'ArrowRight' })
    expect(agent.props('tool')).toBe('snmp')
    await wrapper.findAll('[role="tab"]')[3].trigger('keydown', { key: 'ArrowRight' })
    expect(wrapper.findAll('[role="tab"]')[0].attributes('aria-selected')).toBe('true')
    // Back on ping: the agent panel stays mounted (a running scan survives).
    expect(wrapper.findComponent(AgentDiscoveryPanel).exists()).toBe(true)
    expect(wrapper.findComponent(AgentDiscoveryPanel).props('tool')).toBe('arp')
    await wrapper.findAll('[role="tab"]')[0].trigger('keydown', { key: 'ArrowLeft' })
    expect(wrapper.findAll('[role="tab"]')[3].attributes('aria-selected')).toBe('true')
  })
})
