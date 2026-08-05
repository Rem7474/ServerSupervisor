import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const { wsMessageHandler, streamOptionsByCommand } = vi.hoisted(() => ({
  wsMessageHandler: { current: null as ((payload: unknown) => void) | null },
  streamOptionsByCommand: new Map<string, { onStatus?: (p: { status: string; output?: string }) => void }>(),
}))

vi.mock('./useWebSocket', () => ({
  useWebSocket: (_url: string, onMessage: (payload: unknown) => void) => {
    wsMessageHandler.current = onMessage
    return {
      wsStatus: { value: 'connected' },
      wsError: { value: '' },
      retryCount: { value: 0 },
      dataStaleAlert: { value: false },
      reconnect: vi.fn(),
    }
  },
}))

vi.mock('./useCommandStream', () => ({
  useCommandStream: () => ({
    openCommandStream: (commandId: string, options: { onStatus?: (p: { status: string; output?: string }) => void }) => {
      streamOptionsByCommand.set(commandId, options)
    },
    closeStream: vi.fn(),
  }),
}))

const { updateHostAgent } = vi.hoisted(() => ({ updateHostAgent: vi.fn() }))

vi.mock('../api', () => ({
  default: { sendAptCommand: vi.fn(), updateHostAgent },
  getApiErrorMessage: (e: unknown) => String(e),
}))

import { useApt } from './useApt'
import { useConfirmDialog } from './useConfirmDialog'
import type { Host } from '../types/host'

const HOST: Host = { id: 'h1', name: 'web-01' } as Host
const HOST2: Host = { id: 'h2', name: 'db-01' } as Host

function mountUseApt() {
  let api!: ReturnType<typeof useApt>
  const wrapper = mount({
    setup() {
      api = useApt()
      return () => null
    },
  })
  return { wrapper, api: api! }
}

function emitAptSnapshot(hostId: string, cveUpdatedAt: string | undefined, updatedAt?: string) {
  wsMessageHandler.current?.({
    type: 'apt',
    hosts: [HOST],
    apt_statuses: (cveUpdatedAt || updatedAt)
      ? { [hostId]: { pending_packages: 3, updated_at: updatedAt, cve_updated_at: cveUpdatedAt } }
      : {},
    apt_histories: {},
  })
}

interface FullSnapshotOverrides {
  hosts?: Host[]
  aptStatuses?: Record<string, unknown>
  uuStatuses?: Record<string, unknown>
  latestAgentVersion?: string
}

function emitFullAptSnapshot(overrides: FullSnapshotOverrides = {}) {
  wsMessageHandler.current?.({
    type: 'apt',
    hosts: overrides.hosts || [HOST, HOST2],
    apt_statuses: overrides.aptStatuses || {},
    apt_histories: {},
    uu_statuses: overrides.uuStatuses || {},
    latest_agent_version: overrides.latestAgentVersion || '',
  })
}

beforeEach(() => {
  setActivePinia(createPinia())
  streamOptionsByCommand.clear()
  wsMessageHandler.current = null
  updateHostAgent.mockClear()
  updateHostAgent.mockResolvedValue({ data: {} })
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
})

describe('useApt — post-command CVE enrichment indicator', () => {
  it('marks the host as enriching once an update/upgrade/dist-upgrade command completes, and clears it when a fresher cve_updated_at arrives', () => {
    const { api } = mountUseApt()
    api.liveCommand.value = {
      id: 'cmd-1', hostId: 'h1', host_name: 'web-01', module: 'apt', action: 'update', target: '', status: 'running', output: '',
    }

    api.watchCommand({ id: 'cmd-1', action: 'update', status: 'running', output: '' }, HOST)

    expect(api.enrichingHosts.value.h1).toBeUndefined()

    streamOptionsByCommand.get('cmd-1')?.onStatus?.({ status: 'completed', output: 'Reading package lists... Done' })
    expect(api.enrichingHosts.value.h1).toBe(true)

    // A WS snapshot with the same (stale) cve_updated_at must not clear it.
    emitAptSnapshot('h1', undefined)
    expect(api.enrichingHosts.value.h1).toBe(true)

    // A fresh snapshot (new cve_updated_at) clears it.
    emitAptSnapshot('h1', new Date().toISOString())
    expect(api.enrichingHosts.value.h1).toBeUndefined()
  })

  it('does NOT clear the flag when only updated_at bumps (the fast, CVE-free pending-count refresh) — only cve_updated_at counts', () => {
    const { api } = mountUseApt()
    api.watchCommand({ id: 'cmd-4', action: 'update', status: 'running', output: '' }, HOST)
    streamOptionsByCommand.get('cmd-4')?.onStatus?.({ status: 'completed' })
    expect(api.enrichingHosts.value.h1).toBe(true)

    // Fast pending-packages-only path lands: updated_at bumps, cve_updated_at doesn't.
    emitAptSnapshot('h1', undefined, new Date().toISOString())
    expect(api.enrichingHosts.value.h1).toBe(true)

    // The slow CVE-enriched pass finally lands.
    emitAptSnapshot('h1', new Date().toISOString())
    expect(api.enrichingHosts.value.h1).toBeUndefined()
  })

  it('clears the enriching flag via the safety timeout if no fresh data ever arrives', () => {
    const { api } = mountUseApt()
    api.watchCommand({ id: 'cmd-2', action: 'upgrade', status: 'running', output: '' }, HOST)
    streamOptionsByCommand.get('cmd-2')?.onStatus?.({ status: 'completed' })
    expect(api.enrichingHosts.value.h1).toBe(true)

    vi.advanceTimersByTime(5.5 * 60_000)
    expect(api.enrichingHosts.value.h1).toBeUndefined()
  })

  it('does not set the enriching flag for non-mutating apt actions', () => {
    const { api } = mountUseApt()
    api.watchCommand({ id: 'cmd-3', action: 'install_uu', status: 'running', output: '' }, HOST)
    streamOptionsByCommand.get('cmd-3')?.onStatus?.({ status: 'completed' })
    expect(api.enrichingHosts.value.h1).toBeUndefined()
  })
})

