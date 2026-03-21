import { getCurrentInstance, onUnmounted } from 'vue'

function resolveToken(tokenSource) {
  if (typeof tokenSource === 'function') return tokenSource() || ''
  if (tokenSource && typeof tokenSource === 'object' && 'value' in tokenSource) return tokenSource.value || ''
  return tokenSource || ''
}

function isTerminalStatus(status) {
  return status === 'completed' || status === 'failed'
}

/**
 * @param {{ token: string | { value: string } | (() => string) }} options
 */
export function useCommandStream({ token }) {
  let activeStream = null

  function closeStream() {
    if (!activeStream) return
    activeStream.onopen = null
    activeStream.onmessage = null
    activeStream.onerror = null
    activeStream.onclose = null
    activeStream.close()
    activeStream = null
  }

  function createStreamUrl(commandId) {
    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
    return `${protocol}://${window.location.host}/api/v1/ws/commands/stream/${commandId}`
  }

  function openCommandStream(commandId, {
    onInit,
    onChunk,
    onStatus,
    onClose,
    onError,
    closeOnTerminalStatus = false,
    terminalCloseDelayMs = 500,
  } = {}) {
    closeStream()

    const ws = new WebSocket(createStreamUrl(commandId))
    activeStream = ws

    const closeIfCurrent = () => {
      if (activeStream !== ws) return
      closeStream()
    }

    const scheduleTerminalClose = (status) => {
      if (!closeOnTerminalStatus || !isTerminalStatus(status)) return
      window.setTimeout(closeIfCurrent, terminalCloseDelayMs)
    }

    ws.onopen = () => {
      if (activeStream !== ws) return
      ws.send(JSON.stringify({ type: 'auth', token: resolveToken(token) }))
    }

    ws.onmessage = (event) => {
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

    ws.onerror = () => {
      if (activeStream !== ws) return
      // Close before calling the error callback so the dead socket doesn't
      // linger. closeStream() nullifies all handlers first, so onclose won't
      // fire a second time after this.
      closeStream()
      onError?.(new Error('WebSocket error'))
    }

    ws.onclose = () => {
      const wasCurrent = activeStream === ws
      if (wasCurrent) activeStream = null
      onClose?.()
    }

    return ws
  }

  function collectCommandOutput(commandId, {
    timeoutMs = 20000,
    onInit,
    onChunk,
    onStatus,
  } = {}) {
    return new Promise((resolve, reject) => {
      let output = ''
      let settled = false
      let timeoutId = null

      const finishResolve = (value) => {
        if (settled) return
        settled = true
        if (timeoutId) window.clearTimeout(timeoutId)
        closeStream()
        resolve(value)
      }

      const finishReject = (reason) => {
        if (settled) return
        settled = true
        if (timeoutId) window.clearTimeout(timeoutId)
        closeStream()
        reject(reason)
      }

      openCommandStream(commandId, {
        onInit: (payload) => {
          output = payload.output || ''
          onInit?.(payload, output)
          if (payload.status === 'completed') finishResolve(output)
          else if (payload.status === 'failed') finishReject(new Error(output || 'Command failed'))
        },
        onChunk: (payload) => {
          output += payload.chunk || ''
          onChunk?.(payload, output)
        },
        onStatus: (payload) => {
          if (typeof payload.output === 'string') output = payload.output
          onStatus?.(payload, output)
          if (payload.status === 'completed') finishResolve(output)
          else if (payload.status === 'failed') finishReject(new Error(output || 'Command failed'))
        },
        onClose: () => {
          if (!settled) finishReject(new Error('Connexion WebSocket fermée avant la fin de la commande'))
        },
        onError: (error) => {
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