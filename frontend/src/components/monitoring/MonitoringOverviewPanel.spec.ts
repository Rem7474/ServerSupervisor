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

vi.mock('../../api', () => ({
  default: {
    getUptimeProbes: vi.fn(async () => ({ data: { probes: [probeFixture] } })),
    getUptimeStats: vi.fn(async () => ({ data: { uptime_percent: 99.9 } })),
    getUptimeHistory: vi.fn(async () => ({ data: { results: [] } })),
    getSSLCertificates: vi.fn(async () => ({ data: { certificates: [certFixture] } })),
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
})
