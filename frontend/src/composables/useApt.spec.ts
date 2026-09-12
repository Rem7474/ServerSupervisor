import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const { wsMessageHandler, streamOptionsByCommand, openCommandStream } = vi.hoisted(() => ({
  wsMessageHandler: { current: null as ((payload: unknown) => void) | null },
  streamOptionsByCommand: new Map<string, { onStatus?: (p: { status: string; output?: string }) => void }>(),
  openCommandStream: vi.fn(),
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
      openCommandStream(commandId, options)
    },
    closeStream: vi.fn(),
  }),
}))

const { updateHostAgent, sendAptCommand } = vi.hoisted(() => ({ updateHostAgent: vi.fn(), sendAptCommand: vi.fn() }))

vi.mock('../api', () => ({
  default: { sendAptCommand, updateHostAgent },
  getApiErrorMessage: (e: unknown) => String(e),
}))

import { useApt } from './useApt'
import { useConfirmDialog } from './useConfirmDialog'
import { useGlobalToast } from './useGlobalToast'
import { useAuthStore } from '../stores/auth'
import { setLocale } from '../i18n'
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
  setLocale('fr')
  streamOptionsByCommand.clear()
  wsMessageHandler.current = null
  updateHostAgent.mockClear()
  updateHostAgent.mockResolvedValue({ data: {} })
  sendAptCommand.mockClear()
  openCommandStream.mockClear()
  useGlobalToast().toasts.splice(0)
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

describe('useApt — host filter options', () => {
  it('exposes the quick-filter options with their French labels', () => {
    const { api } = mountUseApt()
    expect(api.hostFilterOptions.value.map((f) => f.label)).toEqual([
      'Tous', 'CVE critiques', 'Sécu > 0', 'Redémarrage requis', 'Agent obsolète',
    ])
  })
})

describe('useApt — runAptCmdForHost', () => {
  beforeEach(() => {
    useAuthStore().role = 'admin'
  })

  it('is a no-op for a caller without run permission', async () => {
    useAuthStore().role = 'viewer'
    sendAptCommand.mockResolvedValue({ data: { commands: [] } })
    const { api } = mountUseApt()

    await api.runAptCmdForHost(HOST, 'update')

    expect(sendAptCommand).not.toHaveBeenCalled()
  })

  it('dispatches "update" immediately without a confirmation dialog', async () => {
    sendAptCommand.mockResolvedValue({ data: { commands: [{ command_id: 'cmd1', host_id: 'h1', status: 'pending' }] } })
    const { api } = mountUseApt()
    const dialog = useConfirmDialog()

    await api.runAptCmdForHost(HOST, 'update')

    expect(dialog.isOpen.value).toBe(false)
    expect(sendAptCommand).toHaveBeenCalledWith(['h1'], 'update')
  })

  it('asks for confirmation before "upgrade", and skips the dispatch when cancelled', async () => {
    const { api } = mountUseApt()
    const dialog = useConfirmDialog()

    const promise = api.runAptCmdForHost(HOST, 'upgrade')
    expect(dialog.title.value).toBe('apt upgrade')
    dialog.onCancel()
    await promise

    expect(sendAptCommand).not.toHaveBeenCalled()
  })

  it('shows the mapped error from a per-command failure when nothing launched', async () => {
    sendAptCommand.mockResolvedValue({ data: { commands: [{ host_id: 'h1', error: 'agent hors ligne' }] } })
    const { api } = mountUseApt()
    const dialog = useConfirmDialog()

    const promise = api.runAptCmdForHost(HOST, 'update')
    await vi.advanceTimersByTimeAsync(0)
    expect(dialog.isOpen.value).toBe(true)
    expect(dialog.title.value).toBe('Erreur')
    expect(dialog.message.value).toBe('agent hors ligne')
    dialog.onConfirm()
    await promise
  })

  it('shows a translated error when the dispatch request itself throws', async () => {
    sendAptCommand.mockRejectedValue(new Error('network down'))
    const { api } = mountUseApt()
    const dialog = useConfirmDialog()

    const promise = api.runAptCmdForHost(HOST, 'update')
    await vi.advanceTimersByTimeAsync(0)
    expect(dialog.isOpen.value).toBe(true)
    expect(dialog.title.value).toBe('Erreur')
    expect(dialog.message.value).toContain('network down')
    dialog.onConfirm()
    await promise
  })
})

