import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { setLocale } from '../i18n'

const { sendProcessesCommand } = vi.hoisted(() => ({ sendProcessesCommand: vi.fn() }))
const { collectCommandOutput } = vi.hoisted(() => ({ collectCommandOutput: vi.fn() }))

vi.mock('../api', () => ({
  default: { sendProcessesCommand },
  getApiErrorMessage: (e: unknown, fallback: string) => (e instanceof Error ? e.message : fallback),
}))

vi.mock('./useCommandStream', () => ({
  useCommandStream: () => ({ collectCommandOutput }),
}))

import { useHostProcesses } from './useHostProcesses'

// useHostProcesses calls useI18n() internally, which requires an active
// component instance — mount a trivial host component instead of invoking
// the composable at module scope.
function mountHost(hostId = 'h1') {
  let api: ReturnType<typeof useHostProcesses> | undefined
  const wrapper = mount(defineComponent({
    setup() {
      api = useHostProcesses(hostId)
      return () => h('div')
    },
  }))
  return { wrapper, api: api! }
}

describe('useHostProcesses', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
  })

  it('loads and parses the process list', async () => {
    sendProcessesCommand.mockResolvedValue({ data: { command_id: 'cmd-1' } })
    collectCommandOutput.mockResolvedValue(JSON.stringify([
      { pid: 1, name: 'systemd', user: 'root', cpu_pct: 0.1, mem_pct: 0.2, mem_rss_kb: 1024, state: 'S' },
    ]))

    const { api } = mountHost()
    await api.load()

    expect(sendProcessesCommand).toHaveBeenCalledWith('h1')
    expect(collectCommandOutput).toHaveBeenCalledWith('cmd-1', { timeoutMs: 60_000 })
    expect(api.processes.value).toHaveLength(1)
    expect(api.processes.value[0].name).toBe('systemd')
    expect(api.error.value).toBe('')
    expect(api.loading.value).toBe(false)
  })

  it('surfaces a parse error when the output is not valid JSON', async () => {
    sendProcessesCommand.mockResolvedValue({ data: { command_id: 'cmd-1' } })
    collectCommandOutput.mockResolvedValue('not json')

    const { api } = mountHost()
    await api.load()

    expect(api.error.value).toBe('Impossible de parser la liste des processus')
    expect(api.processes.value).toEqual([])
  })

  it('surfaces the stream error message when collectCommandOutput rejects with an Error', async () => {
    sendProcessesCommand.mockResolvedValue({ data: { command_id: 'cmd-1' } })
    collectCommandOutput.mockRejectedValue(new Error('stream timed out'))

    const { api } = mountHost()
    await api.load()

    expect(api.error.value).toBe('stream timed out')
  })

  it('falls back to a translated message when collectCommandOutput rejects with a non-Error', async () => {
    sendProcessesCommand.mockResolvedValue({ data: { command_id: 'cmd-1' } })
    collectCommandOutput.mockRejectedValue('boom')

    const { api } = mountHost()
    await api.load()

    expect(api.error.value).toBe('Erreur lors du chargement des processus')
  })

  it('surfaces an error when the initial command dispatch itself fails', async () => {
    sendProcessesCommand.mockRejectedValue(new Error("Impossible d'envoyer la commande"))

    const { api } = mountHost()
    await api.load()

    expect(api.error.value).toBe("Impossible d'envoyer la commande")
    expect(collectCommandOutput).not.toHaveBeenCalled()
  })

  it('sets loading true during the call and false once settled', async () => {
    sendProcessesCommand.mockResolvedValue({ data: { command_id: 'cmd-1' } })
    let resolveOutput: (v: string) => void
    collectCommandOutput.mockReturnValue(new Promise((resolve) => { resolveOutput = resolve }))

    const { api } = mountHost()
    const loadPromise = api.load()
    expect(api.loading.value).toBe(true)

    resolveOutput!('[]')
    await loadPromise
    expect(api.loading.value).toBe(false)
  })
})
