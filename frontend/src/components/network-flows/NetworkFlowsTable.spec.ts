import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import NetworkFlowsTable from './NetworkFlowsTable.vue'
import type { NetworkFlowMetric } from '../../types/networkFlows'

beforeEach(() => {
  setLocale('fr')
})

// NetworkFlowsHistoryChart fetches on mount — irrelevant to the top-talkers
// table's own sorting behavior under test here.
vi.mock('./NetworkFlowsHistoryChart.vue', () => ({
  default: { template: '<div />' },
}))

function makeTalkers(): NetworkFlowMetric[] {
  const base = { id: 0, host_id: 'h1', timestamp: '2026-08-14T12:00:00Z', packets: 0 }
  return [
    { ...base, id: 1, remote_ip: '10.0.0.5', remote_port: 443, protocol: 'tcp', direction: 'outbound', process_name: 'nginx', pid: 100, rx_bytes: 500, tx_bytes: 100, connections: 2, is_others: false },
    { ...base, id: 2, remote_ip: '10.0.0.9', remote_port: 22, protocol: 'tcp', direction: 'inbound', process_name: 'sshd', pid: 200, rx_bytes: 2000, tx_bytes: 50, connections: 1, is_others: false },
    { ...base, id: 3, remote_ip: '10.0.0.1', remote_port: 53, protocol: 'udp', direction: 'outbound', process_name: '', pid: 0, rx_bytes: 10, tx_bytes: 10, connections: 5, is_others: false },
    { ...base, id: 4, remote_ip: '', remote_port: 0, protocol: 'tcp', direction: 'outbound', process_name: '', pid: 0, rx_bytes: 0, tx_bytes: 0, connections: 3, is_others: true },
  ]
}

function mountTable() {
  return mount(NetworkFlowsTable, {
    props: { hostId: 'h1', initialData: makeTalkers() },
  })
}

describe('NetworkFlowsTable — sorting', () => {
  it('defaults to sorting by Rx descending, with the "Autres" row always last', () => {
    const wrapper = mountTable()
    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(4)
    expect(rows[0].text()).toContain('10.0.0.9') // rx_bytes 2000
    expect(rows[1].text()).toContain('10.0.0.5') // rx_bytes 500
    expect(rows[2].text()).toContain('10.0.0.1') // rx_bytes 10
    expect(rows[3].text()).toContain('Autres')
  })

  it('re-sorts by remote IP ascending when that column header is clicked, keeping "Autres" last', async () => {
    const wrapper = mountTable()
    const ipHeader = wrapper.findAll('th')[1].find('button')
    await ipHeader.trigger('click')

    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('10.0.0.1')
    expect(rows[1].text()).toContain('10.0.0.5')
    expect(rows[2].text()).toContain('10.0.0.9')
    expect(rows[3].text()).toContain('Autres')
  })

  it('reverses direction on a second click of the same column', async () => {
    const wrapper = mountTable()
    const ipHeader = wrapper.findAll('th')[1].find('button')
    await ipHeader.trigger('click') // asc
    await ipHeader.trigger('click') // desc

    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('10.0.0.9')
    expect(rows[1].text()).toContain('10.0.0.5')
    expect(rows[2].text()).toContain('10.0.0.1')
    expect(rows[3].text()).toContain('Autres')
  })
})

describe('NetworkFlowsTable', () => {
  it('shows the unavailable state when the collector is off, without fetching', () => {
    const wrapper = mount(NetworkFlowsTable, {
      props: { hostId: 'h1', collectorAvailable: false },
    })
    expect(wrapper.text()).toContain('Trafic réseau indisponible sur cet hôte')
    expect(wrapper.find('table').exists()).toBe(false)
  })

  it('shows the empty state when there are no talkers', () => {
    const wrapper = mount(NetworkFlowsTable, {
      props: { hostId: 'h1', initialData: [] },
    })
    expect(wrapper.text()).toContain('Aucun flux réseau observé')
  })

  it('pluralizes the "Autres" rollup label by connection count', () => {
    const base = { id: 0, host_id: 'h1', timestamp: '2026-08-14T12:00:00Z', packets: 0 }
    const wrapper = mount(NetworkFlowsTable, {
      props: {
        hostId: 'h1',
        initialData: [
          { ...base, id: 1, remote_ip: '', remote_port: 0, protocol: 'tcp', direction: 'outbound', process_name: '', pid: 0, rx_bytes: 0, tx_bytes: 0, connections: 1, is_others: true },
        ],
      },
    })
    expect(wrapper.text()).toContain('Autres (1 connexion)')
  })

  it('filters by search text across IP and process name', async () => {
    const wrapper = mountTable()
    await wrapper.find('input[type="text"]').setValue('sshd')

    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(1)
    expect(rows[0].text()).toContain('sshd')
  })

  it('filters by protocol', async () => {
    const wrapper = mountTable()
    await wrapper.find('select').setValue('udp')

    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(1)
    expect(rows[0].text()).toContain('10.0.0.1')
  })

  it('opens the talker drilldown modal on row click and closes it', async () => {
    const wrapper = mountTable()
    const clickableRow = wrapper.findAll('tbody tr').find((r) => r.classes().includes('clickable-row'))
    await clickableRow?.trigger('click')

    expect(wrapper.find('.modal-title').exists()).toBe(true)

    await wrapper.find('.btn-close').trigger('click')
    expect(wrapper.find('.modal').exists()).toBe(false)
  })
})
