import { ref, onMounted, onUnmounted, Ref } from 'vue'
import { useAuthStore } from '../stores/auth'

type WebSocketStatus = 'connecting' | 'connected' | 'reconnecting' | 'error' | 'disconnected'

interface UseWebSocketOptions {
  debounceMs?: number
}

interface SendOptions {
  stringify?: boolean
}

interface UseWebSocketApi {
  wsStatus: Ref<WebSocketStatus>
  wsError: Ref<string>
  retryCount: Ref<number>
  reconnect: () => void
  disconnect: () => void
  send: (message: unknown, options?: SendOptions) => boolean
}

/**
 * WebSocket status values:
 *   'connecting'   — initial connection attempt
 *   'connected'    — open and authenticated
 *   'reconnecting' — lost connection, retrying
 *   'error'        — blocked (403 origin, 401 auth) — no auto-retry
 *   'disconnected' — manually closed
 */
export function useWebSocket<TPayload = unknown>(
  path: string,
  onMessage: (payload: TPayload) => void,
  options: UseWebSocketOptions = {}
): UseWebSocketApi {
  const { debounceMs = 0 } = options
  const auth = useAuthStore()

  const wsStatus: Ref<WebSocketStatus> = ref('connecting')
  const wsError: Ref<string> = ref('')
  const retryCount: Ref<number> = ref(0)

  let ws: WebSocket | null = null
  let retryTimer: ReturnType<typeof setTimeout> | null = null
  let debounceTimer: ReturnType<typeof setTimeout> | null = null
  let manualClose = false

  // Exponential backoff: 2s, 4s, 8s, capped at 30s
  function retryDelay(): number {
    return Math.min(2000 * Math.pow(2, retryCount.value), 30000)
  }

  function connect(): void {
    if (!auth.token) return
    manualClose = false

    // Close any existing socket before opening a new one (prevents double-instance on reconnect)
    if (ws && ws.readyState !== WebSocket.CLOSED) {
      ws.onclose = null
      ws.close()
      ws = null
    }

    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const url = `${protocol}://${window.location.host}${path}`
    ws = new WebSocket(url)

    ws.onopen = () => {
      ws!.send(JSON.stringify({ type: 'auth', token: auth.token }))
      // Status moves to 'connected' only after the first valid message
    }

    ws.onmessage = (event: MessageEvent) => {
      try {
        const payload = JSON.parse(event.data)

        // Auth error from server (invalid/expired token)
        if (payload.type === 'auth_error') {
          wsStatus.value = 'error'
          wsError.value = 'Authentification refusée — reconnectez-vous'
          ws!.close()
          return
        }

        // First valid message = truly connected
        if (wsStatus.value !== 'connected') {
          wsStatus.value = 'connected'
          wsError.value = ''
          retryCount.value = 0
        }

        if (debounceMs > 0) {
          clearTimeout(debounceTimer!)
          debounceTimer = setTimeout(() => onMessage(payload), debounceMs)
        } else {
          onMessage(payload)
        }
      } catch {
        // Ignore malformed payloads
      }
    }

    ws.onerror = () => {
      // onerror always fires just before onclose — let onclose handle retry logic
    }

    ws.onclose = (event: CloseEvent) => {
      if (manualClose) {
        wsStatus.value = 'disconnected'
        return
      }

      // 403 = origin rejected by CheckOrigin (misconfigured BASE_URL)
      // 1008 = policy violation
      if (event.code === 1002 || event.code === 1008) {
        wsStatus.value = 'error'
        wsError.value = 'Connexion refusée par le serveur — vérifiez la configuration BASE_URL'
        return // No retry — it will keep failing
      }

      // 4001 = custom auth error code we could use in future
      if (event.code === 4001) {
        wsStatus.value = 'error'
        wsError.value = 'Session expirée — rechargez la page'
        return
      }

      // For 403 (gorilla sends this as HTTP before upgrade), code will be 1006 (abnormal closure)
      // We detect it via the fact that we never reached 'connected'
      wsStatus.value = 'reconnecting'
      if (retryCount.value === 0) {
        wsError.value = 'Impossible de se connecter — vérifiez que le serveur est accessible et que BASE_URL est correctement configuré'
      } else {
        wsError.value = ''
      }

      retryCount.value++
      const delay = retryDelay()
      retryTimer = setTimeout(connect, delay)
    }
  }

  function disconnect(): void {
    manualClose = true
    if (retryTimer) clearTimeout(retryTimer)
    if (debounceTimer) clearTimeout(debounceTimer)
    if (ws) {
      ws.close()
      ws = null
    }
    wsStatus.value = 'disconnected'
  }

  function send(message: unknown, { stringify = true }: SendOptions = {}): boolean {
    if (!ws || ws.readyState !== WebSocket.OPEN) return false
    try {
      ws.send(stringify ? JSON.stringify(message) : (message as string))
      return true
    } catch {
      return false
    }
  }

  onMounted(connect)
  onUnmounted(disconnect)

  return { wsStatus, wsError, retryCount, reconnect: connect, disconnect, send }
}
