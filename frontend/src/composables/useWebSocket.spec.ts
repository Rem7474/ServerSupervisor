import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setLocale } from '../i18n'
import { useWebSocket } from './useWebSocket'

// Controllable stand-in for the browser WebSocket: each test drives
// open/message/close by hand, and every instance is recorded so a test can
// tell whether a reconnect actually opened a new socket.
class FakeWebSocket {
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSING = 2
  static readonly CLOSED = 3
  static instances: FakeWebSocket[] = []

  url: string
  readyState = FakeWebSocket.CONNECTING
  onopen: (() => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onerror: (() => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null
  sent: string[] = []
  closeCalls = 0

  constructor(url: string) {
    this.url = url
    FakeWebSocket.instances.push(this)
  }

  send(data: string): void {
    this.sent.push(data)
  }

  // Mirrors the browser: close() eventually delivers a close event to
  // whatever onclose handler is still attached at that point.
  close(): void {
    this.closeCalls++
    this.readyState = FakeWebSocket.CLOSED
    this.onclose?.({ code: 1005 } as CloseEvent)
  }

  open(): void {
    this.readyState = FakeWebSocket.OPEN
    this.onopen?.()
  }

  message(payload: unknown): void {
    this.onmessage?.({ data: JSON.stringify(payload) } as MessageEvent)
  }

  drop(code = 1006): void {
    this.readyState = FakeWebSocket.CLOSED
    this.onclose?.({ code } as CloseEvent)
  }
}

type Api = ReturnType<typeof useWebSocket<{ type?: string; n?: number }>>

function mountSocket(onMessage = vi.fn(), options = {}): { api: Api; wrapper: VueWrapper; onMessage: typeof onMessage } {
  let api!: Api
  const Host = defineComponent({
    setup() {
      api = useWebSocket('/api/v1/ws/test', onMessage, options)
      return () => h('div')
    },
  })
  const wrapper = mount(Host)
  mounted.push(wrapper)
  return { api, wrapper, onMessage }
}

const mounted: VueWrapper[] = []

const latest = (): FakeWebSocket => FakeWebSocket.instances[FakeWebSocket.instances.length - 1]

describe('useWebSocket', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    FakeWebSocket.instances = []
    vi.stubGlobal('WebSocket', FakeWebSocket)
    localStorage.setItem('role', 'admin')
    setActivePinia(createPinia())
    setLocale('en')
  })

  afterEach(() => {
    // Unmount so no earlier test's socket keeps listening for ss:app-resume.
    mounted.splice(0).forEach((w) => w.unmount())
    vi.useRealTimers()
    vi.unstubAllGlobals()
    localStorage.clear()
    setLocale('fr')
  })

  it('does not connect when the user is not authenticated', () => {
    localStorage.removeItem('role')
    setActivePinia(createPinia())
    mountSocket()
    expect(FakeWebSocket.instances).toHaveLength(0)
  })

  it('reports connected only once the first message arrives', () => {
    const { api, onMessage } = mountSocket()
    expect(latest().url).toMatch(/^ws:\/\/.+\/api\/v1\/ws\/test$/)

    latest().open()
    expect(api.wsStatus.value).toBe('connecting')

    latest().message({ type: 'snapshot', n: 1 })
    expect(api.wsStatus.value).toBe('connected')
    expect(onMessage).toHaveBeenCalledWith({ type: 'snapshot', n: 1 })
  })

  it('stops on auth_error instead of looping on reconnects', () => {
    const { api, onMessage } = mountSocket()
    latest().open()
    latest().message({ type: 'auth_error', error: 'unauthorized' })

    expect(api.wsStatus.value).toBe('error')
    expect(api.wsError.value).toBe('Authentication refused — sign in again')
    expect(onMessage).not.toHaveBeenCalled()

    vi.advanceTimersByTime(120_000)
    expect(FakeWebSocket.instances).toHaveLength(1)
    expect(api.wsStatus.value).toBe('error')
  })

  it.each([1002, 1008])('does not retry after a %i policy close', (code) => {
    const { api } = mountSocket()
    latest().drop(code)

    expect(api.wsStatus.value).toBe('error')
    expect(api.wsError.value).toBe('Connection refused by the server — check the BASE_URL configuration')
    vi.advanceTimersByTime(120_000)
    expect(FakeWebSocket.instances).toHaveLength(1)
  })

