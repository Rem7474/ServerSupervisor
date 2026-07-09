//
// frontend/public/service-worker.js is plain JS living outside `src/`, only
// registered in production builds (see main.ts, gated by import.meta.env.PROD).
// It was previously invisible to typecheck/lint/Vitest, so nothing in CI ever
// exercised its fetch-handling logic. This spec loads the *real* file (not a
// reimplementation) into a minimal fake ServiceWorkerGlobalScope and drives its
// `fetch` listener directly, to lock in current behavior and guard the failure
// mode reported in issue #199: a transient network failure gets repackaged as a
// generic, indistinguishable "503 Service Unavailable" instead of surfacing the
// real connection error.
import { describe, it, expect, beforeEach, vi } from 'vitest'
import SW_SOURCE from '../../public/service-worker.js?raw'

type Listener = (event: FakeFetchEvent) => void

interface FakeRequest {
  url: string
  method?: string
  mode?: string
  destination?: string
}

interface FakeFetchEvent {
  request: Required<FakeRequest>
  respondWith: (p: Promise<Response>) => void
  waitUntil: (p: Promise<unknown>) => void
}

function createFakeCacheStorage() {
  const named = new Map<string, Map<string, Response>>()
  function storeFor(name: string) {
    if (!named.has(name)) named.set(name, new Map())
    return named.get(name)!
  }
  function keyOf(req: unknown): string {
    return typeof req === 'string' ? req : (req as { url: string }).url
  }
  return {
    open: vi.fn(async (name: string) => {
      const store = storeFor(name)
      return {
        match: vi.fn(async (req: unknown) => store.get(keyOf(req))),
        put: vi.fn(async (req: unknown, res: Response) => {
          store.set(keyOf(req), res)
        }),
        addAll: vi.fn(async (urls: string[]) => {
          for (const u of urls) store.set(u, new Response('cached'))
        }),
        delete: vi.fn(async (req: unknown) => store.delete(keyOf(req))),
      }
    }),
    match: vi.fn(async (req: unknown) => {
      const key = keyOf(req)
      for (const store of named.values()) {
        if (store.has(key)) return store.get(key)
      }
      return undefined
    }),
    keys: vi.fn(async () => Array.from(named.keys())),
    delete: vi.fn(async (name: string) => named.delete(name)),
  }
}

function loadServiceWorker() {
  const listeners = new Map<string, Listener[]>()
  const fakeSelf = {
    addEventListener: (type: string, handler: Listener) => {
      if (!listeners.has(type)) listeners.set(type, [])
      listeners.get(type)!.push(handler)
    },
    skipWaiting: vi.fn(),
    clients: {
      claim: vi.fn(),
      matchAll: vi.fn(async () => []),
      openWindow: vi.fn(),
    },
    registration: { showNotification: vi.fn() },
    location: { origin: 'http://localhost:3000' },
  }
  const fakeCaches = createFakeCacheStorage()
  const fakeFetch = vi.fn()

  // Executes the real service-worker.js source in this sandboxed scope: `self`,
  // `caches` and `fetch` are bare identifiers in the script (it *is* the global
  // scope in a real browser), so they're injected as function parameters here.
  const run = new Function('self', 'caches', 'fetch', SW_SOURCE)
  run(fakeSelf, fakeCaches, fakeFetch)

  function dispatch(type: string, event: FakeFetchEvent) {
    for (const handler of listeners.get(type) || []) handler(event)
  }

  return { self: fakeSelf, caches: fakeCaches, fetch: fakeFetch, dispatch }
}

function makeFetchEvent(request: FakeRequest) {
  let respondWithPromise: Promise<Response> | undefined
  const event: FakeFetchEvent = {
    request: { method: 'GET', mode: '', destination: '', ...request },
    respondWith: (p) => {
      respondWithPromise = p
    },
    waitUntil: () => {},
  }
  return { event, getResponse: () => respondWithPromise }
}

async function flushMicrotasks() {
  await new Promise((resolve) => setTimeout(resolve, 0))
}

