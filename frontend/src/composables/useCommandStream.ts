import { getCurrentInstance, onUnmounted } from 'vue'

type TokenSource = string | { value: string } | (() => string)

interface CommandStreamOptions {
  onInit?: (payload: any) => void
  onChunk?: (payload: any) => void
  onStatus?: (payload: any) => void
  onClose?: () => void
  onError?: (error: Error) => void
  closeOnTerminalStatus?: boolean
  terminalCloseDelayMs?: number
}

interface CollectCommandOutputOptions {
  timeoutMs?: number
  onInit?: (payload: any, output: string) => void
  onChunk?: (payload: any, output: string) => void
  onStatus?: (payload: any, output: string) => void
}

interface UseCommandStreamApi {
  openCommandStream: (commandId: string, options?: CommandStreamOptions) => WebSocket
  collectCommandOutput: (commandId: string, options?: CollectCommandOutputOptions) => Promise<string>
  closeStream: () => void
}

function resolveToken(tokenSource: TokenSource): string {
  if (typeof tokenSource === 'function') return tokenSource() || ''
  if (tokenSource && typeof tokenSource === 'object' && 'value' in tokenSource)
    return (tokenSource as { value: string }).value || ''
  return tokenSource || ''
}

function isTerminalStatus(status: string): boolean {
  return status === 'completed' || status === 'failed'
}

export function useCommandStream({ token }: { token: TokenSource }): UseCommandStreamApi {
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
      ws.send(JSON.stringify({ type: 'auth', token: resolveToken(token) }))
    }

    ws.onmessage = (event: MessageEvent): void => {
      if (activeStream !== ws) return
      try {
        const payload = JSON.parse(event.data)
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
      let timeoutId: ReturnType<typeof setTimeout> | null = null

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
        onInit: (payload: any) => {
          output = payload.output || ''
          onInit?.(payload, output)
          if (payload.status === 'completed') finishResolve(output)
          else if (payload.status === 'failed') finishReject(new Error(output || 'Command failed'))
        },
        onChunk: (payload: any) => {
          output += payload.chunk || ''
          onChunk?.(payload, output)
        },
        onStatus: (payload: any) => {
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
        finishReject(new Error('Timeout'))
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
