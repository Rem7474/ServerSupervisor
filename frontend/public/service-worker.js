const CACHE_NAME = 'serversupervisor-v1'
const RUNTIME_CACHE = 'serversupervisor-runtime'
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
            if (cacheName !== CACHE_NAME && cacheName !== RUNTIME_CACHE) {
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

  // API requests: network-first with cache fallback
  if (url.pathname.startsWith('/api/')) {
    event.respondWith(
      fetch(event.request)
        .then((response) => {
          // Cache successful API responses
          if (response && response.status === 200) {
            const cache = caches.open(RUNTIME_CACHE)
            cache.then((c) => c.put(event.request, response.clone()))
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
        cacheNames.map((cacheName) => caches.delete(cacheName))
      )
    })
  }
})
