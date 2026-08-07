import { getCurrentInstance, onUnmounted } from 'vue'
import type {
  CommandStreamMessage,
  CommandStreamInitMsg,
  CommandStreamChunkMsg,
  CommandStatusUpdateMsg,
} from '../types/ws'

type TokenSource = string | { value: string } | (() => string)

interface CommandStreamOptions {
  onInit?: (payload: CommandStreamInitMsg) => void
  onChunk?: (payload: CommandStreamChunkMsg) => void
  onStatus?: (payload: CommandStatusUpdateMsg) => void
  onClose?: () => void
  onError?: (error: Error) => void
  closeOnTerminalStatus?: boolean
  terminalCloseDelayMs?: number
}

interface CollectCommandOutputOptions {
  timeoutMs?: number
  onInit?: (payload: CommandStreamInitMsg, output: string) => void
  onChunk?: (payload: CommandStreamChunkMsg, output: string) => void
  onStatus?: (payload: CommandStatusUpdateMsg, output: string) => void
}

interface UseCommandStreamApi {
  openCommandStream: (commandId: string, options?: CommandStreamOptions) => WebSocket
  collectCommandOutput: (commandId: string, options?: CollectCommandOutputOptions) => Promise<string>
  closeStream: () => void
}

function resolveToken(tokenSource: TokenSource | undefined): string {
  if (!tokenSource) return ''
  if (typeof tokenSource === 'function') return tokenSource() || ''
  if (typeof tokenSource === 'object' && 'value' in tokenSource)
    return (tokenSource as { value: string }).value || ''
  return tokenSource || ''
}

function isTerminalStatus(status: string): boolean {
  return status === 'completed' || status === 'failed'
}

/**
 * `token` is kept as an optional argument for source-level backwards compat
 * with call sites that still hand a token getter. WebSocket authentication is
 * now carried by the ss_access cookie attached to the upgrade request, so the
 * token value itself is no longer consulted.
 */
