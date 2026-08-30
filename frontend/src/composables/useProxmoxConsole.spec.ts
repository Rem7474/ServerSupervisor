import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useProxmoxConsole } from './useProxmoxConsole'

// Same shape as useCommandStream.spec.ts's FakeWebSocket, plus a `send` spy
// since this transport is bidirectional.
class FakeWebSocket {
  static instances: FakeWebSocket[] = []
  static OPEN = 1
  url: string
  readyState = 1
  onopen: (() => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onerror: (() => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null
  sent: string[] = []
  closeCalls = 0
  binaryType = 'blob'

  constructor(url: string) {
    this.url = url
    FakeWebSocket.instances.push(this)
  }

  send(data: string): void {
    this.sent.push(data)
  }

  close(): void {
    this.closeCalls++
  }

  open(): void {
    this.onopen?.()
  }

  // Text control frames (console_error).
  message(data: string): void {
    this.onmessage?.({ data } as MessageEvent)
  }

  // Binary PTY output frames — see useProxmoxConsole.ts's onmessage.
  binaryMessage(bytes: Uint8Array): void {
    this.onmessage?.({ data: bytes.buffer } as MessageEvent)
  }

  simulateClose(code = 1006): void {
    this.onclose?.({ code } as CloseEvent)
  }
}

// Minimal fake standing in for an xterm.js Terminal — just enough surface
// (onData/write) for the composable to bind to.
class FakeTerminal {
  written: (string | Uint8Array)[] = []
  private handler: ((data: string) => void) | null = null

  onData(handler: (data: string) => void): { dispose: () => void } {
    this.handler = handler
    return { dispose: () => { this.handler = null } }
  }

  write(data: string | Uint8Array): void {
    this.written.push(data)
  }

  type(data: string): void {
    this.handler?.(data)
  }
}

describe('useProxmoxConsole', () => {
  beforeEach(() => {
    FakeWebSocket.instances = []
    vi.stubGlobal('WebSocket', FakeWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('opens a websocket scoped to the guest id and reports connected on open', () => {
    const { open, status } = useProxmoxConsole()
    const term = new FakeTerminal()

    open('guest-123', term as unknown as import('@xterm/xterm').Terminal)
    expect(status.value).toBe('connecting')

    const socket = FakeWebSocket.instances[0]
    expect(socket.url).toContain('/api/v1/ws/proxmox/console/guest-123')
    expect(socket.binaryType).toBe('arraybuffer')
    socket.open()
    expect(status.value).toBe('connected')
  })

  it('forwards keystrokes typed into the terminal to the socket', () => {
    const { open } = useProxmoxConsole()
    const term = new FakeTerminal()
    open('guest-1', term as unknown as import('@xterm/xterm').Terminal)
    const socket = FakeWebSocket.instances[0]
    socket.open()

    term.type('ls -la\n')

    expect(socket.sent).toEqual(['ls -la\n'])
  })

  it('applies the input transform (sticky Ctrl) to keystrokes before sending them', () => {
    const { open } = useProxmoxConsole()
    const term = new FakeTerminal()
    open('guest-1', term as unknown as import('@xterm/xterm').Terminal, {
      transformInput: (data) => (data === 'c' ? '\u0003' : data),
    })
    const socket = FakeWebSocket.instances[0]
    socket.open()

    term.type('c')
    term.type('x')

    expect(socket.sent).toEqual(['\u0003', 'x'])
  })

  it('sendInput puts touch-bar keys on the wire without echoing them locally', () => {
    const { open, sendInput } = useProxmoxConsole()
    const term = new FakeTerminal()
    open('guest-1', term as unknown as import('@xterm/xterm').Terminal)
    const socket = FakeWebSocket.instances[0]
    socket.open()

    sendInput('\u0003')

    expect(socket.sent).toEqual(['\u0003'])
    expect(term.written).toEqual([]) // the remote PTY decides what it echoes
  })

  it('sendInput is a no-op with no open session', () => {
    const { sendInput } = useProxmoxConsole()

    expect(() => sendInput('\u0003')).not.toThrow()
    expect(FakeWebSocket.instances).toHaveLength(0)
  })

  it('writes raw binary server output straight to the terminal as bytes', () => {
    const { open } = useProxmoxConsole()
    const term = new FakeTerminal()
    open('guest-1', term as unknown as import('@xterm/xterm').Terminal)
    const socket = FakeWebSocket.instances[0]
    socket.open()

    const bytes = new TextEncoder().encode('total 12\r\ndrwxr-xr-x ...\r\n')
    socket.binaryMessage(bytes)

    expect(term.written).toEqual([bytes])
  })

  it('ignores a text frame that is not a recognized control message', () => {
    const { open } = useProxmoxConsole()
    const term = new FakeTerminal()
    open('guest-1', term as unknown as import('@xterm/xterm').Terminal)
    const socket = FakeWebSocket.instances[0]
    socket.open()

    socket.message('not json')

    expect(term.written).toEqual([])
  })

  it('sends a resize control message as JSON, not raw text', () => {
    const { open, resize } = useProxmoxConsole()
    const term = new FakeTerminal()
    open('guest-1', term as unknown as import('@xterm/xterm').Terminal)
    const socket = FakeWebSocket.instances[0]
    socket.open()

    resize(120, 40)

    expect(socket.sent).toEqual([JSON.stringify({ type: 'resize', cols: 120, rows: 40 })])
  })

  it('surfaces a console_error control message without writing it to the terminal', () => {
    const { open, status, errorMessage } = useProxmoxConsole()
    const term = new FakeTerminal()
    open('guest-1', term as unknown as import('@xterm/xterm').Terminal)
    const socket = FakeWebSocket.instances[0]
    socket.open()

    socket.message(JSON.stringify({ type: 'console_error', error: 'admin only' }))

    expect(status.value).toBe('error')
    expect(errorMessage.value).toBe('admin only')
    expect(term.written).toEqual([])
  })

  it('reports error status when the socket errors', () => {
    const { open, status } = useProxmoxConsole()
    const term = new FakeTerminal()
    open('guest-1', term as unknown as import('@xterm/xterm').Terminal)
    const socket = FakeWebSocket.instances[0]
    socket.open()

    socket.onerror?.()

    expect(status.value).toBe('error')
  })

  it('ignores late events from a superseded socket after re-opening', () => {
    const { open, status } = useProxmoxConsole()
    const term = new FakeTerminal()
    open('guest-1', term as unknown as import('@xterm/xterm').Terminal)
    const staleSocket = FakeWebSocket.instances[0]
    staleSocket.open()
    expect(status.value).toBe('connected')

    // Re-opening tears down and replaces the socket — the old one is now stale.
    open('guest-1', term as unknown as import('@xterm/xterm').Terminal)
    const newSocket = FakeWebSocket.instances[1]

    // A late error/close from the superseded socket must not affect status.
    staleSocket.onerror?.()
    expect(status.value).toBe('connecting')
    staleSocket.simulateClose()
    expect(status.value).toBe('connecting')

    newSocket.open()
    expect(status.value).toBe('connected')
  })

  it('does not auto-reconnect after an unexpected close', () => {
    const { open, status } = useProxmoxConsole()
    const term = new FakeTerminal()
    open('guest-1', term as unknown as import('@xterm/xterm').Terminal)
    const socket = FakeWebSocket.instances[0]
    socket.open()

    socket.simulateClose()

    expect(status.value).toBe('disconnected')
    expect(FakeWebSocket.instances.length).toBe(1)
  })

  it('close() marks the session idle and does not reconnect', () => {
    const { open, close, status } = useProxmoxConsole()
    const term = new FakeTerminal()
    open('guest-1', term as unknown as import('@xterm/xterm').Terminal)
    const socket = FakeWebSocket.instances[0]
    socket.open()

    close()
    socket.simulateClose()

    expect(status.value).toBe('idle')
    expect(socket.closeCalls).toBe(1)
  })
})
