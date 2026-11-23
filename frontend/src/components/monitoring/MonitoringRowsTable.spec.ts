import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import MonitoringRowsTable from './MonitoringRowsTable.vue'
import type { MonitoringRow } from '../../composables/useMonitoringOverview'
import type { UptimeProbe } from '../../types/uptime'
import type { SSLCertificate } from '../../types/ssl'

const RouterLinkStub = { props: ['to'], template: '<a class="rl" :href="to"><slot /></a>' }

function mkProbe(over: Partial<UptimeProbe> = {}): UptimeProbe {
  return {
    id: 'p1', name: 'API', type: 'http', target: 'https://api.example.com', interval_sec: 60, timeout_sec: 10,
    expected_status: 200, expected_body_regex: '', follow_redirects: true, verify_tls: true, enabled: true,
    last_status: 'up', consecutive_failures: 0, created_at: '', updated_at: '', ...over,
  } as UptimeProbe
}
function mkCert(over: Partial<SSLCertificate> = {}): SSLCertificate {
  return {
    id: 'c1', name: 'API', host: 'api.example.com', port: 443, enabled: true, days_remaining: 30,
    created_at: '', updated_at: '', ...over,
  } as SSLCertificate
}

function mountTable(rows: MonitoringRow[], props: Record<string, unknown> = {}) {
  return mount(MonitoringRowsTable, {
    props: {
      rows,
      sort: { col: 'name', dir: 'asc' },
      probeStats: {},
      probeHistory: {},
      checkingProbeId: null,
      checkingCertId: null,
      isAdmin: true,
      page: 1,
      totalPages: 1,
      pageSize: 25,
      filteredCount: rows.length,
      probeBadge: () => 'bg-green-lt',
      probeStatusLabel: () => 'UP',
      uptimeBadgeClass: () => 'bg-green-lt',
      daysLabel: (d: number | null | undefined) => `${d}j`,
      daysBadge: () => 'bg-blue-lt',
      ...props,
    },
    global: { stubs: { 'router-link': RouterLinkStub, RouterLink: RouterLinkStub } },
  })
}

const probeRow = (): MonitoringRow => ({ id: 'r1', name: 'API', probe: mkProbe() })
const certRow = (): MonitoringRow => ({ id: 'r2', name: 'Cert', cert: mkCert() })
const mergedRow = (): MonitoringRow => ({
  id: 'r3', name: 'Merged', npmProxyHostId: 'npm-9', probe: mkProbe({ id: 'p9' }), cert: mkCert({ id: 'c9' }),
})