describe('service-worker.js fetch handling', () => {
  let sw: ReturnType<typeof loadServiceWorker>

  beforeEach(() => {
    sw = loadServiceWorker()
  })

  it('ignores non-GET requests, letting the browser handle them natively', () => {
    const { event, getResponse } = makeFetchEvent({
      url: 'http://localhost:3000/api/v1/alert-rules',
      method: 'POST',
    })
    sw.dispatch('fetch', event)

    expect(getResponse()).toBeUndefined()
    expect(sw.fetch).not.toHaveBeenCalled()
  })

  it('ignores cross-origin and websocket requests', () => {
    const external = makeFetchEvent({ url: 'https://example.com/api/data' })
    sw.dispatch('fetch', external.event)
    expect(external.getResponse()).toBeUndefined()

    const ws = makeFetchEvent({ url: 'ws://localhost:3000/api/v1/ws/notifications' })
    sw.dispatch('fetch', ws.event)
    expect(ws.getResponse()).toBeUndefined()
  })

  it('passes through and caches a successful GET /api/ response', async () => {
    sw.fetch.mockResolvedValueOnce(new Response('[{"id":1}]', { status: 200 }))

    const { event, getResponse } = makeFetchEvent({ url: 'http://localhost:3000/api/v1/alert-rules' })
    sw.dispatch('fetch', event)
    const response = await getResponse()

    expect(response!.status).toBe(200)
    await flushMicrotasks()
    const cached = await sw.caches.match({ url: 'http://localhost:3000/api/v1/alert-rules' })
    expect(cached).toBeDefined()
  })

  it('does not cache a non-200 GET /api/ response', async () => {
    sw.fetch.mockResolvedValueOnce(new Response('not found', { status: 404 }))

    const { event, getResponse } = makeFetchEvent({ url: 'http://localhost:3000/api/v1/alert-rules/999' })
    sw.dispatch('fetch', event)
    await getResponse()
    await flushMicrotasks()

    const cached = await sw.caches.match({ url: 'http://localhost:3000/api/v1/alert-rules/999' })
    expect(cached).toBeUndefined()
  })

  it('regression (#199): masks a real network failure on GET /api/ as a generic 503 when nothing is cached', async () => {
    sw.fetch.mockRejectedValueOnce(new TypeError('Failed to fetch'))

    const { event, getResponse } = makeFetchEvent({ url: 'http://localhost:3000/api/v1/alert-rules' })
    sw.dispatch('fetch', event)
    const response = await getResponse()

    // This is the exact symptom from issue #199: a transient network failure
    // (backend unreachable, e.g. mid-restart) is repackaged as a generic 503
    // that is indistinguishable from a real server error, instead of surfacing
    // the underlying connection failure.
    expect(response!.status).toBe(503)
    expect(await response!.json()).toEqual({ error: 'Offline - no cached data available' })
  })

  it('serves a cached GET /api/ response instead of the 503 fallback when available', async () => {
    const cache = await sw.caches.open('serversupervisor-runtime-test')
    await cache.put('http://localhost:3000/api/v1/alert-rules', new Response('["cached"]', { status: 200 }))

    sw.fetch.mockRejectedValueOnce(new TypeError('Failed to fetch'))
    const { event, getResponse } = makeFetchEvent({ url: 'http://localhost:3000/api/v1/alert-rules' })
    sw.dispatch('fetch', event)
    const response = await getResponse()

    expect(response!.status).toBe(200)
    expect(await response!.text()).toBe('["cached"]')
  })

  it('regression (#199): masks a failed page navigation (e.g. opening /alerts while the backend is down) as a generic 503', async () => {
    sw.fetch.mockRejectedValueOnce(new TypeError('Failed to fetch'))

    const { event, getResponse } = makeFetchEvent({
      url: 'http://localhost:3000/alerts',
      mode: 'navigate',
    })
    sw.dispatch('fetch', event)
    const response = await getResponse()

    expect(response!.status).toBe(503)
    expect(await response!.text()).toBe('Offline - application shell unavailable')
  })

  it('serves the cached app shell instead of the 503 fallback on a failed navigation when index.html is cached', async () => {
    const cache = await sw.caches.open('serversupervisor-static-test')
    await cache.put('/index.html', new Response('<html>shell</html>', { status: 200 }))

    sw.fetch.mockRejectedValueOnce(new TypeError('Failed to fetch'))
    const { event, getResponse } = makeFetchEvent({ url: 'http://localhost:3000/alerts', mode: 'navigate' })
    sw.dispatch('fetch', event)
    const response = await getResponse()

    expect(response!.status).toBe(200)
    expect(await response!.text()).toBe('<html>shell</html>')
  })
})