describe('useApt — bulkAptCmd', () => {
  it('shows the single-host success message', async () => {
    sendAptCommand.mockResolvedValue({ data: { commands: [{ command_id: 'cmd1', host_id: 'h1', status: 'pending' }] } })
    const { api } = mountUseApt()
    emitFullAptSnapshot({ hosts: [HOST] })
    api.selectedHosts.value = ['h1']

    const dialog = useConfirmDialog()
    const promise = api.bulkAptCmd('upgrade')
    dialog.onConfirm()
    await promise

    expect(sendAptCommand).toHaveBeenCalledWith(['h1'], 'upgrade')
    // A single selected host takes a single-host phrasing branch (never
    // reaches the toast at all, since selectedHosts.length === 1 and there
    // are no failures) — no toast is expected here.
    expect(useGlobalToast().toasts.length).toBe(0)
  })

  it('shows the multi-host success message with the host count', async () => {
    sendAptCommand.mockResolvedValue({
      data: {
        commands: [
          { command_id: 'cmd1', host_id: 'h1', status: 'pending' },
          { command_id: 'cmd2', host_id: 'h2', status: 'pending' },
        ],
      },
    })
    const { api } = mountUseApt()
    emitFullAptSnapshot({ hosts: [HOST, HOST2] })
    api.selectedHosts.value = ['h1', 'h2']

    const dialog = useConfirmDialog()
    const promise = api.bulkAptCmd('update')
    dialog.onConfirm()
    await promise

    expect(sendAptCommand).toHaveBeenCalledWith(['h1', 'h2'], 'update')
    const toasts = useGlobalToast().toasts
    expect(toasts[toasts.length - 1]?.message).toBe('apt update lancée sur 2 hôtes')
    expect(toasts[toasts.length - 1]?.type).toBe('success')
  })

  it('reports a mixed launched/failed summary', async () => {
    sendAptCommand.mockResolvedValue({
      data: {
        commands: [
          { command_id: 'cmd1', host_id: 'h1', status: 'pending' },
          { host_id: 'h2', error: 'agent hors ligne' },
        ],
      },
    })
    const { api } = mountUseApt()
    emitFullAptSnapshot({ hosts: [HOST, HOST2] })
    api.selectedHosts.value = ['h1', 'h2']

    const dialog = useConfirmDialog()
    const promise = api.bulkAptCmd('update')
    dialog.onConfirm()
    await promise

    expect(sendAptCommand).toHaveBeenCalledWith(['h1', 'h2'], 'update')
    const toasts = useGlobalToast().toasts
    expect(toasts[toasts.length - 1]?.message).toBe('apt update lancée sur web-01 — échec sur : db-01')
    expect(toasts[toasts.length - 1]?.type).toBe('warning')
  })

  it('does not dispatch when the bulk confirmation is cancelled', async () => {
    const { api } = mountUseApt()
    emitFullAptSnapshot({ hosts: [HOST] })
    api.selectedHosts.value = ['h1']

    const dialog = useConfirmDialog()
    const promise = api.bulkAptCmd('dist-upgrade')
    expect(dialog.variant.value).toBe('danger')
    dialog.onCancel()
    await promise

    expect(sendAptCommand).not.toHaveBeenCalled()
  })

  it('shows a translated error dialog when the bulk dispatch request throws', async () => {
    sendAptCommand.mockRejectedValue(new Error('network down'))
    const { api } = mountUseApt()
    emitFullAptSnapshot({ hosts: [HOST] })
    api.selectedHosts.value = ['h1']

    const dialog = useConfirmDialog()
    const promise = api.bulkAptCmd('upgrade')
    dialog.onConfirm() // confirms the "apt upgrade" action dialog
    await vi.advanceTimersByTimeAsync(0) // let the rejected sendAptCommand settle and reopen the dialog as an error
    expect(dialog.title.value).toBe('Erreur')
    expect(dialog.message.value).toContain('network down')
    dialog.onConfirm() // dismiss the error dialog so the function can resolve
    await promise
  })
})

describe('useApt — resume live command tracking on mount', () => {
  it('resumes watching an already-running command found in the first WS snapshot', () => {
    const { api } = mountUseApt()

    wsMessageHandler.current?.({
      type: 'apt',
      hosts: [HOST],
      apt_statuses: {},
      apt_histories: {
        h1: [{ id: 'cmd-resume', action: 'dist-upgrade', status: 'running', output: '', created_at: new Date().toISOString() }],
      },
    })

    expect(openCommandStream).toHaveBeenCalledWith('cmd-resume', expect.anything())
    expect(api.showConsole.value).toBe(true)
    expect(api.liveCommand.value?.id).toBe('cmd-resume')
  })

  it('does not resume-subscribe when the latest command is already terminal', () => {
    const { api } = mountUseApt()

    wsMessageHandler.current?.({
      type: 'apt',
      hosts: [HOST],
      apt_statuses: {},
      apt_histories: {
        h1: [{ id: 'cmd-done', action: 'update', status: 'completed', output: 'ok', created_at: new Date().toISOString() }],
      },
    })

    expect(openCommandStream).not.toHaveBeenCalled()
    expect(api.showConsole.value).toBe(false)
  })

  it('only checks for a resumable command on the first snapshot, so a console the user closed stays closed', () => {
    const { api } = mountUseApt()

    wsMessageHandler.current?.({
      type: 'apt',
      hosts: [HOST],
      apt_statuses: {},
      apt_histories: { h1: [] },
    })
    expect(openCommandStream).not.toHaveBeenCalled()

    // A command starts running and the user watches then closes it manually.
    api.watchCommand({ id: 'cmd-5', action: 'upgrade', status: 'running', output: '' }, HOST)
    expect(openCommandStream).toHaveBeenCalledTimes(1)
    api.closeLiveConsole()
    expect(api.showConsole.value).toBe(false)

    // A later snapshot still reports it running — must not silently reopen it.
    wsMessageHandler.current?.({
      type: 'apt',
      hosts: [HOST],
      apt_statuses: {},
      apt_histories: {
        h1: [{ id: 'cmd-5', action: 'upgrade', status: 'running', output: '', created_at: new Date().toISOString() }],
      },
    })
    expect(openCommandStream).toHaveBeenCalledTimes(1)
    expect(api.showConsole.value).toBe(false)
  })
})
