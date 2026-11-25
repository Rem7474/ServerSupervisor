import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import type { SLAReport, SLAWindow } from '../../types/uptime'

const { getUptimeSLA } = vi.hoisted(() => ({ getUptimeSLA: vi.fn() }))
vi.mock('../../api', () => ({
  default: { getUptimeSLA },
  getApiErrorMessage: (_e: unknown, fallback: string) => fallback,
}))

import MonitoringSlaPanel from './MonitoringSlaPanel.vue'

const RouterLinkStub = { props: ['to'], template: '<a :href="to"><slot /></a>' }

function win(days: number, over: Partial<SLAWindow> = {}): SLAWindow {
  return { days, covered_days: days, total_checks: 100, up_checks: 100, uptime_percent: 100,
    budget_remaining_percent: 100, allowed_downtime_minutes: 43.2, status: 'met', ...over }
}

const report: SLAReport = {
  generated_at: '2026-11-25T12:00:00Z',
  window_days: [7, 30, 90],
  probes: [
    {
      probe_id: 'a', name: 'API', enabled: true, slo_target: 99.9, data_since: '2026-10-27T00:00:00Z',
      windows: [win(7), win(30, { uptime_percent: 99.85, budget_remaining_percent: 0, status: 'breached' }),
        win(90, { covered_days: 30, uptime_percent: 99.95, status: 'at_risk', budget_remaining_percent: 10 })],
    },
    {
      probe_id: 'b', name: 'Old', enabled: false, slo_target: 99,
      windows: [7, 30, 90].map((d) => win(d, { covered_days: 0, uptime_percent: undefined, budget_remaining_percent: undefined, status: 'no_data' })),
    },
  ],
}

async function mountPanel() {
  const wrapper = mount(MonitoringSlaPanel, { global: { stubs: { 'router-link': RouterLinkStub, RouterLink: RouterLinkStub } } })
  await flushPromises()
  return wrapper
}

describe('MonitoringSlaPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
  })

  it('shows each window, its coverage when partial, and the 30-day error budget', async () => {
    getUptimeSLA.mockResolvedValue({ data: report })
    const wrapper = await mountPanel()

    const headers = wrapper.findAll('th').map((h) => h.text())
    expect(headers).toEqual(['Service', 'Objectif', '7 j', '30 j', '90 j', "Budget d'erreur (30 j)", 'Données'])

    const api = wrapper.find('tr[data-probe="a"]')
    const cells = api.findAll('td')
    expect(cells[1]!.text()).toBe('99,9 %')
    expect(cells[2]!.text()).toBe('100 %')
    expect(cells[3]!.find('.badge').classes()).toContain('bg-danger-lt')
    // 90-day window only covers 30 days: said explicitly, not hidden.
    expect(cells[4]!.text()).toContain('30 j sur 90')
    expect(cells[2]!.text()).not.toContain('sur')
    expect(cells[5]!.text()).toContain('0 % restant')
    expect(cells[5]!.text()).toContain('Non respecté')
    expect(cells[5]!.find('.progress-bar').classes()).toContain('bg-danger')
    expect(cells[6]!.text()).toContain('depuis le 27 oct. 2026')
  })

  it('marks a disabled probe and a probe without data', async () => {
    getUptimeSLA.mockResolvedValue({ data: report })
    const wrapper = await mountPanel()
    const old = wrapper.find('tr[data-probe="b"]')
    expect(old.text()).toContain('Désactivée')
    expect(old.text()).toContain('aucune donnée')
    expect(old.findAll('.badge').length).toBe(1)
  })

  it('shows the empty and error states', async () => {
    getUptimeSLA.mockResolvedValue({ data: { ...report, probes: [] } })
    expect((await mountPanel()).text()).toContain("Aucune sonde à mesurer")

    getUptimeSLA.mockRejectedValue(new Error('boom'))
    expect((await mountPanel()).find('.alert-danger').text()).toContain('Impossible de charger le rapport SLA')
  })

  it('uses the at-risk colour for the budget bar', async () => {
    const atRisk = structuredClone(report)
    atRisk.probes[0]!.windows[1] = win(30, { budget_remaining_percent: 10, status: 'at_risk' })
    getUptimeSLA.mockResolvedValue({ data: atRisk })
    const wrapper = await mountPanel()
    expect(wrapper.find('tr[data-probe="a"] .progress-bar').classes()).toContain('bg-warning')
  })
})
