import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
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
