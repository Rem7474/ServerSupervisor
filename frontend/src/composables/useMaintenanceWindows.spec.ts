import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { setLocale } from '../i18n'
import { useConfirmDialog } from './useConfirmDialog'

const { getAllMaintenanceWindows, createMaintenanceWindow, createGlobalMaintenanceWindow, deleteMaintenanceWindow } = vi.hoisted(() => ({
  getAllMaintenanceWindows: vi.fn(),
  createMaintenanceWindow: vi.fn(),
  createGlobalMaintenanceWindow: vi.fn(),
  deleteMaintenanceWindow: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { getAllMaintenanceWindows, createMaintenanceWindow, createGlobalMaintenanceWindow, deleteMaintenanceWindow },
  getApiErrorMessage: (e: unknown, fallback: string) => (e instanceof Error ? e.message : fallback),
}))

import { useMaintenanceWindows } from './useMaintenanceWindows'

// useI18n() and useConfirmDialog() both need an active component instance.
function mountHost() {
  let api: ReturnType<typeof useMaintenanceWindows> | undefined
  const wrapper = mount(defineComponent({
    setup() {
      api = useMaintenanceWindows()
      return () => h('div')
    },
  }))
  return { wrapper, api: api! }
}

describe('useMaintenanceWindows', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
  })

  it('loads windows', async () => {
    getAllMaintenanceWindows.mockResolvedValue({ data: [{ id: 1, reason: 'Kernel', starts_at: '', ends_at: '' }] })
    const { api } = mountHost()
    await api.load()
    expect(api.windows.value).toHaveLength(1)
    expect(api.fetched.value).toBe(true)
  })

  it('surfaces a translated fallback error on a failed load', async () => {
    getAllMaintenanceWindows.mockRejectedValue('non-error rejection')
    const { api } = mountHost()
    await api.load()
    expect(api.error.value).toBe('Impossible de charger les fenêtres de maintenance')
  })

  it('creates a host-scoped window', async () => {
    createMaintenanceWindow.mockResolvedValue({ data: {} })
    getAllMaintenanceWindows.mockResolvedValue({ data: [] })
    const { api } = mountHost()
    const ok = await api.create('h1', { reason: 'x', starts_at: 'a', ends_at: 'b' })
    expect(ok).toBe(true)
    expect(createMaintenanceWindow).toHaveBeenCalledWith('h1', { reason: 'x', starts_at: 'a', ends_at: 'b' })
    expect(createGlobalMaintenanceWindow).not.toHaveBeenCalled()
  })

  it('creates a global window when hostId is null', async () => {
    createGlobalMaintenanceWindow.mockResolvedValue({ data: {} })
    getAllMaintenanceWindows.mockResolvedValue({ data: [] })
    const { api } = mountHost()
    await api.create(null, { reason: 'x', starts_at: 'a', ends_at: 'b' })
    expect(createGlobalMaintenanceWindow).toHaveBeenCalled()
  })

  it('surfaces a translated create error and returns false', async () => {
    createMaintenanceWindow.mockRejectedValue(new Error('boom'))
    const { api } = mountHost()
    const ok = await api.create('h1', { reason: 'x', starts_at: 'a', ends_at: 'b' })
    expect(ok).toBe(false)
    expect(api.saveError.value).toBe('boom')
  })

  it('deletes a window only after confirming, using the host name in the message', async () => {
    deleteMaintenanceWindow.mockResolvedValue({})
    const { api } = mountHost()
    api.windows.value = [{ id: 1, reason: 'Kernel update', starts_at: '', ends_at: '', host_name: 'web-01' } as never]

    const dialog = useConfirmDialog()
    const removePromise = api.remove(api.windows.value[0])
    await vi.waitFor(() => expect(dialog.isOpen.value).toBe(true))
    expect(dialog.title.value).toBe('Supprimer la fenêtre de maintenance')
    expect(dialog.message.value).toBe('Supprimer la fenêtre de maintenance "Kernel update" (web-01) ?')

    dialog.onConfirm()
    await removePromise
    expect(deleteMaintenanceWindow).toHaveBeenCalledWith(1)
    expect(api.windows.value).toHaveLength(0)
  })

  it('falls back to the translated "all hosts" label when a window has no host', async () => {
    const { api } = mountHost()
    api.windows.value = [{ id: 2, reason: 'Global', starts_at: '', ends_at: '', host_id: null } as never]

    const dialog = useConfirmDialog()
    const removePromise = api.remove(api.windows.value[0])
    await vi.waitFor(() => expect(dialog.isOpen.value).toBe(true))
    expect(dialog.message.value).toContain('tous les hôtes')

    dialog.onCancel()
    await removePromise
    expect(deleteMaintenanceWindow).not.toHaveBeenCalled()
  })
})