  it('does not retry after a 4001 session-expired close', () => {
    const { api } = mountSocket()
    latest().drop(4001)

    expect(api.wsStatus.value).toBe('error')
    expect(api.wsError.value).toBe('Session expired — reload the page')
    vi.advanceTimersByTime(120_000)
    expect(FakeWebSocket.instances).toHaveLength(1)
  })

  it('retries an abnormal close with a growing delay capped at 30s', () => {
    const { api } = mountSocket()

    latest().drop()
    expect(api.wsStatus.value).toBe('reconnecting')
    expect(api.wsError.value).toBe('Could not connect — check that the server is reachable and that BASE_URL is configured correctly')

    // retryCount is incremented before the delay is computed, so the first
    // retry waits 4s (same as useCommandStream's identical backoff).
    vi.advanceTimersByTime(3999)
    expect(FakeWebSocket.instances).toHaveLength(1)
    vi.advanceTimersByTime(1)
    expect(FakeWebSocket.instances).toHaveLength(2)

    latest().drop()
    expect(api.wsError.value).toBe('')
    vi.advanceTimersByTime(8000)
    expect(FakeWebSocket.instances).toHaveLength(3)

    for (let i = 0; i < 5; i++) {
      latest().drop()
      vi.advanceTimersByTime(30_000)
    }
    expect(FakeWebSocket.instances).toHaveLength(8)
    expect(api.retryCount.value).toBe(7)
  })

  it('flags stale data after a successful reconnect and clears the flag after 3s', () => {
    const { api } = mountSocket()
    latest().open()
    latest().message({ n: 1 })

    latest().drop()
    vi.advanceTimersByTime(4000)
    latest().open()
    expect(api.dataStaleAlert.value).toBe(true)

    latest().message({ n: 2 })
    expect(api.wsStatus.value).toBe('connected')
    expect(api.retryCount.value).toBe(0)

    vi.advanceTimersByTime(3000)
    expect(api.dataStaleAlert.value).toBe(false)
  })

  it('delivers the first message immediately and debounces the following ones', () => {
    const { onMessage } = mountSocket(vi.fn(), { debounceMs: 500 })
    latest().open()

    latest().message({ n: 1 })
    expect(onMessage).toHaveBeenCalledTimes(1)

    latest().message({ n: 2 })
    latest().message({ n: 3 })
    expect(onMessage).toHaveBeenCalledTimes(1)

    vi.advanceTimersByTime(500)
    expect(onMessage).toHaveBeenCalledTimes(2)
    expect(onMessage).toHaveBeenLastCalledWith({ n: 3 })
  })

  it('ignores malformed payloads', () => {
    const { api, onMessage } = mountSocket()
    latest().open()
    latest().onmessage?.({ data: '{not json' } as MessageEvent)

    expect(onMessage).not.toHaveBeenCalled()
    expect(api.wsStatus.value).toBe('connecting')
  })

  it('reconnects on app resume after an auth error, but not while healthy', () => {
    const { api } = mountSocket()
    latest().open()
    latest().message({ n: 1 })

    window.dispatchEvent(new CustomEvent('ss:app-resume'))
    expect(FakeWebSocket.instances).toHaveLength(1)

    latest().message({ type: 'auth_error' })
    expect(api.wsStatus.value).toBe('error')

    // Resume events are throttled to one reconnect per 1.5s.
    vi.advanceTimersByTime(1500)
    window.dispatchEvent(new CustomEvent('ss:app-resume'))
    expect(FakeWebSocket.instances).toHaveLength(2)
  })

  it('sends only while open', () => {
    const { api } = mountSocket()
    expect(api.send({ a: 1 })).toBe(false)

    latest().open()
    expect(api.send({ a: 1 })).toBe(true)
    expect(api.send('raw', { stringify: false })).toBe(true)
    expect(latest().sent).toEqual(['{"a":1}', 'raw'])
  })

  it('closes the socket and cancels a pending retry on unmount', () => {
    const { api, wrapper } = mountSocket()
    latest().drop()
    wrapper.unmount()
    mounted.splice(mounted.indexOf(wrapper), 1)

    expect(api.wsStatus.value).toBe('disconnected')
    vi.advanceTimersByTime(60_000)
    expect(FakeWebSocket.instances).toHaveLength(1)
  })
})
