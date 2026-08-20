import { getCurrentInstance, onUnmounted, ref, type Ref } from 'vue'
import type { Terminal } from '@xterm/xterm'

export type ConsoleStatus = 'idle' | 'connecting' | 'connected' | 'disconnected' | 'error'

interface UseProxmoxConsoleApi {
  status: Ref<ConsoleStatus>
  errorMessage: Ref<string>
  open: (guestId: string, term: Terminal) => void
  resize: (cols: number, rows: number) => void
  close: () => void
}

interface ConsoleErrorMessage {
  type: 'console_error'
  error: string
}

function isConsoleError(value: unknown): value is ConsoleErrorMessage {
  return (
    typeof value === 'object' &&
    value !== null &&
    (value as { type?: unknown }).type === 'console_error'
  )
}

/**
 * Bidirectional transport for one interactive LXC console session
 * (keystrokes/paste -> server, PTY output <- server), backing
 * ProxmoxConsole.vue. Unlike useCommandStream, an unexpected drop here does
 * NOT auto-reconnect: a shell session interrupted mid-command must not
 * silently rebind — the user re-opens explicitly instead. `open` binds
 * directly to an already-constructed xterm.js Terminal instance (its
 * lifecycle — mount/dispose — belongs to the component, not this
 * composable).
 */
export function useProxmoxConsole(): UseProxmoxConsoleApi {
  const status = ref<ConsoleStatus>('idle')
  const errorMessage = ref('')

  let ws: WebSocket | null = null
  let manualClose = false
  let dataDisposable: { dispose: () => void } | null = null

  function createConsoleUrl(guestId: string): string {
    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
    return `${protocol}://${window.location.host}/api/v1/ws/proxmox/console/${encodeURIComponent(guestId)}`
  }

  function close(): void {
    manualClose = true
    status.value = 'idle'
    dataDisposable?.dispose()
    dataDisposable = null
    if (!ws) return
    ws.onopen = null
    ws.onmessage = null
    ws.onerror = null
    ws.onclose = null
    ws.close()
    ws = null
  }

  function open(guestId: string, term: Terminal): void {
    close()
    manualClose = false
    status.value = 'connecting'
    errorMessage.value = ''

    const socket = new WebSocket(createConsoleUrl(guestId))
    // PTY output arrives as binary frames (see ws.ProxmoxConsole's doc
    // comment) — arraybuffer lets us hand xterm.js a Uint8Array directly,
    // whose own parser correctly buffers a multi-byte UTF-8 character split
    // across two frames instead of corrupting it the way per-message
    // string decoding would.
    socket.binaryType = 'arraybuffer'
    ws = socket

    dataDisposable = term.onData((data: string) => {
      if (ws === socket && socket.readyState === WebSocket.OPEN) socket.send(data)
    })

    socket.onopen = (): void => {
      if (ws !== socket) return
      status.value = 'connected'
    }

    socket.onmessage = (event: MessageEvent): void => {
      if (ws !== socket) return

      // Raw PTY output — the server always sends this as a binary frame
      // (see ws.ProxmoxConsole). xterm.js's Uint8Array overload of write()
      // decodes UTF-8 incrementally across calls, unlike treating each
      // message as an independent JS string.
      if (event.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(event.data))
        return
      }
      if (typeof event.data !== 'string') return

      // Anything else is a text control frame — currently only
      // console_error, sent once immediately before the server closes the
      // connection.
      try {
        const parsed: unknown = JSON.parse(event.data)
        if (isConsoleError(parsed)) {
          status.value = 'error'
          errorMessage.value = parsed.error
        }
      } catch {
        // Not a recognized control message: ignore.
      }
    }

    socket.onerror = (): void => {
      if (ws !== socket) return
      status.value = 'error'
    }

    socket.onclose = (): void => {
      const wasCurrent = ws === socket
      if (wasCurrent) ws = null
      if (!wasCurrent) return
      if (status.value !== 'error') status.value = manualClose ? 'idle' : 'disconnected'
    }
  }

  function resize(cols: number, rows: number): void {
    if (!ws || ws.readyState !== WebSocket.OPEN) return
    ws.send(JSON.stringify({ type: 'resize', cols, rows }))
  }

  if (getCurrentInstance()) {
    onUnmounted(() => {
      close()
    })
  }

  return { status, errorMessage, open, resize, close }
}
