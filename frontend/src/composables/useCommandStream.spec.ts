import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useCommandStream } from './useCommandStream'

// Minimal controllable fake standing in for the browser WebSocket — lets each
// test drive open/message/close/error deterministically instead of depending
// on a real socket. Every instance is recorded in `instances` so a test can
// grab "the most recently constructed socket" to assert a reconnect actually
// opened a fresh one.
class FakeWebSocket {
  static instances: FakeWebSocket[] = []
  url: string
  onopen: (() => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onerror: (() => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null
  closeCalls = 0

  constructor(url: string) {
    this.url = url
    FakeWebSocket.instances.push(this)
  }

  close(): void {
    this.closeCalls++
  }

  open(): void {
    this.onopen?.()
  }

  message(payload: unknown): void {
    this.onmessage?.({ data: JSON.stringify(payload) } as MessageEvent)
  }

  // Simulates the browser delivering a close event — including one that
  // arrives after the caller already invoked close() itself, which is the
  // normal sequence for both a manual close and a genuine network drop.
  simulateClose(code = 1006): void {
    this.onclose?.({ code } as CloseEvent)
  }
}

describe('useCommandStream — reconnect on unexpected close', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    FakeWebSocket.instances = []
    vi.stubGlobal('WebSocket', FakeWebSocket)
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('reopens the same command stream after an unexpected close, and the new connection catches the console up', () => {
    const onInit = vi.fn()
    const onClose = vi.fn()
    const { openCommandStream } = useCommandStream()

    openCommandStream('cmd-1', { onInit, onClose })
    expect(FakeWebSocket.instances).toHaveLength(1)
    const first = FakeWebSocket.instances[0]
    first.open()
    first.message({ type: 'cmd_stream_init', status: 'running', output: 'partial output' })
    expect(onInit).toHaveBeenCalledTimes(1)

    // Network blip: the socket drops without anyone calling closeStream().
    first.simulateClose(1006)

    // onClose must NOT fire for a drop we're about to recover from — the
    // console should never look "closed", just briefly behind.
    expect(onClose).not.toHaveBeenCalled()

    // Backoff for the first retry elapses (retryCount is incremented before
    // computing the delay, so the first retry is 2000 * 2^1 = 4s — same
    // off-by-one as useWebSocket.ts's identical retryDelay()).
    vi.advanceTimersByTime(4000)
    expect(FakeWebSocket.instances).toHaveLength(2)

    // The server always answers a fresh subscription with the command's full
    // current state — this is what actually catches the console up.
    const second = FakeWebSocket.instances[1]
    expect(second.url).toBe(first.url)
    second.open()
    second.message({ type: 'cmd_stream_init', status: 'completed', output: 'partial output + the rest' })
    expect(onInit).toHaveBeenCalledTimes(2)
    expect(onInit).toHaveBeenLastCalledWith(expect.objectContaining({ output: 'partial output + the rest' }))
  })

  it('does not reconnect once the caller closes the stream deliberately', () => {
    const onClose = vi.fn()
    const { openCommandStream, closeStream } = useCommandStream()

    openCommandStream('cmd-2', { onClose })
    const first = FakeWebSocket.instances[0]
    first.open()

    closeStream()
    expect(first.closeCalls).toBe(1)

    // The browser still delivers the close event asynchronously after we
    // called close() ourselves — must not be mistaken for an unexpected drop.
    first.simulateClose(1000)
    vi.advanceTimersByTime(30000)

    expect(FakeWebSocket.instances).toHaveLength(1) // no reconnect attempt
  })

  it('does not reconnect on a non-retryable close code (e.g. policy/auth rejection)', () => {
    const onClose = vi.fn()
    const { openCommandStream } = useCommandStream()

    openCommandStream('cmd-3', { onClose })
    const first = FakeWebSocket.instances[0]
    first.open()
    first.simulateClose(1008)

    vi.advanceTimersByTime(30000)

    expect(FakeWebSocket.instances).toHaveLength(1)
    expect(onClose).toHaveBeenCalledTimes(1)
  })
})