describe('MonitoringRowsTable', () => {
  beforeEach(() => setLocale('fr'))

  it('renders no body rows for an empty list', () => {
    const w = mountTable([])
    expect(w.findAll('tbody tr')).toHaveLength(0)
  })

  it('links a row to the host detail when it has an NPM host, else to its probe, else to its cert', () => {
    const w = mountTable([mergedRow(), probeRow(), certRow()])
    const hrefs = w.findAll('a.rl').map(a => a.attributes('href'))
    expect(hrefs).toEqual(['/monitoring/host/npm-9', '/monitoring/probes/p1', '/monitoring/ssl/c1'])
    expect(w.findAll('tbody tr')[0].text()).toContain('NPM')
    expect(w.findAll('tbody tr')[1].text()).not.toContain('NPM')
  })

  it('shows the probe target for probe rows and host:port for cert-only rows', () => {
    const w = mountTable([probeRow(), certRow()])
    const rows = w.findAll('tbody tr')
    expect(rows[0].find('code').text()).toBe('https://api.example.com')
    expect(rows[1].find('code').text()).toBe('api.example.com:443')
  })

  it('shows status, uptime and ssl badges, or dashes when the data is missing', () => {
    const withData = mountTable([mergedRow()], { probeStats: { p9: { uptime_percent: 99.04 } } })
    const text = withData.find('tbody tr').text()
    expect(text).toContain('UP')
    expect(text).toContain('99.0%')
    expect(text).toContain('30j')

    const certOnly = mountTable([certRow()])
    const cells = certOnly.findAll('tbody tr td')
    expect(cells[1].text()).toBe('—') // status
    expect(cells[2].text()).toBe('—') // availability
    expect(cells[3].text()).toBe('—') // uptime
    const probeOnly = mountTable([probeRow()])
    expect(probeOnly.findAll('tbody tr td')[4].text()).toBe('—') // ssl
  })

  it('renders one tick per history entry, colored by success', () => {
    const w = mountTable([probeRow()], {
      probeHistory: { p1: [
        { id: 1, checked_at: '2026-01-01T10:00:00Z', success: true },
        { id: 2, checked_at: '2026-01-01T10:01:00Z', success: false },
      ] },
    })
    const ticks = w.findAll('.tracking > div')
    expect(ticks).toHaveLength(2)
    expect(ticks[0].classes()).toContain('bg-success')
    expect(ticks[1].classes()).toContain('bg-danger')
  })

  it('dims a disabled probe and labels it', () => {
    const w = mountTable([{ id: 'r', name: 'Off', probe: mkProbe({ enabled: false }) }])
    expect(w.find('tbody tr').classes()).toContain('opacity-60')
    expect(w.find('tbody tr').text()).toContain('désactivée')
    const enabled = mountTable([probeRow()])
    expect(enabled.find('tbody tr').classes()).not.toContain('opacity-60')
  })

  it('shows "Jamais" when never checked and a relative time otherwise', () => {
    const never = mountTable([probeRow()])
    expect(never.findAll('tbody tr td')[5].text()).toBe('Jamais')
    const checked = mountTable([{ id: 'r', name: 'X', cert: mkCert({ last_checked_at: new Date().toISOString() }) }])
    expect(checked.findAll('tbody tr td')[5].text()).not.toBe('Jamais')
  })

  it('emits toggleSort with the matching column from each sortable header', async () => {
    const w = mountTable([probeRow()])
    const buttons = w.findAll('thead button')
    for (const b of buttons) await b.trigger('click')
    expect(w.emitted('toggleSort')?.map(e => e[0])).toEqual(['name', 'status', 'uptime', 'ssl_days', 'last_checked'])
  })

  it('marks only the active sort column with a direction arrow', () => {
    const w = mountTable([probeRow()], { sort: { col: 'uptime', dir: 'desc' } })
    const headers = w.findAll('thead button').map(b => b.text())
    expect(headers.filter(h => h.includes('↓'))).toHaveLength(1)
    expect(headers[2]).toContain('↓')
  })

  it('emits per-action events with the row payloads', async () => {
    const row = mergedRow()
    const w = mountTable([row])
    await w.findAll('tbody button').find(b => b.text() === 'Vérifier')!.trigger('click')
    await w.find('[aria-label="Modifier la sonde"]').trigger('click')
    await w.find('[aria-label="Supprimer la sonde"]').trigger('click')
    await w.find('[aria-label="Modifier le certificat"]').trigger('click')
    await w.find('[aria-label="Supprimer le certificat"]').trigger('click')
    expect(w.emitted('check')![0][0]).toEqual(row)
    expect(w.emitted('editProbe')![0][0]).toEqual(row.probe)
    expect(w.emitted('deleteProbe')![0][0]).toEqual(row.probe)
    expect(w.emitted('editCert')![0][0]).toEqual(row.cert)
    expect(w.emitted('deleteCert')![0][0]).toEqual(row.cert)
  })

  it('only offers the probe actions on a probe-only row and the cert actions on a cert-only row', () => {
    const p = mountTable([probeRow()])
    expect(p.find('[aria-label="Modifier la sonde"]').exists()).toBe(true)
    expect(p.find('[aria-label="Modifier le certificat"]').exists()).toBe(false)
    const c = mountTable([certRow()])
    expect(c.find('[aria-label="Modifier la sonde"]').exists()).toBe(false)
    expect(c.find('[aria-label="Modifier le certificat"]').exists()).toBe(true)
  })

  it('hides every action for a non-admin', () => {
    const w = mountTable([mergedRow()], { isAdmin: false })
    expect(w.findAll('tbody button')).toHaveLength(0)
  })

  it('disables the verify button only for the row currently being checked', () => {
    const probeBusy = mountTable([probeRow()], { checkingProbeId: 'p1' })
    expect(probeBusy.find('tbody button').attributes('disabled')).toBeDefined()
    const certBusy = mountTable([certRow()], { checkingCertId: 'c1' })
    expect(certBusy.find('tbody button').attributes('disabled')).toBeDefined()
    const other = mountTable([probeRow()], { checkingProbeId: 'other' })
    expect(other.find('tbody button').attributes('disabled')).toBeUndefined()
  })

  it('hides the footer for a single page and shows the range + emits selectPage otherwise', async () => {
    expect(mountTable([probeRow()]).find('.card-footer').exists()).toBe(false)

    const w = mountTable([probeRow()], { page: 2, totalPages: 3, pageSize: 25, filteredCount: 60 })
    expect(w.find('.card-footer').text()).toContain('26–50 sur 60')
    await w.findAll('.page-link').find(a => a.text() === '3')!.trigger('click')
    expect(w.emitted('selectPage')![0]).toEqual([3])

    const last = mountTable([probeRow()], { page: 3, totalPages: 3, pageSize: 25, filteredCount: 60 })
    expect(last.find('.card-footer').text()).toContain('51–60 sur 60')
  })
})
