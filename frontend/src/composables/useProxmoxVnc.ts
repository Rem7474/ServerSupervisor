import { getCurrentInstance, onUnmounted, ref, type Ref } from 'vue'
import type RFB from '@novnc/novnc'
import { i18n } from '../i18n'

export type VncStatus = 'idle' | 'connecting' | 'connected' | 'disconnected' | 'error'

interface UseProxmoxVncApi {
  status: Ref<VncStatus>
  errorMessage: Ref<string>
  open: (guestId: string, target: HTMLElement) => Promise<void>
  sendCtrlAltDel: () => void
  focus: () => void
  close: () => void
}

/** Close codes 4000-4999 carry the server's reason (see ws.ProxmoxVNC). */
function isServerError(code: number): boolean {
  return code >= 4000 && code < 5000
}

/**
 * The server puts an errors.json catalog key in the close reason when it has
 * one (PROXMOX_CONSOLE_NOT_CONFIGURED, …), otherwise PVE's own message.
 */
function closeReasonMessage(reason: string): string {
  if (reason && i18n.global.te(`errors.${reason}`)) return i18n.global.t(`errors.${reason}`)
  return reason || i18n.global.t('proxmox.vncConnectionFailed')
}

/**
 * One QEMU VNC session, backing ProxmoxVncConsole.vue. The browser talks RFB
 * to this app's own WebSocket (/api/v1/ws/proxmox/vnc/:guest_id), never to
 * PVE: the server holds the PVE ticket and answers the VNC password
 * challenge itself, then offers noVNC the "None" security type. No automatic
 * reconnect, same as the LXC console: the user reconnects explicitly.
 *
 * The WebSocket is created here and handed to noVNC (instead of a URL) so its
 * close code and reason stay readable: noVNC only reports a clean/unclean
 * flag, which is not enough to say why the server refused the session.
 */
export function useProxmoxVnc(): UseProxmoxVncApi {
  const status = ref<VncStatus>('idle')
  const errorMessage = ref('')

  let rfb: RFB | null = null
  // Bumped by every open/close: callbacks of a previous session (including
  // an open still waiting on the noVNC import) check it and stand down.
  let generation = 0

  function createVncUrl(guestId: string): string {
    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
    return `${protocol}://${window.location.host}/api/v1/ws/proxmox/vnc/${encodeURIComponent(guestId)}`
  }

  function fail(message: string): void {
    status.value = 'error'
    errorMessage.value = message
  }

  function close(): void {
    generation++
    const current = rfb
    rfb = null
    status.value = 'idle'
    current?.disconnect()
  }

  async function open(guestId: string, target: HTMLElement): Promise<void> {
    close()
    const gen = generation
    status.value = 'connecting'
    errorMessage.value = ''

    let RFBClass: typeof RFB
    try {
      // Lazy: noVNC is only fetched once someone actually opens a VM console.
      RFBClass = (await import('@novnc/novnc')).default
    } catch {
      if (gen === generation) fail(i18n.global.t('proxmox.vncLoadFailed'))
      return
    }
    if (gen !== generation) return

    const socket = new WebSocket(createVncUrl(guestId))
    socket.binaryType = 'arraybuffer'
    // Registered before noVNC attaches its own handler, so the server's
    // reason is recorded before noVNC reports the disconnect.
    socket.addEventListener('close', (event: CloseEvent) => {
      if (gen !== generation) return
      if (isServerError(event.code)) fail(closeReasonMessage(event.reason))
    })

    const client = new RFBClass(target, socket, { shared: true })
    // Fit the VM's framebuffer into the dialog rather than resizing the
    // guest's own display, which would change the VM's resolution for
    // every other viewer.
    client.scaleViewport = true
    client.resizeSession = false
    rfb = client

    client.addEventListener('connect', () => {
      if (gen === generation) status.value = 'connected'
    })
    client.addEventListener('disconnect', (event: Event) => {
      if (gen !== generation) return
      rfb = null
      if (status.value === 'error') return
      const clean = (event as CustomEvent<{ clean?: boolean }>).detail?.clean
      if (clean) status.value = 'disconnected'
      else fail(i18n.global.t('proxmox.vncConnectionFailed'))
    })
    client.addEventListener('securityfailure', (event: Event) => {
      if (gen !== generation) return
      const reason = (event as CustomEvent<{ reason?: string }>).detail?.reason
      fail(reason || i18n.global.t('proxmox.vncConnectionFailed'))
    })
  }

  function sendCtrlAltDel(): void {
    if (status.value !== 'connected') return
    rfb?.sendCtrlAltDel()
  }

  function focus(): void {
    rfb?.focus()
  }

  if (getCurrentInstance()) {
    onUnmounted(close)
  }

  return { status, errorMessage, open, sendCtrlAltDel, focus, close }
}
