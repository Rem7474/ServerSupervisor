import { ref } from 'vue'
import api from '../api'
import { getApiErrorMessage } from '../api/client'
import { useConfirmDialog } from './useConfirmDialog'
import { addToast } from './useGlobalToast'

export type GuestPowerAction = 'start' | 'shutdown' | 'reboot'

export const GUEST_ACTION_LABELS: Record<GuestPowerAction, string> = {
  start: 'Démarrer',
  shutdown: 'Arrêter',
  reboot: 'Redémarrer',
}

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
  const dialog = useConfirmDialog()
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
        title: `${GUEST_ACTION_LABELS[action]} ${guest.name || `#${guest.vmid}`} ?`,
        message: action === 'shutdown'
          ? 'Une extinction propre (ACPI) sera demandée à la VM/CT. Les services qui y tournent seront interrompus.'
          : 'La VM/CT va redémarrer immédiatement. Les services qui y tournent seront interrompus le temps du redémarrage.',
        variant: 'danger',
        okLabel: GUEST_ACTION_LABELS[action],
      })
      if (!confirmed) return
    }
    actionLoading.value = { ...actionLoading.value, [guest.id]: action }
    try {
      await api.proxmoxGuestAction(guest.id, action)
      addToast(`${GUEST_ACTION_LABELS[action]} envoyé.`, 'success')
      await onDone?.()
    } catch (e: unknown) {
      addToast(getApiErrorMessage(e, `Échec de l'action "${GUEST_ACTION_LABELS[action]}"`), 'error')
    } finally {
      const next = { ...actionLoading.value }
      delete next[guest.id]
      actionLoading.value = next
    }
  }

  return { actionLoading, isLoading, performGuestAction }
}