describe('useApt — extended search + reboot/outdated-agent filters', () => {
  it('search also matches a pending package name across the fleet', () => {
    const { api } = mountUseApt()
    emitFullAptSnapshot({
      aptStatuses: {
        h1: { package_list: JSON.stringify(['nginx', 'curl']) },
        h2: { package_list: JSON.stringify(['postgresql']) },
      },
    })
    api.hostSearch.value = 'curl'
    expect(api.filteredHosts.value.map((h) => h.id)).toEqual(['h1'])
  })

  it('search also matches a CVE id, case-insensitively', () => {
    const { api } = mountUseApt()
    emitFullAptSnapshot({
      aptStatuses: {
        h1: { cve_list: [{ id: 'CVE-2024-1234', severity: 'HIGH' }] },
        h2: { cve_list: [] },
      },
    })
    api.hostSearch.value = 'cve-2024-1234'
    expect(api.filteredHosts.value.map((h) => h.id)).toEqual(['h1'])
  })

  it('the "reboot" quick filter shows only hosts with reboot_required', () => {
    const { api } = mountUseApt()
    emitFullAptSnapshot({
      uuStatuses: {
        h1: { installed: true, enabled: true, reboot_required: true },
        h2: { installed: true, enabled: true, reboot_required: false },
      },
    })
    api.hostQuickFilter.value = 'reboot'
    expect(api.filteredHosts.value.map((h) => h.id)).toEqual(['h1'])
  })

  it('the "outdated_agent" quick filter shows only hosts whose agent_version differs from latest', () => {
    const { api } = mountUseApt()
    emitFullAptSnapshot({
      hosts: [
        { ...HOST, agent_version: '1.0.0' } as Host,
        { ...HOST2, agent_version: '2.0.0' } as Host,
      ],
      latestAgentVersion: '2.0.0',
    })
    api.hostQuickFilter.value = 'outdated_agent'
    expect(api.filteredHosts.value.map((h) => h.id)).toEqual(['h1'])
  })

  it('does not flag any host as outdated before latestAgentVersion has arrived', () => {
    const { api } = mountUseApt()
    emitFullAptSnapshot({ hosts: [{ ...HOST, agent_version: '1.0.0' } as Host], latestAgentVersion: '' })
    expect(api.isAgentOutdated(api.hosts.value[0])).toBe(false)
  })
})

describe('useApt — bulk agent update', () => {
  it('updates only the selected AND outdated hosts, after confirmation', async () => {
    const { api } = mountUseApt()
    emitFullAptSnapshot({
      hosts: [
        { ...HOST, agent_version: '1.0.0' } as Host,
        { ...HOST2, agent_version: '2.0.0' } as Host,
      ],
      latestAgentVersion: '2.0.0',
    })
    api.selectedHosts.value = ['h1', 'h2']
    expect(api.outdatedSelectedHosts.value.map((h) => h.id)).toEqual(['h1'])

    const dialog = useConfirmDialog()
    const updatePromise = api.bulkAgentUpdate()
    dialog.onConfirm()
    await updatePromise

    expect(updateHostAgent).toHaveBeenCalledTimes(1)
    expect(updateHostAgent).toHaveBeenCalledWith('h1')
  })

  it('does nothing when no selected host is outdated', async () => {
    const { api } = mountUseApt()
    emitFullAptSnapshot({
      hosts: [{ ...HOST, agent_version: '2.0.0' } as Host],
      latestAgentVersion: '2.0.0',
    })
    api.selectedHosts.value = ['h1']

    await api.bulkAgentUpdate()

    expect(updateHostAgent).not.toHaveBeenCalled()
  })

  it('does not update anything when the confirmation is cancelled', async () => {
    const { api } = mountUseApt()
    emitFullAptSnapshot({
      hosts: [{ ...HOST, agent_version: '1.0.0' } as Host],
      latestAgentVersion: '2.0.0',
    })
    api.selectedHosts.value = ['h1']

    const dialog = useConfirmDialog()
    const updatePromise = api.bulkAgentUpdate()
    dialog.onCancel()
    await updatePromise

    expect(updateHostAgent).not.toHaveBeenCalled()
  })
})
