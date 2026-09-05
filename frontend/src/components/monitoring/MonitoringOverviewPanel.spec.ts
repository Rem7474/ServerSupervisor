import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../../i18n'
import type { UptimeProbe, SSLCertificate } from '../../types/generated'
import { useAuthStore } from '../../stores/auth'

const RouterLinkStub = { props: ['to'], template: '<a :href="to"><slot /></a>' }

const probeFixture: UptimeProbe = {
  id: 'probe-1', name: 'API prod', type: 'http', target: 'https://api.example.com',
  interval_sec: 60, timeout_sec: 10, expected_status: 200, expected_body_regex: '',
  follow_redirects: true, verify_tls: true, enabled: true, last_status: 'up',
  consecutive_failures: 0, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
  npm_proxy_host_id: 'npm-1', npm_proxy_host_domain: 'api.example.com',
}

const certFixture: SSLCertificate = {
  id: 'cert-1', name: 'api.example.com', host: 'api.example.com', port: 443,
  enabled: true, days_remaining: 42, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
  npm_proxy_host_id: 'npm-1', npm_proxy_host_domain: 'api.example.com',
}

interface HistoryTick { id: string; checked_at: string; success: boolean }

const {
  getUptimeProbes, getUptimeStats, getUptimeHistory, getSSLCertificates,
  createUptimeProbe, updateUptimeProbe, createSSLCertificate, updateSSLCertificate,
} = vi.hoisted(() => ({
  getUptimeProbes: vi.fn(async (): Promise<{ data: { probes: UptimeProbe[] } }> => ({ data: { probes: [] } })),
  getUptimeStats: vi.fn(async () => ({ data: { uptime_percent: 99.9 } })),
  getUptimeHistory: vi.fn(async (): Promise<{ data: { results: HistoryTick[] } }> => ({ data: { results: [] } })),
  getSSLCertificates: vi.fn(async (): Promise<{ data: { certificates: SSLCertificate[] } }> => ({ data: { certificates: [] } })),
  createUptimeProbe: vi.fn(async () => ({ data: {} })),
  updateUptimeProbe: vi.fn(async () => ({ data: {} })),
  createSSLCertificate: vi.fn(async () => ({ data: {} })),
  updateSSLCertificate: vi.fn(async () => ({ data: {} })),
}))

vi.mock('../../api', () => ({
  default: {
    getUptimeProbes, getUptimeStats, getUptimeHistory, getSSLCertificates,
    createUptimeProbe, updateUptimeProbe, createSSLCertificate, updateSSLCertificate,
  },
}))

import MonitoringOverviewPanel from './MonitoringOverviewPanel.vue'

async function mountPanel() {
  setActivePinia(createPinia())
  const auth = useAuthStore()
  auth.setAuth({ role: 'admin', username: 'admin' } as never, 'admin')
  const wrapper = mount(MonitoringOverviewPanel, {
    global: { stubs: { 'router-link': RouterLinkStub, RouterLink: RouterLinkStub } },
  })
  await flushPromises()
  return wrapper
}

