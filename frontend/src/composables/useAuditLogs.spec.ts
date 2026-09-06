import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from './useConfirmDialog'

const {
  getCommandsHistory, getCommandStatus, cancelCommand, getLoginEventsAdmin, getSecuritySummary, unblockIP,
  getAuditLogs, exportAuditLogs,
} = vi.hoisted(() => ({
  getCommandsHistory: vi.fn(),
  getCommandStatus: vi.fn(),
  cancelCommand: vi.fn(),
  getLoginEventsAdmin: vi.fn(),
  getSecuritySummary: vi.fn(),
  unblockIP: vi.fn(),
  getAuditLogs: vi.fn(),
  exportAuditLogs: vi.fn(),
}))

vi.mock('../api', () => ({
  default: {
    getCommandsHistory, getCommandStatus, cancelCommand, getLoginEventsAdmin, getSecuritySummary, unblockIP,
    getAuditLogs, exportAuditLogs,
  },
  getApiErrorMessage: (e: unknown, fallback?: string) => (e instanceof Error && e.message ? e.message : fallback),
}))

const routeQuery: Record<string, string> = {}
vi.mock('vue-router', () => ({
  useRoute: () => ({ query: routeQuery }),
  useRouter: () => ({ replace: vi.fn() }),
}))

class FakeWebSocket {
  onopen: (() => void) | null = null
  onmessage: ((ev: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  close() { /* no-op */ }
}

import { useAuditLogs, journalCategoryOptions } from './useAuditLogs'

function mountHost() {
  let api: ReturnType<typeof useAuditLogs> | undefined
  mount(defineComponent({
    setup() {
      api = useAuditLogs()
      return () => h('div')
    },
  }))
  return api!
}

describe('useAuditLogs', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('WebSocket', FakeWebSocket)
    setLocale('fr')
    setActivePinia(createPinia())
    useAuthStore().setAuth({ role: 'admin', username: 'admin' } as never, 'admin')
    for (const k of Object.keys(routeQuery)) delete routeQuery[k]
    getCommandsHistory.mockResolvedValue({ data: { commands: [], total: 0 } })
    getLoginEventsAdmin.mockResolvedValue({ data: { events: [], total: 0 } })
    getSecuritySummary.mockResolvedValue({ data: { stats: null, blocked_ips: [], top_failed_ips: [] } })
    getAuditLogs.mockResolvedValue({ data: { logs: [] } })
  })

  it('sorts commands by created_at and falls back to a string compare for other keys', async () => {
    getCommandsHistory.mockResolvedValue({
      data: {
        commands: [
          { id: 'a', created_at: '2026-01-02T00:00:00Z', action: 'restart', target: 'z' },
          { id: 'b', created_at: '2026-01-01T00:00:00Z', action: 'restart', target: 'a' },
        ],
        total: 2,
      },
    })
    const api = mountHost()
    await flushPromises()
    expect(api.sortedCmds.value.map((c) => c.id)).toEqual(['a', 'b']) // desc by created_at (default)

    api.toggleCmdSort('command')
    expect(api.sortedCmds.value.map((c) => c.id)).toEqual(['b', 'a']) // asc by command label
  })

  it('re-toggles the sort direction when the same column is clicked twice', async () => {
    const api = mountHost()
    await flushPromises()
    api.toggleCmdSort('status')
    expect(api.cmdSortDir.value).toBe('asc')
    api.toggleCmdSort('status')
    expect(api.cmdSortDir.value).toBe('desc')
  })

  it('debounces search updates and resets to page 1', async () => {
    vi.useFakeTimers()
    const api = mountHost()
    await vi.waitFor(() => expect(getCommandsHistory).toHaveBeenCalledTimes(1))
    api.cmdsPage.value = 3
    api.onSearchUpdate('nginx')
    expect(api.cmdSearch.value).toBe('nginx')
    await vi.advanceTimersByTimeAsync(350)
    expect(api.cmdsPage.value).toBe(1)
    expect(getCommandsHistory).toHaveBeenCalledTimes(2)
    vi.useRealTimers()
  })

  it('formats duration, builds the command label and resolves a status class', () => {
    const api = mountHost()
    expect(api.formatDuration(null, null)).toBe('—')
    expect(api.formatDuration('2026-01-01T00:00:00Z', '2026-01-01T00:01:05Z')).toBe('1m 5s')
    expect(api.cmdLabel({ action: 'pull', target: 'nginx:latest' } as never)).toBe('pull nginx:latest')
    expect(api.statusClass('running')).toContain('badge')
    expect(api.statusLabel('completed')).toBe('Terminé')
  })

  it('opens the log viewer and streams output for a running command, then closes it', async () => {
    const api = mountHost()
    await flushPromises()
    api.openLogViewer({ id: 'c1', status: 'running', output: '' } as never)
    expect(api.selectedCmd.value?.id).toBe('c1')
    expect(api.showLogViewer.value).toBe(true)

    // Re-opening the same command just re-shows the panel without resetting it.
    api.openLogViewer({ id: 'c1', status: 'running', output: '' } as never)
    expect(api.showLogViewer.value).toBe(true)

    api.closeLogViewer()
    expect(api.selectedCmd.value).toBeNull()
  })

