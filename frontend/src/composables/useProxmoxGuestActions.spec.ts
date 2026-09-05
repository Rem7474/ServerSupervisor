import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../i18n'
import { useConfirmDialog } from './useConfirmDialog'

const { proxmoxGuestAction } = vi.hoisted(() => ({
  proxmoxGuestAction: vi.fn(),
}))

vi.mock('../api', () => ({
  default: { proxmoxGuestAction },
}))

const { addToast } = vi.hoisted(() => ({ addToast: vi.fn() }))
vi.mock('./useGlobalToast', () => ({ addToast }))

import { useProxmoxGuestActions } from './useProxmoxGuestActions'

function mountHost() {
  let api!: ReturnType<typeof useProxmoxGuestActions>
  const wrapper = mount({
    setup() {
      api = useProxmoxGuestActions()
      return () => null
    },
  })
  return { wrapper, api: api! }
}

describe('useProxmoxGuestActions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
  })

  it('confirms with the guest name in the dialog title, and shows a translated success toast', async () => {
    proxmoxGuestAction.mockResolvedValue({ data: {} })
    const { api } = mountHost()
    const dialog = useConfirmDialog()

    const actionPromise = api.performGuestAction({ id: 'g1', name: 'web-01', vmid: 100 }, 'shutdown')
    await vi.waitFor(() => expect(dialog.isOpen.value).toBe(true))
    expect(dialog.title.value).toBe('Arrêter web-01 ?')
    dialog.onConfirm()
    await actionPromise

    expect(proxmoxGuestAction).toHaveBeenCalledWith('g1', 'shutdown')
    expect(addToast).toHaveBeenCalledWith('Arrêter envoyé.', 'success')
  })

  it('falls back to "#<vmid>" in the dialog title when the guest has no name', async () => {
    const { api } = mountHost()
    const dialog = useConfirmDialog()

    const actionPromise = api.performGuestAction({ id: 'g1', vmid: 100 }, 'reboot')
    await vi.waitFor(() => expect(dialog.isOpen.value).toBe(true))
    expect(dialog.title.value).toBe('Redémarrer #100 ?')
    dialog.onCancel()
    await actionPromise

    expect(proxmoxGuestAction).not.toHaveBeenCalled()
  })

  it('shows a translated error toast when the action fails', async () => {
    proxmoxGuestAction.mockRejectedValue(new Error('boom'))
    const { api } = mountHost()
    const dialog = useConfirmDialog()

    const actionPromise = api.performGuestAction({ id: 'g1', name: 'web-01', vmid: 100 }, 'shutdown')
    await vi.waitFor(() => expect(dialog.isOpen.value).toBe(true))
    dialog.onConfirm()
    await actionPromise

    expect(addToast).toHaveBeenCalledWith('boom', 'error')
  })
})
