import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../i18n'
import { useConfirmDialog } from './useConfirmDialog'
import type { NPMProxyHostEnriched } from '../types/npm'

const { listAllProxyHosts, setNPMEnabled, updateProxyHost } = vi.hoisted(() => ({
  listAllProxyHosts: vi.fn(),
  setNPMEnabled: vi.fn(),
  updateProxyHost: vi.fn(),
}))

vi.mock('../api/npm', () => ({
  npmApi: { listAllProxyHosts, setNPMEnabled, updateProxyHost },
}))

vi.mock('../api/client', () => ({
  getApiErrorMessage: (e: unknown, fallback?: string) => (e instanceof Error && e.message ? e.message : fallback),
}))

import { useNPM } from './useNPM'

function mountHost() {
  let api: ReturnType<typeof useNPM> | undefined
  mount(defineComponent({
    setup() {
      api = useNPM()
      return () => h('div')
    },
  }))
  return api!
}

function host(overrides: Record<string, unknown> = {}): NPMProxyHostEnriched {
  return {
    id: 'h1', connection_name: 'main', domain_names: ['a.example.com'],
    forward_host: '10.0.0.1', forward_port: 8080, npm_enabled: true,
    uptime_monitoring_enabled: false, ssl_monitoring_enabled: false, ssl_enabled: true,
    ...overrides,
  } as NPMProxyHostEnriched
}

describe('useNPM', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
  })

  it('sorts hosts by every sort key and toggles direction', async () => {
    listAllProxyHosts.mockResolvedValue({
      data: {
        proxy_hosts: [
          host({ id: 'a', connection_name: 'zeta', domain_names: ['z.example.com'], forward_host: 'z', ssl_days_remaining: 5, uptime_status: 'up' }),
          host({ id: 'b', connection_name: 'alpha', domain_names: ['a.example.com'], forward_host: 'a', ssl_days_remaining: 1, uptime_status: 'down' }),
        ],
      },
    })
    const api = mountHost()
    await flushPromises()
    expect(api.sortedHosts.value.map((h) => h.id)).toEqual(['b', 'a']) // connection_name asc (default)

    api.toggleSort('uptime_status')
    expect(api.sortedHosts.value[0].uptime_status).toBe('down') // down ranks first

    api.toggleSort('ssl_days_remaining')
    expect(api.sortedHosts.value[0].id).toBe('b') // fewer days first

    api.toggleSort('connection_name')
    api.toggleSort('connection_name')
    expect(api.sortDir.value).toBe('desc')
  })

  it('flags a proxy host active in NPM with no uptime monitoring as needing attention', () => {
    const api = mountHost()
    expect(api.needsAttention(host({ npm_enabled: true, uptime_monitoring_enabled: false }))).toBe(true)
    expect(api.needsAttention(host({ npm_enabled: true, uptime_monitoring_enabled: true }))).toBe(false)
    expect(api.needsAttention(host({ npm_enabled: false, uptime_monitoring_enabled: false }))).toBe(false)
  })

  it('shows a translated error when loading proxy hosts fails', async () => {
    listAllProxyHosts.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    expect(api.loadError.value).toBe('Impossible de charger les proxy hosts.')
  })

  it('declining the disable-in-NPM confirmation rolls the checkbox back', async () => {
    listAllProxyHosts.mockResolvedValue({ data: { proxy_hosts: [host()] } })
    const api = mountHost()
    await flushPromises()
    const dialog = useConfirmDialog()
    const target = api.hosts.value[0]
    const p = api.toggleNPM(target, false)
    expect(dialog.title.value).toBe('Désactiver le proxy host')
    expect(dialog.message.value).toBe('Désactiver "a.example.com" dans NPM coupe immédiatement le routage réel vers ce service.')
    dialog.onCancel()
    await p
    expect(target.npm_enabled).toBe(true)
    expect(setNPMEnabled).not.toHaveBeenCalled()
  })

  it('confirms disabling in NPM, turning off monitoring flags too, then rolls back on API failure', async () => {
    listAllProxyHosts.mockResolvedValue({
      data: { proxy_hosts: [host({ uptime_monitoring_enabled: true, ssl_monitoring_enabled: true })] },
    })
    setNPMEnabled.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    const dialog = useConfirmDialog()
    const target = api.hosts.value[0]
    const p = api.toggleNPM(target, false)
    dialog.onConfirm()
    await p
    expect(api.actionError.value).toBe('Impossible de désactiver le proxy host dans NPM.')
    expect(target.npm_enabled).toBe(true)
    expect(target.uptime_monitoring_enabled).toBe(true)
  })

  it('enables a proxy host in NPM without a confirmation prompt', async () => {
    listAllProxyHosts.mockResolvedValue({ data: { proxy_hosts: [host({ npm_enabled: false })] } })
    setNPMEnabled.mockResolvedValue({ data: host({ npm_enabled: true }) })
    const api = mountHost()
    await flushPromises()
    await api.toggleNPM(api.hosts.value[0], true)
    expect(setNPMEnabled).toHaveBeenCalledWith('h1', true)
    expect(api.actionError.value).toBe('')
  })

  it('shows a translated error and rolls back when enabling in NPM fails', async () => {
    listAllProxyHosts.mockResolvedValue({ data: { proxy_hosts: [host({ npm_enabled: false })] } })
    setNPMEnabled.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    const target = api.hosts.value[0]
    await api.toggleNPM(target, true)
    expect(api.actionError.value).toBe("Impossible d'activer le proxy host dans NPM.")
    expect(target.npm_enabled).toBe(false)
  })

  it('toggles uptime monitoring and rolls back with a translated error on failure', async () => {
    listAllProxyHosts.mockResolvedValue({ data: { proxy_hosts: [host()] } })
    updateProxyHost.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    const target = api.hosts.value[0]
    await api.toggle(target, 'uptime_monitoring_enabled', true)
    expect(api.actionError.value).toBe('Erreur lors de la mise à jour du monitoring.')
    expect(target.uptime_monitoring_enabled).toBe(false)
  })

  it('toggles monitoring successfully and applies the server response', async () => {
    listAllProxyHosts.mockResolvedValue({ data: { proxy_hosts: [host()] } })
    updateProxyHost.mockResolvedValue({ data: host({ uptime_monitoring_enabled: true }) })
    const api = mountHost()
    await flushPromises()
    await api.toggle(api.hosts.value[0], 'uptime_monitoring_enabled', true)
    expect(api.hosts.value[0].uptime_monitoring_enabled).toBe(true)
  })

  it('translates error messages to English when the locale is switched', async () => {
    setLocale('en')
    listAllProxyHosts.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    expect(api.loadError.value).toBe('Unable to load the proxy hosts.')
  })
})
