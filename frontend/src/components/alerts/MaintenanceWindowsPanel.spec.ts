import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../../i18n'

const { getAllMaintenanceWindows, createMaintenanceWindow, deleteMaintenanceWindow, getHosts } = vi.hoisted(() => ({
  getAllMaintenanceWindows: vi.fn(),
  createMaintenanceWindow: vi.fn(),
  deleteMaintenanceWindow: vi.fn(),
  getHosts: vi.fn(),
}))

vi.mock('../../api', () => ({
  default: {
    getAllMaintenanceWindows, createMaintenanceWindow, deleteMaintenanceWindow,
    createGlobalMaintenanceWindow: vi.fn(),
    getHosts,
  },
  getApiErrorMessage: (e: unknown, fallback: string) => (e instanceof Error ? e.message : fallback),
}))

import { useConfirmDialog } from '../../composables/useConfirmDialog'
import MaintenanceWindowsPanel from './MaintenanceWindowsPanel.vue'

beforeEach(() => {
  vi.clearAllMocks()
  setLocale('fr')
  setActivePinia(createPinia())
  getAllMaintenanceWindows.mockResolvedValue({ data: [] })
  getHosts.mockResolvedValue({ data: [] })
})

describe('MaintenanceWindowsPanel', () => {
  it('shows the empty state when there are no windows', async () => {
    const wrapper = mount(MaintenanceWindowsPanel, { props: { isAdmin: true } })
    await flushPromises()
    expect(wrapper.text()).toContain('Aucune fenêtre de maintenance')
  })

  it('hides the "new window" button for a non-admin', async () => {
    const wrapper = mount(MaintenanceWindowsPanel, { props: { isAdmin: false } })
    await flushPromises()
    expect(wrapper.find('button.btn-primary').exists()).toBe(false)
  })

  it('toggles the create form and submits a new window', async () => {
    createMaintenanceWindow.mockResolvedValue({ data: {} })
    getHosts.mockResolvedValue({ data: [{ id: 'h1', name: 'web-01' }] })
    const wrapper = mount(MaintenanceWindowsPanel, { props: { isAdmin: true } })
    await flushPromises()

    await wrapper.find('button.btn-primary').trigger('click')
    expect(wrapper.find('form').exists()).toBe(true)

    await wrapper.find('select').setValue('h1')
    await wrapper.find('input[type="datetime-local"]').setValue('2026-01-01T00:00')
    const inputs = wrapper.findAll('input[type="datetime-local"]')
    await inputs[0].setValue('2026-01-01T00:00')
    await inputs[1].setValue('2026-01-02T00:00')
    await wrapper.find('input[type="text"]').setValue('Kernel update')

    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(createMaintenanceWindow).toHaveBeenCalledWith('h1', expect.objectContaining({ reason: 'Kernel update' }))
    expect(wrapper.find('form').exists()).toBe(false)
  })

  it('renders a row per window, badging a global (no host) window', async () => {
    getAllMaintenanceWindows.mockResolvedValue({
      data: [
        { id: 1, reason: 'Kernel update', starts_at: '2026-01-01T00:00:00Z', ends_at: '2026-01-02T00:00:00Z', host_id: 'h1', host_name: 'web-01', created_by: 'alice' },
        { id: 2, reason: 'Global patch', starts_at: '2026-01-01T00:00:00Z', ends_at: '2026-01-02T00:00:00Z', host_id: null, created_by: 'bob' },
      ],
    })
    const wrapper = mount(MaintenanceWindowsPanel, { props: { isAdmin: true } })
    await flushPromises()

    expect(wrapper.text()).toContain('web-01')
    expect(wrapper.text()).toContain('Tous les hôtes')
    expect(wrapper.text()).toContain('alice')
  })

  it('deletes a window only after confirming', async () => {
    deleteMaintenanceWindow.mockResolvedValue({})
    getAllMaintenanceWindows.mockResolvedValue({
      data: [{ id: 1, reason: 'Kernel update', starts_at: '', ends_at: '', host_name: 'web-01' }],
    })
    const wrapper = mount(MaintenanceWindowsPanel, { props: { isAdmin: true } })
    await flushPromises()

    const dialog = useConfirmDialog()
    const clickPromise = wrapper.find('button[aria-label="Supprimer la fenêtre de maintenance"]').trigger('click')
    await vi.waitFor(() => expect(dialog.isOpen.value).toBe(true))
    dialog.onConfirm()
    await clickPromise
    await flushPromises()

    expect(deleteMaintenanceWindow).toHaveBeenCalledWith(1)
  })
})
