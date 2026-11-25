// Test doubles for noVNC's RFB client and the WebSocket handed to it, shared
// by useProxmoxVnc.spec.ts and ProxmoxVncConsole.spec.ts. Both specs
// vi.mock('@novnc/novnc') with FakeRFB and vi.stubGlobal('WebSocket',
// FakeVncSocket), then drive the session by dispatching the events noVNC and
// the socket would emit.

export class FakeVncSocket extends EventTarget {
  static instances: FakeVncSocket[] = []
  url: string
  binaryType = 'blob'

  constructor(url: string) {
    super()
    this.url = url
    FakeVncSocket.instances.push(this)
  }

  /** The server ending the session, as ws.ProxmoxVNC does on an error. */
  serverClose(code: number, reason = ''): void {
    const event = new Event('close') as Event & { code: number; reason: string }
    event.code = code
    event.reason = reason
    this.dispatchEvent(event)
  }
}

export class FakeRFB extends EventTarget {
  static instances: FakeRFB[] = []
  target: HTMLElement
  channel: unknown
  options: unknown
  scaleViewport = false
  resizeSession = true
  disconnectCalls = 0
  ctrlAltDelCalls = 0
  focusCalls = 0

  constructor(target: HTMLElement, channel: unknown, options?: unknown) {
    super()
    this.target = target
    this.channel = channel
    this.options = options
    FakeRFB.instances.push(this)
  }

  disconnect(): void {
    this.disconnectCalls++
  }

  sendCtrlAltDel(): void {
    this.ctrlAltDelCalls++
  }

  focus(): void {
    this.focusCalls++
  }

  emit(type: string, detail?: unknown): void {
    this.dispatchEvent(new CustomEvent(type, { detail }))
  }
}

export function resetFakeNoVnc(): void {
  FakeRFB.instances = []
  FakeVncSocket.instances = []
}
