import { ref, onMounted, onUnmounted } from 'vue'
import { useAuthStore } from '../stores/auth'

/**
 * WebSocket status values:
 *   'connecting'   — initial connection attempt
 *   'connected'    — open and authenticated
 *   'reconnecting' — lost connection, retrying
 *   'error'        — blocked (403 origin, 401 auth) — no auto-retry
 *   'disconnected' — manually closed
 */

export function useWebSocket(path, onMessage) {
  const auth = useAuthStore()

  const wsStatus = ref('connecting')
  const wsError = ref('')       // human-readable error message
  const retryCount = ref(0)

  let ws = null
  let retryTimer = null
  let manualClose = false

  // Exponential backoff: 2s, 4s, 8s, capped at 30s
  function retryDelay() {
    return Math.min(2000 * Math.pow(2, retryCount.value), 30000)
  }

  function connect() {
    if (!auth.token) return
    manualClose = false

    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const url = `${protocol}://${window.location.host}${path}`
    ws = new WebSocket(url)

    ws.onopen = () => {
      ws.send(JSON.stringify({ type: 'auth', token: auth.token }))
      // Status moves to 'connected' only after the first valid message
    }

    ws.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data)

        // Auth error from server (invalid/expired token)
        if (payload.type === 'auth_error') {
          wsStatus.value = 'error'
          wsError.value = 'Authentification refusée — reconnectez-vous'
          ws.close()
          return
        }

        // First valid message = truly connected
        if (wsStatus.value !== 'connected') {
          wsStatus.value = 'connected'
          wsError.value = ''
          retryCount.value = 0
        }

        onMessage(payload)
      } catch {
        // Ignore malformed payloads
      }
    }

    ws.onerror = () => {
      // onerror always fires just before onclose — let onclose handle retry logic
    }

    ws.onclose = (event) => {
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
      if (wsStatus.value === 'connecting' && retryCount.value === 0) {
        wsStatus.value = 'error'
        wsError.value = 'Impossible de se connecter — vérifiez que le serveur est accessible et que BASE_URL est correctement configuré'
        // Still retry, maybe it's a temporary glitch
      } else {
        wsStatus.value = 'reconnecting'
        wsError.value = ''
      }

      retryCount.value++
      const delay = retryDelay()
      retryTimer = setTimeout(connect, delay)
    }
  }

  function disconnect() {
    manualClose = true
    clearTimeout(retryTimer)
    if (ws) {
      ws.close()
      ws = null
    }
    wsStatus.value = 'disconnected'
  }

  onMounted(connect)
  onUnmounted(disconnect)

  return { wsStatus, wsError, retryCount, reconnect: connect }
}