const SW_VERSION = '2026-04-12-v1'
const STATIC_CACHE_PREFIX = 'serversupervisor-static'
const RUNTIME_CACHE_PREFIX = 'serversupervisor-runtime'
const CACHE_NAME = `${STATIC_CACHE_PREFIX}-${SW_VERSION}`
const RUNTIME_CACHE = `${RUNTIME_CACHE_PREFIX}-${SW_VERSION}`
const CACHE_PREFIXES = [STATIC_CACHE_PREFIX, RUNTIME_CACHE_PREFIX]
const ASSETS_TO_CACHE = [
  '/',
  '/index.html',
  '/manifest.json',
]

// Installation: cache static assets
self.addEventListener('install', (event) => {
  console.log('[ServiceWorker] Installing service worker...')
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then((cache) => {
        console.log('[ServiceWorker] Caching essential assets')
        return cache.addAll(ASSETS_TO_CACHE)
      })
      .then(() => self.skipWaiting())
  )
})

// Activation: clean old caches
self.addEventListener('activate', (event) => {
  console.log('[ServiceWorker] Activating service worker...')
  event.waitUntil(
    caches.keys()
      .then((cacheNames) => {
        return Promise.all(
          cacheNames.map((cacheName) => {
            const isServerSupervisorCache = CACHE_PREFIXES.some((prefix) => cacheName.startsWith(prefix))
            if (isServerSupervisorCache && cacheName !== CACHE_NAME && cacheName !== RUNTIME_CACHE) {
              console.log('[ServiceWorker] Deleting old cache:', cacheName)
              return caches.delete(cacheName)
            }
          })
        )
      })
      .then(() => self.clients.claim())
  )
})

// Fetch: implement caching strategy
self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url)

  // Skip non-GET requests
  if (event.request.method !== 'GET') {
    return
  }

  // Skip external APIs and websockets
  if (url.origin !== self.location.origin || url.protocol === 'ws:' || url.protocol === 'wss:') {
    return
  }

  // HTML navigations: network-first to avoid stale app shell after tab resume/deploy.
  if (event.request.mode === 'navigate' || event.request.destination === 'document') {
    event.respondWith(
      fetch(event.request)
        .then((response) => {
          if (response && response.status === 200) {
            const clone = response.clone()
            caches.open(CACHE_NAME).then((cache) => cache.put('/index.html', clone))
          }
          return response
        })
        .catch(async () => {
          const cachedPage = await caches.match(event.request)
          if (cachedPage) {
            return cachedPage
          }
          const cachedShell = await caches.match('/index.html')
          if (cachedShell) {
            return cachedShell
          }
          return new Response(
            'Offline - application shell unavailable',
            {
              status: 503,
              statusText: 'Service Unavailable',
              headers: { 'Content-Type': 'text/plain' },
            }
          )
        })
    )
    return
  }

  // API requests: network-first with cache fallback
  if (url.pathname.startsWith('/api/')) {
    event.respondWith(
      fetch(event.request)
        .then((response) => {
          // Cache successful API responses
          if (response && response.status === 200) {
            const responseClone = response.clone()
            caches.open(RUNTIME_CACHE).then((c) => c.put(event.request, responseClone))
          }
          return response
        })
        .catch(() => {
          // Return cached response if network fails
          return caches.match(event.request)
            .then((cachedResponse) => {
              if (cachedResponse) {
                console.log('[ServiceWorker] Serving cached API response:', url.pathname)
                return cachedResponse
              }
              // If no cache and no network, return offline page
              return new Response(
                JSON.stringify({ error: 'Offline - no cached data available' }),
                { 
                  status: 503,
                  statusText: 'Service Unavailable',
                  headers: { 'Content-Type': 'application/json' }
                }
              )
            })
        })
    )
    return
  }

  // Static assets: cache-first strategy
  event.respondWith(
    caches.match(event.request)
      .then((cachedResponse) => {
        if (cachedResponse) {
          // Cache hit - return cached version and update in background
          fetch(event.request)
            .then((response) => {
              if (response && response.status === 200) {
                caches.open(CACHE_NAME)
                  .then((cache) => cache.put(event.request, response))
              }
            })
            .catch(() => {}) // Ignore network errors in background
          return cachedResponse
        }
        // No cache - fetch from network
        return fetch(event.request)
          .then((response) => {
            // Cache successful responses
            if (response && response.status === 200 && response.type === 'basic') {
              const responseToCache = response.clone()
              caches.open(CACHE_NAME)
                .then((cache) => cache.put(event.request, responseToCache))
            }
            return response
          })
          .catch(() => {
            // Network failed and no cache available
            // Return offline page for HTML requests
            if (event.request.destination === 'document') {
              return caches.match('/index.html')
            }
            return new Response(
              'Offline - resource not available',
              {
                status: 503,
                statusText: 'Service Unavailable',
                headers: { 'Content-Type': 'text/plain' }
              }
            )
          })
      })
  )
})

// Handle incoming Web Push notifications (background / app closed on mobile)
self.addEventListener('push', (event) => {
  let data = {}
  if (event.data) {
    try {
      data = event.data.json()
    } catch {
      data.body = event.data.text()
    }
  }
  const title = data.title || 'ServerSupervisor'
  const options = {
    body: data.body || 'Nouvelle alerte détectée',
    icon: '/favicon.ico',
    badge: '/favicon.ico',
    tag: data.tag || 'ss-alert',
    data: { url: data.url || '/alerts?tab=incidents' },
    requireInteraction: false,
    renotify: true,
  }
  event.waitUntil(
    self.registration.showNotification(title, options)
  )
})

// Open / focus the app when a push notification is clicked
self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const targetUrl = (event.notification.data && event.notification.data.url) || '/alerts?tab=incidents'
  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clientList) => {
      // Focus first open tab if available
      for (const client of clientList) {
        if (client.url.includes(self.location.origin) && 'focus' in client) {
          return client.focus()
        }
      }
      // No open tab — open the target URL in a new window
      if (clients.openWindow) {
        return clients.openWindow(targetUrl)
      }
    })
  )
})

// Background sync for failed requests (optional future feature)
self.addEventListener('sync', (event) => {
  if (event.tag === 'sync-api-requests') {
    event.waitUntil(
      // Retry failed API requests when connection restores
      console.log('[ServiceWorker] Background sync triggered')
    )
  }
})

// Message handling for client communication
self.addEventListener('message', (event) => {
  if (event.data && event.data.type === 'SKIP_WAITING') {
    self.skipWaiting()
  }
  if (event.data && event.data.type === 'CLEAR_CACHE') {
    caches.keys().then((cacheNames) => {
      return Promise.all(
        cacheNames
          .filter((cacheName) => CACHE_PREFIXES.some((prefix) => cacheName.startsWith(prefix)))
          .map((cacheName) => caches.delete(cacheName))
      )
    })
  }
})
