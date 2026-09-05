import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'

const { getRunbooks, createRunbook, updateRunbook, deleteRunbook, runRunbook, getRunbookExecutions, getRunbookExecution } = vi.hoisted(() => ({
  getRunbooks: vi.fn(),
  createRunbook: vi.fn(),
  updateRunbook: vi.fn(),
  deleteRunbook: vi.fn(),
  runRunbook: vi.fn(),
  getRunbookExecutions: vi.fn(),
  getRunbookExecution: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { getRunbooks, createRunbook, updateRunbook, deleteRunbook, runRunbook, getRunbookExecutions, getRunbookExecution },
}))

vi.mock('../api/client', () => ({
  getApiErrorMessage: (e: unknown, fallback?: string) => (e instanceof Error && e.message ? e.message : fallback ?? 'error'),
}))

import { useRunbooks, actionsForModule, moduleRequiresTarget, emptyStep } from './useRunbooks'

function mountHost() {
  let api: ReturnType<typeof useRunbooks> | undefined
  mount(defineComponent({
    setup() {
      api = useRunbooks()
      return () => h('div')
    },
  }))
  return api!
}

describe('useRunbooks — module-level helpers', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('actionsForModule returns translated labels for docker and falls back to the raw value for an unknown module', () => {
    const docker = actionsForModule('docker')
    expect(docker.find((a) => a.value === 'restart')?.label).toBe('Redémarrer')
    expect(docker.find((a) => a.value === 'compose_pull')?.label).toBe('Mettre à jour les images')
    expect(actionsForModule('unknown-module')).toEqual([])
  })

  it('translates action labels to English when the locale is switched', () => {
    setLocale('en')
    const docker = actionsForModule('docker')
    expect(docker.find((a) => a.value === 'restart')?.label).toBe('Restart')
  })

  it('moduleRequiresTarget flags journal/systemd/custom only', () => {
    expect(moduleRequiresTarget('journal')).toBe(true)
    expect(moduleRequiresTarget('systemd')).toBe(true)
    expect(moduleRequiresTarget('custom')).toBe(true)
    expect(moduleRequiresTarget('docker')).toBe(false)
  })

  it('emptyStep returns a docker/restart step template', () => {
    expect(emptyStep()).toEqual({ host_id: '', module: 'docker', action: 'restart', target: '', payload: '', continue_on_failure: false })
  })
})

describe('useRunbooks()', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    setActivePinia(createPinia())
    getRunbooks.mockResolvedValue({ data: [] })
  })

  it('loads runbooks and marks fetched even on error', async () => {
    getRunbooks.mockRejectedValue(new Error('boom'))
    const api = mountHost()
    await api.loadRunbooks()
    expect(api.error.value).toBe('boom')
    expect(api.fetched.value).toBe(true)
    expect(api.runbooks.value).toEqual([])
  })

  it('creates a runbook, reloads the list and closes the modal', async () => {
    createRunbook.mockResolvedValue({})
    getRunbooks.mockResolvedValue({ data: [{ id: 'r1', name: 'x', steps: [] }] })
    const api = mountHost()
    await flushPromises()
    api.startAdd()
    const ok = await api.saveRunbook('name', 'desc', [])
    expect(ok).toBe(true)
    expect(api.showModal.value).toBe(false)
    expect(api.runbooks.value).toHaveLength(1)
  })

  it('updates an existing runbook when editing', async () => {
    const existing = { id: 'r1', name: 'x', description: '', steps: [] }
    getRunbooks.mockResolvedValue({ data: [existing] })
    updateRunbook.mockResolvedValue({})
    const api = mountHost()
    await flushPromises()
    api.startEdit(existing as never)
    await api.saveRunbook('renamed', '', [])
    expect(updateRunbook).toHaveBeenCalledWith('r1', expect.objectContaining({ name: 'renamed' }))
  })

  it('reports a save error without closing the modal', async () => {
    createRunbook.mockRejectedValue(new Error('save failed'))
    const api = mountHost()
    await flushPromises()
    api.startAdd()
    const ok = await api.saveRunbook('x', '', [])
    expect(ok).toBe(false)
    expect(api.saveError.value).toBe('save failed')
    expect(api.showModal.value).toBe(true)
  })

  it('deletes a runbook and reports an error on failure', async () => {
    deleteRunbook.mockRejectedValue(new Error('delete failed'))
    const api = mountHost()
    await flushPromises()
    await api.deleteRunbook({ id: 'r1' } as never)
    expect(api.error.value).toBe('delete failed')
  })

  it('runs a runbook, opens its history and selects the latest execution', async () => {
    runRunbook.mockResolvedValue({})
    getRunbookExecutions.mockResolvedValue({ data: [{ id: 'e1', status: 'running' }] })
    getRunbookExecution.mockResolvedValue({ data: { id: 'e1', status: 'running', steps: [] } })
    const api = mountHost()
    await flushPromises()
    await api.runRunbook({ id: 'r1', name: 'x' } as never)
    expect(api.historyRunbook.value?.id).toBe('r1')
    expect(api.selectedExecution.value?.id).toBe('e1')
    expect(api.runningIds.value.has('r1')).toBe(false)
  })

  it('reports an error when running a runbook fails', async () => {
    runRunbook.mockRejectedValue(new Error('run failed'))
    const api = mountHost()
    await flushPromises()
    await api.runRunbook({ id: 'r1', name: 'x' } as never)
    expect(api.error.value).toBe('run failed')
  })

  it('opens and closes the history modal, clearing selection state', async () => {
    getRunbookExecutions.mockResolvedValue({ data: [] })
    const api = mountHost()
    await flushPromises()
    await api.openHistory({ id: 'r1', name: 'x' } as never)
    expect(api.historyRunbook.value?.id).toBe('r1')
    api.closeHistory()
    expect(api.historyRunbook.value).toBeNull()
    expect(api.executions.value).toEqual([])
  })
})
