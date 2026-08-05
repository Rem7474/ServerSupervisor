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

vi.mock('../api', () => ({
  default: { sendAptCommand: vi.fn() },
  getApiErrorMessage: (e: unknown) => String(e),
}))

import { useApt } from './useApt'
import type { Host } from '../types/host'

const HOST: Host = { id: 'h1', name: 'web-01' } as Host

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

beforeEach(() => {
  setActivePinia(createPinia())
  streamOptionsByCommand.clear()
  wsMessageHandler.current = null
  openCommandStream.mockClear()
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
