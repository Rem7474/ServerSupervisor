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

  function closeStream(): void {
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
      // Close before calling the error callback so the dead socket doesn't
      // linger. closeStream() nullifies all handlers first, so onclose won't
      // fire a second time after this.
      closeStream()
      onError?.(new Error('WebSocket error'))
    }

    ws.onclose = (): void => {
      const wasCurrent = activeStream === ws
      if (wasCurrent) activeStream = null
      onClose?.()
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