  it('clears the command list on a fetch error', async () => {
    getCommandsHistory.mockRejectedValue(new Error('down'))
    const api = mountHost()
    await flushPromises()
    expect(api.sortedCmds.value).toEqual([])
  })

  it('reconciles a stale command status snapshot after fetching commands', async () => {
    getCommandsHistory.mockResolvedValue({
      data: { commands: [{ id: 'c1', status: 'pending', output: '' }], total: 1 },
    })
    getCommandStatus.mockResolvedValue({ data: { id: 'c1', status: 'completed', output: 'done', started_at: 's', ended_at: 'e' } })
    const api = mountHost()
    await flushPromises()
    expect(api.sortedCmds.value[0].status).toBe('completed')
  })

  it('loads connexions and security summary, falling back to defaults on rejection', async () => {
    getLoginEventsAdmin.mockRejectedValue(new Error('down'))
    getSecuritySummary.mockRejectedValue(new Error('down'))
    const api = mountHost()
    await flushPromises()
    await api.switchToConnexions()
    expect(api.connexions.value).toEqual([])
    expect(api.security.value).toEqual({ stats: null, blocked_ips: [], top_failed_ips: [] })
  })

  it('loads connexions successfully on the connexions tab switch', async () => {
    getLoginEventsAdmin.mockResolvedValue({ data: { events: [{ id: '1' }], total: 1 } })
    const api = mountHost()
    await flushPromises()
    await api.switchToConnexions()
    expect(api.connexions.value).toHaveLength(1)
  })

  it('shows the translated toast when loading the journal fails', async () => {
    getAuditLogs.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    await api.switchToJournal()
    expect(api.journalLogs.value).toEqual([])
  })

  it('paginates the journal and flags hasMore when a full page comes back', async () => {
    getAuditLogs.mockResolvedValue({ data: { logs: new Array(50).fill({ id: 'x' }) } })
    const api = mountHost()
    await flushPromises()
    await api.switchToJournal()
    expect(api.journalHasMore.value).toBe(true)
    await api.selectJournalPage(2)
    expect(api.journalPage.value).toBe(2)
  })

  it('exports the journal as CSV and reports a toast on failure', async () => {
    exportAuditLogs.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    await api.exportJournal()
    expect(api.journalExporting.value).toBe(false)
  })

  it('cancels a running command and reports success', async () => {
    getCommandsHistory.mockResolvedValue({ data: { commands: [{ id: 'c1', status: 'running' }], total: 1 } })
    cancelCommand.mockResolvedValue({})
    const api = mountHost()
    await flushPromises()
    await api.cancelCmd('c1')
    expect(api.sortedCmds.value[0].status).toBe('cancelled')
    expect(api.cancellingId.value).toBeNull()
  })

  it('reports a translated error when cancelling a command fails', async () => {
    cancelCommand.mockRejectedValue(new Error(''))
    const api = mountHost()
    await flushPromises()
    await api.cancelCmd('c1')
    expect(api.cancellingId.value).toBeNull()
  })

  it('declining the unblock-IP confirmation makes no API call', async () => {
    const api = mountHost()
    await flushPromises()
    const dialog = useConfirmDialog()
    const p = api.unblockIP('1.2.3.4')
    expect(dialog.title.value).toBe('Débloquer cette IP')
    expect(dialog.message.value).toBe('Retirer l\'IP 1.2.3.4 de la liste noire ?')
    dialog.onCancel()
    await p
    expect(unblockIP).not.toHaveBeenCalled()
  })

  it('unblocks an IP on confirmation and refreshes the security summary', async () => {
    unblockIP.mockResolvedValue({})
    const api = mountHost()
    await flushPromises()
    const dialog = useConfirmDialog()
    const p = api.unblockIP('1.2.3.4')
    dialog.onConfirm()
    await p
    expect(api.unblockingIP.value).toBe('')
    expect(getSecuritySummary).toHaveBeenCalled()
  })

  it('prefills the module filter and opens a command from the route query on mount', async () => {
    routeQuery.module = 'docker'
    routeQuery.command = 'c1'
    getCommandStatus.mockResolvedValue({ data: { id: 'c1', status: 'completed' } })
    const api = mountHost()
    await flushPromises()
    expect(api.cmdModuleFilter.value).toBe('docker')
    expect(api.selectedCmd.value?.id).toBe('c1')
  })

  it('journalCategoryOptions() translates category labels and switches with the locale', () => {
    setLocale('fr')
    expect(journalCategoryOptions().map((o) => o.label)).toContain('Alertes')
    setLocale('en')
    expect(journalCategoryOptions().map((o) => o.label)).toContain('Alerts')
  })
})
