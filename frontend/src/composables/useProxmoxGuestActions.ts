import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import api from '../api'
import { getApiErrorMessage } from '../api/client'
import { useConfirmDialog } from './useConfirmDialog'
import { addToast } from './useGlobalToast'

export type GuestPowerAction = 'start' | 'shutdown' | 'reboot'

interface GuestActionTarget {
  id: string
  name?: string
  vmid?: number
}

// Shared by ProxmoxGuestView (one guest, admin-only power actions) and
// ProxmoxNodeGuestsTab (the node's VM/LXC list — the buttons used to only
// exist on the single-guest detail page, forcing a click-through just to
// start/stop a guest from the node's own list).
export function useProxmoxGuestActions() {
  const { t } = useI18n()
  const dialog = useConfirmDialog()

  const actionLabels: Record<GuestPowerAction, string> = {
    start: t('proxmox.startButton'),
    shutdown: t('proxmox.stopButton'),
    reboot: t('proxmox.restartButton'),
  }
  // Keyed by guest id (not a single scalar) so a table with many rows shows a
  // per-row spinner without one guest's in-flight action disabling every
  // other row's buttons.
  const actionLoading = ref<Record<string, GuestPowerAction | undefined>>({})

  function isLoading(guestId: string | undefined): GuestPowerAction | null {
    if (!guestId) return null
    return actionLoading.value[guestId] ?? null
  }

  // start doesn't interrupt anything running, so it's fired without a confirm
  // step; shutdown/reboot do, so they go through the same dialog pattern used
  // for every other destructive action in this app.
  async function performGuestAction(
    guest: GuestActionTarget,
    action: GuestPowerAction,
    onDone?: () => void | Promise<void>
  ): Promise<void> {
    if (action !== 'start') {
      const confirmed = await dialog.confirm({
        title: t('proxmox.guestActionConfirmTitle', { action: actionLabels[action], name: guest.name || `#${guest.vmid}` }),
        message: action === 'shutdown'
          ? t('proxmox.guestShutdownConfirmMessage')
          : t('proxmox.guestRebootConfirmMessage'),
        variant: 'danger',
        okLabel: actionLabels[action],
      })
      if (!confirmed) return
    }
    actionLoading.value = { ...actionLoading.value, [guest.id]: action }
    try {
      await api.proxmoxGuestAction(guest.id, action)
      addToast(t('proxmox.guestActionSentToast', { action: actionLabels[action] }), 'success')
      await onDone?.()
    } catch (e: unknown) {
      addToast(getApiErrorMessage(e, t('proxmox.guestActionFailedToast', { action: actionLabels[action] })), 'error')
    } finally {
      const next = { ...actionLoading.value }
      delete next[guest.id]
      actionLoading.value = next
    }
  }

  return { actionLoading, isLoading, performGuestAction }
}
