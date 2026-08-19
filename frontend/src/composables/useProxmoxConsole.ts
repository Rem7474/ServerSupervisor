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
      if (typeof event.data !== 'string') return

      // A console_error is only ever sent once, immediately before the
      // server closes the connection (see ws.ProxmoxConsole) — real shell
      // output essentially never happens to parse as this exact shape.
      try {
        const parsed: unknown = JSON.parse(event.data)
        if (isConsoleError(parsed)) {
          status.value = 'error'
          errorMessage.value = parsed.error
          return
        }
      } catch {
        // Not JSON: ordinary PTY output, fall through to write it.
      }
      term.write(event.data)
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
