import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { AxiosError, AxiosHeaders } from 'axios'
import { setLocale } from '../i18n'
import type { PublicStatusPage } from '../types/uptime'

const { getPublicStatus } = vi.hoisted(() => ({ getPublicStatus: vi.fn() }))
vi.mock('../api', () => ({ default: { getPublicStatus } }))

import StatusPageView from './StatusPageView.vue'

function days(pcts: Array<number | null>) {
  return pcts.map((p, i) => ({ date: `2026-11-${String(i + 1).padStart(2, '0')}`, uptime_percent: p ?? undefined }))
}

const page: PublicStatusPage = {
  title: 'Acme',
  description: 'Our public services',
  status: 'partial_outage',
  generated_at: '2026-11-25T12:00:00Z',
  services: [
    { name: 'Website', status: 'operational', uptime_percent_90d: 99.95, days: days([100, 99.5, null]) },
    { name: 'API', status: 'major_outage', days: days([null, null, 80]) },
  ],
  incidents: [
    { service: 'API', started_at: '2026-11-25T11:50:00Z' },
    { service: 'Website', started_at: '2026-11-24T08:00:00Z', resolved_at: '2026-11-24T09:15:00Z' },
  ],
}

function notFound(): AxiosError {
  return new AxiosError('nf', 'ERR_BAD_REQUEST', undefined, undefined, {
    status: 404, statusText: 'Not Found', data: {}, headers: {}, config: { headers: new AxiosHeaders() },
  })
}

const stubs = { LocaleSwitcher: true }

describe('StatusPageView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
  })
  afterEach(() => vi.useRealTimers())

  it('renders the overall status, each published service and its 90-day bar', async () => {
    getPublicStatus.mockResolvedValue({ data: page })
    const wrapper = mount(StatusPageView, { global: { stubs } })
    await flushPromises()

    expect(wrapper.find('h1').text()).toBe('Acme')
    expect(wrapper.text()).toContain('Our public services')
    expect(wrapper.find('[data-test="overall"]').text()).toContain('Certains services sont perturbés')

    const services = wrapper.findAll('[data-test="service"]')
    expect(services).toHaveLength(2)
    expect(services[0]!.text()).toContain('Website')
    expect(services[0]!.text()).toContain('Opérationnel')
    expect(services[0]!.text()).toContain('99,95 % sur 90 jours')
    expect(services[1]!.text()).toContain('En panne')
    expect(services[1]!.text()).toContain('Pas de données sur 90 jours')

    const bar = services[0]!.findAll('.uptime-day')
    expect(bar.map((d) => d.classes().find((c) => c.startsWith('uptime-day-')))).toEqual(['uptime-day-ok', 'uptime-day-warn', 'uptime-day-none'])
    expect(bar[2]!.attributes('title')).toContain('aucune donnée')
    expect(services[0]!.find('.uptime-bar').attributes('aria-label')).toContain('Website')
  })

  it('lists ongoing and resolved incidents generically, with a duration', async () => {
    getPublicStatus.mockResolvedValue({ data: page })
    const wrapper = mount(StatusPageView, { global: { stubs } })
    await flushPromises()
    const incidents = wrapper.findAll('[data-test="incident"]')
    expect(incidents).toHaveLength(2)
    expect(incidents[0]!.text()).toContain('Indisponibilité : API')
    expect(incidents[0]!.text()).toContain('En cours depuis')
    expect(incidents[1]!.text()).toContain('1 h 15 min')
  })

  it('shows the empty states', async () => {
    getPublicStatus.mockResolvedValue({ data: { ...page, title: '', status: 'unknown', services: [], incidents: [] } })
    const wrapper = mount(StatusPageView, { global: { stubs } })
    await flushPromises()
    expect(wrapper.find('h1').text()).toBe('État des services')
    expect(wrapper.text()).toContain("Aucun service n'est publié")
    expect(wrapper.text()).toContain('Aucun incident')
    expect(wrapper.find('[data-test="overall"]').text()).toContain('inconnu')
  })

  it('shows an unavailable page on 404 (page disabled)', async () => {
    getPublicStatus.mockRejectedValue(notFound())
    const wrapper = mount(StatusPageView, { global: { stubs } })
    await flushPromises()
    expect(wrapper.text()).toContain('Page de statut indisponible')
    expect(wrapper.find('[data-test="service"]').exists()).toBe(false)
  })

  it('keeps the last snapshot under a warning when a refresh fails, and refreshes every minute', async () => {
    vi.useFakeTimers()
    getPublicStatus.mockResolvedValueOnce({ data: page }).mockRejectedValueOnce(new Error('network'))
    const wrapper = mount(StatusPageView, { global: { stubs } })
    await flushPromises()
    expect(getPublicStatus).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(60_000)
    await flushPromises()
    expect(getPublicStatus).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(wrapper.findAll('[data-test="service"]')).toHaveLength(2)

    wrapper.unmount()
    await vi.advanceTimersByTimeAsync(120_000)
    expect(getPublicStatus).toHaveBeenCalledTimes(2)
  })

  it('formats short and sub-minute durations', async () => {
    getPublicStatus.mockResolvedValue({ data: { ...page, incidents: [
      { service: 'A', started_at: '2026-11-24T08:00:00Z', resolved_at: '2026-11-24T08:00:20Z' },
      { service: 'B', started_at: '2026-11-24T08:00:00Z', resolved_at: '2026-11-24T08:12:00Z' },
    ] } })
    const wrapper = mount(StatusPageView, { global: { stubs } })
    await flushPromises()
    const text = wrapper.findAll('[data-test="incident"]').map((i) => i.text())
    expect(text[0]).toContain("moins d'une minute")
    expect(text[1]).toContain('12 min')
  })
})