describe('MonitoringOverviewPanel (characterization)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    getUptimeProbes.mockResolvedValue({ data: { probes: [probeFixture] } })
    getUptimeStats.mockResolvedValue({ data: { uptime_percent: 99.9 } })
    getUptimeHistory.mockResolvedValue({ data: { results: [] } })
    getSSLCertificates.mockResolvedValue({ data: { certificates: [certFixture] } })
  })

  it('merges the probe and cert sharing an npm_proxy_host_id into one row', async () => {
    const wrapper = await mountPanel()
    expect(wrapper.text()).toContain('api.example.com')
  })

  it('editing the probe opens the probe form (incl. HTTP-only fields) with associated labels', async () => {
    const wrapper = await mountPanel()
    await wrapper.find('[aria-label="Modifier la sonde"]').trigger('click')

    for (const id of [
      'monitoring-edit-probe-name',
      'monitoring-edit-probe-type',
      'monitoring-edit-probe-target',
      'monitoring-edit-probe-interval',
      'monitoring-edit-probe-timeout',
      'monitoring-edit-probe-expected-status',
      'monitoring-edit-probe-expected-body-regex',
    ]) {
      expect(wrapper.find(`#${id}`).exists(), `#${id}`).toBe(true)
      expect(wrapper.find(`label[for="${id}"]`).exists(), `label for #${id}`).toBe(true)
    }
  })

  it('editing the cert opens the cert form with associated labels', async () => {
    const wrapper = await mountPanel()
    await wrapper.find('[aria-label="Modifier le certificat"]').trigger('click')

    for (const id of ['monitoring-edit-cert-name', 'monitoring-edit-cert-host', 'monitoring-edit-cert-port', 'monitoring-edit-cert-sni']) {
      expect(wrapper.find(`#${id}`).exists(), `#${id}`).toBe(true)
      expect(wrapper.find(`label[for="${id}"]`).exists(), `label for #${id}`).toBe(true)
    }
  })

  it('the shared "create" modal renders probe fields, and cert fields once toggled on', async () => {
    const wrapper = await mountPanel()
    // Only openCreateProbe is exposed (see the component's own defineExpose
    // comment) — it's the single real entry point (MonitoringView's button).
    await (wrapper.vm as unknown as { openCreateProbe: () => void }).openCreateProbe()
    await flushPromises()

    for (const id of [
      'monitoring-create-name',
      'monitoring-create-probe-type',
      'monitoring-create-probe-target',
      'monitoring-create-probe-interval',
      'monitoring-create-probe-timeout',
      'monitoring-create-probe-expected-status',
      'monitoring-create-probe-expected-body-regex',
    ]) {
      expect(wrapper.find(`#${id}`).exists(), `#${id}`).toBe(true)
    }
    // Cert fields not shown yet (probe-only by default).
    expect(wrapper.find('#monitoring-create-cert-host').exists()).toBe(false)

    // Toggle cert on, probe off, to reach the cert-only Host/Port fields
    // (with both toggled on, the cert target is derived from the probe's
    // instead, per the "Certificat SSL vérifié sur ..." branch).
    const buttons = wrapper.findAll('button')
    const certToggle = buttons.find((b) => b.text() === 'Certificat SSL')
    const probeToggle = buttons.find((b) => b.text() === 'Sonde uptime')
    expect(certToggle && probeToggle).toBeTruthy()
    await certToggle!.trigger('click')
    await probeToggle!.trigger('click')

    for (const id of ['monitoring-create-cert-host', 'monitoring-create-cert-port', 'monitoring-create-cert-sni']) {
      expect(wrapper.find(`#${id}`).exists(), `#${id}`).toBe(true)
    }
  })

  it('shows the translated "no probe/cert configured" empty state (with an admin CTA) when nothing is monitored', async () => {
    getUptimeProbes.mockResolvedValue({ data: { probes: [] } })
    getSSLCertificates.mockResolvedValue({ data: { certificates: [] } })
    const wrapper = await mountPanel()

    expect(wrapper.text()).toContain('Aucune sonde ni certificat configuré')
    expect(wrapper.text()).toContain('Créez une sonde uptime ou un certificat SSL pour commencer à surveiller un service.')
    expect(wrapper.text()).toContain('Nouveau suivi')
  })

  it('shows the translated "no results" empty state (no CTA) when a search matches nothing', async () => {
    const wrapper = await mountPanel()
    const search = wrapper.find('input[placeholder="Rechercher un hôte…"]')
    await search.setValue('no-such-host')
    await flushPromises()

    expect(wrapper.text()).toContain('Aucun résultat pour cette recherche')
    expect(wrapper.text()).toContain('Modifiez votre recherche.')
    expect(wrapper.text()).not.toContain('Nouveau suivi')
  })

  it('shows a translated OK/KO tooltip per heartbeat tick, reflecting each tick\'s own outcome', async () => {
    getUptimeHistory.mockResolvedValue({
      data: { results: [{ id: 't1', checked_at: '2026-01-01T00:00:00Z', success: true }, { id: 't2', checked_at: '2026-01-01T00:01:00Z', success: false }] },
    })
    const wrapper = await mountPanel()

    const ticks = wrapper.findAll('.tracking .flex-fill')
    expect(ticks.length).toBeGreaterThanOrEqual(2)
    expect(ticks.some((el) => el.attributes('title')?.includes('— OK'))).toBe(true)
    expect(ticks.some((el) => el.attributes('title')?.includes('— KO'))).toBe(true)
  })

  it('shows the translated "saving" label while a probe save is in flight, then reverts', async () => {
    let resolveCreate!: (v: { data: object }) => void
    createUptimeProbe.mockReturnValue(new Promise((resolve) => { resolveCreate = resolve }))
    const wrapper = await mountPanel()
    await (wrapper.vm as unknown as { openCreateProbe: () => void }).openCreateProbe()
    await flushPromises()

    await wrapper.find('#monitoring-create-name').setValue('New probe')
    await wrapper.find('#monitoring-create-probe-target').setValue('https://example.com')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    const submitButton = wrapper.findAll('button[type="submit"]').find((b) => b.text().includes('Enregistrement') || b.text().includes('Enregistrer'))
    expect(submitButton!.text()).toBe('Enregistrement…')

    resolveCreate({ data: {} })
    await flushPromises()
    await flushPromises()
  })

  it('derives the target label/placeholder from the selected probe type (URL / host / host:port)', async () => {
    const wrapper = await mountPanel()
    await (wrapper.vm as unknown as { openCreateProbe: () => void }).openCreateProbe()
    await flushPromises()

    const typeSelect = wrapper.find('#monitoring-create-probe-type')
    const targetLabel = () => wrapper.find('label[for="monitoring-create-probe-target"]').text()
    const targetPlaceholder = () => wrapper.find('#monitoring-create-probe-target').attributes('placeholder')

    expect(targetLabel()).toBe('URL')
    expect(targetPlaceholder()).toBe('https://example.com/health')

    await typeSelect.setValue('icmp')
    expect(targetLabel()).toBe('Hôte ou IP')
    expect(targetPlaceholder()).toBe('192.168.1.1 ou switch.local')

    await typeSelect.setValue('tcp')
    expect(targetLabel()).toBe('host:port')
    expect(targetPlaceholder()).toBe('example.com:443')
  })
})
