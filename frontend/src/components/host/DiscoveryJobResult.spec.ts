import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import DiscoveryJobResult from './DiscoveryJobResult.vue'
import type { DiscoveryJob } from '../../types/discovery'

const base = { id: 'j', host_id: 'h', status: 'completed', created_at: '' }

function render(job: Partial<DiscoveryJob>) {
  return mount(DiscoveryJobResult, { props: { job: { ...base, kind: 'arp', ...job } as DiscoveryJob } })
}

describe('DiscoveryJobResult', () => {
  beforeEach(() => setLocale('fr'))

  it('shows failures and cancellations', () => {
    expect(render({ status: 'failed', error: 'cancelled' }).text()).toContain('Découverte annulée')
    expect(render({ status: 'failed', error: 'SNMP timeout' }).text()).toContain('Échec de la découverte : SNMP timeout')
    expect(render({ status: 'failed' }).text()).toContain('—')
  })

  it('renders the ARP table with vendor hints and inventory badges', () => {
    const w = render({
      arp: {
        truncated: true,
        entries: [
          { ip: '10.0.0.1', mac: 'bc:24:11:00:00:01', interface: 'eth0', vendor_hint: 'Proxmox VE', locally_administered: false, already_registered: true, existing_host_name: 'pve' },
          { ip: '10.0.0.2', mac: 'a6:00:00:00:00:01', interface: 'eth0', locally_administered: true, already_registered: true },
          { ip: '10.0.0.3', mac: '00:11:22:33:44:55', interface: 'eth1', locally_administered: false, already_registered: false },
        ],
      },
    })
    const text = w.text()
    expect(text).toContain('3 voisins')
    expect(text).toContain('liste tronquée')
    expect(text).toContain('Proxmox VE')
    expect(text).toContain('MAC locale')
    expect(text).toContain('Enregistré — pve')
    expect(text).toContain('Nouveau')
    expect(w.findAll('tbody tr')).toHaveLength(3)
    expect(w.find('table').element.closest('.table-responsive')).not.toBeNull()
  })

  it('shows an empty ARP table', () => {
    expect(render({ arp: { entries: [] } }).text()).toContain('Table ARP vide')
  })

  it('renders port-scan results, partial or empty', () => {
    const scan = { cidr: '10.0.0.0/24', hosts_scanned: 254, ports_scanned: 2, probes_done: 100, probes_total: 508, cancelled: true, timed_out: false, duration_ms: 65000 }
    let w = render({ kind: 'port_scan', port_scan: { ...scan, hosts: [{ ip: '10.0.0.5', open_ports: [22, 443], already_registered: false }] } })
    expect(w.text()).toContain('Scan annulé : résultat partiel (100 / 508 sondes)')
    expect(w.findAll('.badge.font-monospace').map((b) => b.text())).toEqual(['22', '443'])

    w = render({ kind: 'port_scan', port_scan: { ...scan, cancelled: false, timed_out: true, hosts: [] } })
    expect(w.text()).toContain('Durée maximale atteinte')
    expect(w.text()).toContain('Aucun port ouvert')

    w = render({ kind: 'port_scan', port_scan: { ...scan, cancelled: false, hosts: [] } })
    expect(w.find('.alert').exists()).toBe(false)
  })

  it('renders the SNMP system group and interfaces', () => {
    const snmp = { target: '10.0.0.9', version: '2c', sys_descr: 'Linux nas', sys_object_id: '.1.3.6.1.4.1.8072', sys_uptime_seconds: 90061, sys_contact: '', sys_name: 'nas', sys_location: 'rack' }
    let w = render({
      kind: 'snmp',
      snmp: {
        ...snmp,
        interfaces_truncated: true,
        interfaces: [
          { index: 1, descr: 'lo', speed_bps: 10_000_000, admin_status: 'up', oper_status: 'up' },
          { index: 2, descr: 'eth0', speed_bps: 0, phys_address: 'aa:bb:cc:dd:ee:ff', admin_status: 'up', oper_status: 'down' },
        ],
      },
    })
    const text = w.text()
    expect(text).toContain('nas')
    expect(text).toContain('Linux nas')
    expect(text).toContain('1j 1h')
    expect(text).toContain('v2c — 10.0.0.9')
    expect(text).toContain('2 interfaces')
    expect(text).toContain('10 Mb/s')
    expect(text).toContain('down (admin up)')
    expect(w.findAll('.bg-success-lt')).toHaveLength(1)

    w = render({ kind: 'snmp', snmp: { ...snmp, interfaces_error: 'timeout' } })
    expect(w.text()).toContain('Résumé des interfaces indisponible : timeout')
    expect(w.find('table').exists()).toBe(false)
  })

  it('renders nothing for a job still running', () => {
    expect(render({ status: 'running' }).text()).toBe('')
  })
})
