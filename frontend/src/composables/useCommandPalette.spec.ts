import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

const { getIPInventory, getAllContainers } = vi.hoisted(() => ({
  getIPInventory: vi.fn().mockResolvedValue({ data: { proxmox_guests: [], npm_hosts: [] } }),
  getAllContainers: vi.fn().mockResolvedValue({ data: { containers: [] } }),
}))

vi.mock('../api', () => ({
  default: { getIPInventory, getAllContainers },
}))

import { useAuthStore } from '../stores/auth'
import { useAlertRulesStore } from '../stores/alertRules'
import { useCommandPalette } from './useCommandPalette'
import type { PaletteResult } from './useCommandPalette'
import { setLocale } from '../i18n'

function mountPalette(role: 'admin' | 'viewer') {
  setActivePinia(createPinia())
  const auth = useAuthStore()
  auth.setAuth({ role, username: 'u' } as never, role)

  let api!: ReturnType<typeof useCommandPalette>
  mount({
    setup() {
      api = useCommandPalette()
      return () => null
    },
  })
  return api
}

beforeEach(() => {
  setLocale('fr')
})

describe('useCommandPalette — nav results', () => {
  it('translates nav labels through the active locale', () => {
    const { query, results } = mountPalette('admin')
    query.value = ''
    const dashboard = results.value.find((r: PaletteResult) => r.to === '/')
    expect(dashboard?.label).toBe('Dashboard')
    expect(dashboard?.sublabel).toBe('Centre de contrôle')

    setLocale('en')
    const dashboardEn = results.value.find((r: PaletteResult) => r.to === '/')
    expect(dashboardEn?.sublabel).toBe('Control center')
  })

  it('excludes admin-only nav sections for a non-admin user', () => {
    const { query, results } = mountPalette('viewer')
    query.value = ''
    expect(results.value.some((r: PaletteResult) => r.to === '/settings')).toBe(false)
  })

  it('filters results by the current query, matching either the item or its section', () => {
    const { query, results } = mountPalette('admin')
    query.value = 'docker'
    expect(results.value.some((r: PaletteResult) => r.to === '/docker')).toBe(true)
    expect(results.value.some((r: PaletteResult) => r.to === '/')).toBe(false)
  })

  it('uses a stable, non-French group key for nav results', () => {
    const { query, results } = mountPalette('admin')
    query.value = ''
    const dashboard = results.value.find((r: PaletteResult) => r.to === '/')
    expect(dashboard?.group).toBe('navigation')
  })
})

describe('useCommandPalette — guest results', () => {
  // proxmoxGuestIPs/ipInventoryLoaded are module-level singletons in
  // useCommandPalette.ts (shared across every useCommandPalette() call —
  // see the file's own comment on why), so a second test in this file
  // calling open() would hit ensureIPInventoryLoaded's already-loaded guard
  // and never see a re-mocked getIPInventory response. Every guest/NPM
  // fixture this describe block needs is therefore seeded from the one
  // fetch below and asserted in a single test.
  it('finds a Proxmox guest by name/VMID/IP/associated domain, deep-links to its page, and enriches the sublabel', async () => {
    getIPInventory.mockResolvedValue({
      data: {
        proxmox_guests: [
          { guest_id: 'g1', name: 'web-01', node: 'pve1', guest_type: 'lxc', vmid: 101, status: 'running', ip_addresses: ['10.0.0.5'] },
          { guest_id: 'g2', name: 'db-01', node: 'pve2', guest_type: 'vm', vmid: 202, status: 'stopped', ip_addresses: [], host_id: 'h1', host_name: 'srv-db' },
        ],
        npm_hosts: [
          { proxy_host_id: 1, domain_names: ['app.example.com'], forward_host: '10.0.0.5', forward_port: 443, matched_type: 'proxmox_guest', matched_id: 'g1' },
        ],
      },
    })
    const { query, results, open } = mountPalette('admin')
    open()
    await flushPromises()

    for (const q of ['web-01', '101', '10.0.0.5', 'app.example.com']) {
      query.value = q
      const guest = results.value.find((r: PaletteResult) => r.key === 'guest:g1')
      expect(guest, `query "${q}" should match the guest`).toBeDefined()
      expect(guest?.group).toBe('guests')
      expect(guest?.label).toBe('web-01')
      expect(guest?.sublabel).toBe('pve1 · En cours · app.example.com')
      expect(guest?.to).toBe('/proxmox/guests/g1')
    }

    query.value = 'db-01'
    const linkedGuest = results.value.find((r: PaletteResult) => r.key === 'guest:g2')
    expect(linkedGuest?.sublabel).toContain('→ srv-db')
  })
})

describe('useCommandPalette — alert results', () => {
  it('translates the disabled-rule sublabel through the active locale', () => {
    const { query, results } = mountPalette('admin')
    const alertRulesStore = useAlertRulesStore()
    alertRulesStore.rules = [
      { id: '1', name: 'CPU haut', metric: 'cpu_percent', enabled: false } as never,
    ]
    query.value = 'cpu'
    const rule = results.value.find((r: PaletteResult) => r.key === 'alert-rule:1')
    expect(rule?.group).toBe('alerts')
    expect(rule?.sublabel).toBe('Désactivée')

    setLocale('en')
    const ruleEn = results.value.find((r: PaletteResult) => r.key === 'alert-rule:1')
    expect(ruleEn?.sublabel).toBe('Disabled')
  })
})