export function useCommandStream({ token }: { token?: TokenSource } = {}): UseCommandStreamApi {
  let activeStream: WebSocket | null = null
  // Set right before any deliberate close (closeStream() itself, or a
  // terminal-status auto-close) so ws.onclose can tell that apart from an
  // unexpected drop (network blip, server restart) that should reconnect
  // instead of leaving the stream — and whatever console is showing it —
  // silently stuck on stale output. Mirrors useWebSocket.ts's manualClose.
  let manualClose = false
  let retryTimer: ReturnType<typeof setTimeout> | null = null
  let retryCount = 0

  function closeStream(): void {
    manualClose = true
    if (retryTimer) { clearTimeout(retryTimer); retryTimer = null }
    if (!activeStream) return
    activeStream.onopen = null
    activeStream.onmessage = null
    activeStream.onerror = null
    activeStream.onclose = null
    activeStream.close()
    activeStream = null
  }

  function createStreamUrl(commandId: string): string {
    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
    return `${protocol}://${window.location.host}/api/v1/ws/commands/stream/${commandId}`
  }

  // Exponential backoff (4s, 8s, 16s, capped at 30s) — same formula and
  // increment-before-delay order as useWebSocket.ts, for a consistent
  // reconnect feel across the app.
  function retryDelay(): number {
    return Math.min(2000 * Math.pow(2, retryCount), 30000)
  }

  function openCommandStream(commandId: string, options: CommandStreamOptions = {}): WebSocket {
    const {
      onInit,
      onChunk,
      onStatus,
      onClose,
      onError,
      closeOnTerminalStatus = false,
      terminalCloseDelayMs = 500,
    } = options

    closeStream()
    manualClose = false

    const ws = new WebSocket(createStreamUrl(commandId))
    activeStream = ws

    const closeIfCurrent = (): void => {
      if (activeStream !== ws) return
      closeStream()
    }

    const scheduleTerminalClose = (status: string): void => {
      if (!closeOnTerminalStatus || !isTerminalStatus(status)) return
      window.setTimeout(closeIfCurrent, terminalCloseDelayMs)
    }

    ws.onopen = (): void => {
      if (activeStream !== ws) return
      retryCount = 0
      // The session cookie attached by the browser to the WebSocket upgrade
      // authenticates the connection; no in-band auth message is needed.
      // resolveToken stays callable for backwards compatibility with older
      // call sites that still pass a token getter — its value is unused now.
      void resolveToken(token)
    }

    ws.onmessage = (event: MessageEvent): void => {
      if (activeStream !== ws) return
      try {
        const parsed = JSON.parse(event.data) as unknown
        if (typeof parsed !== 'object' || parsed === null) return
        const payload = parsed as CommandStreamMessage
        if (payload.type === 'cmd_stream_init') {
          // A reconnect lands here too — the server always answers a fresh
          // subscription with the command's current full status + buffered
          // output (server/internal/ws/endpoints.go), so this one message is
          // what actually "catches up" a console left stale by a dropped
          // connection.
          onInit?.(payload)
          scheduleTerminalClose(payload.status)
        } else if (payload.type === 'cmd_stream') {
          onChunk?.(payload)
        } else if (payload.type === 'cmd_status_update') {
          onStatus?.(payload)
          scheduleTerminalClose(payload.status)
        }
      } catch {
        // Ignore malformed payloads
      }
    }

    ws.onerror = (): void => {
      if (activeStream !== ws) return
      // Deliberately don't touch activeStream/handlers or call close() here —
      // a WebSocket error is always followed by a close event, and onclose
      // below owns all teardown plus the reconnect decision. Clearing
      // activeStream here too would make onclose see wasCurrent=false and
      // skip reconnecting.
      onError?.(new Error('WebSocket error'))
    }

    ws.onclose = (event: CloseEvent): void => {
      const wasCurrent = activeStream === ws
      if (wasCurrent) activeStream = null

      // 1002/1008 (origin/policy rejection) and 4001 (custom auth error) mean
      // retrying is pointless — it'll keep failing the same way. Same
      // carve-out as useWebSocket.ts.
      const nonRetryable = event.code === 1002 || event.code === 1008 || event.code === 4001
      if (manualClose || nonRetryable || !wasCurrent) {
        onClose?.()
        return
      }

      // Unexpected drop: reconnect to the same command instead of surfacing
      // this as "closed" — the caller's callbacks (and whatever console is
      // rendering them) keep working transparently once the new connection's
      // cmd_stream_init message lands.
      retryCount++
      retryTimer = setTimeout(() => {
        openCommandStream(commandId, options)
      }, retryDelay())
    }

    return ws
  }

  function collectCommandOutput(
    commandId: string,
    options: CollectCommandOutputOptions = {}
  ): Promise<string> {
    const { timeoutMs = 20000, onInit, onChunk, onStatus } = options

    return new Promise((resolve, reject) => {
      let output = ''
      let settled = false
      let timeoutId: number | null = null

      const finishResolve = (value: string): void => {
        if (settled) return
        settled = true
        if (timeoutId) window.clearTimeout(timeoutId)
        closeStream()
        resolve(value)
      }

      const finishReject = (reason: Error): void => {
        if (settled) return
        settled = true
        if (timeoutId) window.clearTimeout(timeoutId)
        closeStream()
        reject(reason)
      }

      openCommandStream(commandId, {
        onInit: (payload: CommandStreamInitMsg) => {
          output = payload.output || ''
          onInit?.(payload, output)
          if (payload.status === 'completed') finishResolve(output)
          else if (payload.status === 'failed') finishReject(new Error(output || 'Command failed'))
        },
        onChunk: (payload: CommandStreamChunkMsg) => {
          output += payload.chunk || ''
          onChunk?.(payload, output)
        },
        onStatus: (payload: CommandStatusUpdateMsg) => {
          if (typeof payload.output === 'string') output = payload.output
          onStatus?.(payload, output)
          if (payload.status === 'completed') finishResolve(output)
          else if (payload.status === 'failed') finishReject(new Error(output || 'Command failed'))
        },
        onClose: () => {
          if (!settled)
            finishReject(new Error('Connexion WebSocket fermée avant la fin de la commande'))
        },
        onError: (error: Error) => {
          finishReject(error)
        },
      })

      timeoutId = window.setTimeout(() => {
        finishReject(new Error("Timeout : l'agent n'a pas répondu dans le délai imparti (hôte hors-ligne ou surchargé ?)"))
      }, timeoutMs)
    })
  }

  if (getCurrentInstance()) {
    onUnmounted(() => {
      closeStream()
    })
  }

  return {
    openCommandStream,
    collectCommandOutput,
    closeStream,
  }
}
