import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import '@tabler/core/dist/css/tabler.min.css'
import '@tabler/core/dist/js/tabler.min.js'
import './style.css'

type FatalDetail = {
  title?: string
  message?: string
}

function escapeHtml(input: string): string {
  return input
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

const appRoot = document.getElementById('app')

function renderBootPlaceholder(): void {
  if (!appRoot || appRoot.childElementCount > 0) return
  appRoot.innerHTML = `
    <div class="page">
      <div class="page-wrapper">
        <div class="page-body">
          <div class="container-xl">
            <div class="card">
              <div class="card-body py-5 d-flex align-items-center gap-3">
                <div class="spinner-border text-primary" role="status" aria-hidden="true"></div>
                <div>
                  <div class="fw-semibold">Initialisation de l'application</div>
                  <div class="text-secondary small">Chargement en cours...</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  `
}

function renderFatalFallback(detail: FatalDetail): void {
  if (!appRoot) return
  const title = escapeHtml(detail.title || 'Erreur critique de l interface')
  const message = escapeHtml(detail.message || 'Une erreur inattendue a interrompu le rendu de l application.')
  appRoot.innerHTML = `
    <div class="page">
      <div class="page-wrapper">
        <div class="page-body">
          <div class="container-xl">
            <div class="alert alert-danger" role="alert">
              <div class="d-flex">
                <div class="me-3">
                  <span class="avatar avatar-sm bg-red-lt text-red">!</span>
                </div>
                <div class="flex-fill">
                  <h3 class="alert-title mb-1">${title}</h3>
                  <div class="text-secondary mb-3">${message}</div>
                  <button type="button" class="btn btn-danger" id="fatal-reload-btn">Recharger l'application</button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  `

  const reloadButton = document.getElementById('fatal-reload-btn')
  reloadButton?.addEventListener('click', () => window.location.reload())
}

function toErrorMessage(reason: unknown): string {
  if (reason instanceof Error) {
    return reason.message
  }
  if (typeof reason === 'string') {
    return reason
  }
  return 'Erreur inconnue'
}

renderBootPlaceholder()

window.addEventListener('error', (event: ErrorEvent) => {
  renderFatalFallback({
    title: 'Erreur JavaScript non gérée',
    message: toErrorMessage(event.error ?? event.message),
  })
})

// Errors expected during reconnect/wake: network failures, aborted fetches, WS close races.
// These must NOT destroy the app — just log them.
function isBenignRejection(reason: unknown): boolean {
  const msg = reason instanceof Error
    ? reason.message.toLowerCase()
    : typeof reason === 'string' ? reason.toLowerCase() : ''
  if (!msg) return false
  return (
    msg.includes('aborted') ||
    msg.includes('abort') ||
    msg.includes('failed to fetch') ||
    msg.includes('load failed') ||
    msg.includes('networkerror') ||
    msg.includes('network error') ||
    msg.includes('the internet connection appears to be offline') ||
    msg.includes('websocket') ||
    // axios cancel
    (reason instanceof Error && reason.name === 'CanceledError') ||
    (reason instanceof Error && reason.name === 'AbortError')
  )
}

window.addEventListener('unhandledrejection', (event: PromiseRejectionEvent) => {
  if (isBenignRejection(event.reason)) {
    console.warn('[PWA] Unhandled rejection (réseau/veille, ignoré):', event.reason)
    event.preventDefault()
    return
  }
  renderFatalFallback({
    title: 'Erreur asynchrone non gérée',
    message: toErrorMessage(event.reason),
  })
})

window.addEventListener('ss:fatal-error', (event: Event) => {
  const customEvent = event as CustomEvent<FatalDetail>
  renderFatalFallback(customEvent.detail || {})
})

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.mount('#app')

// Register service worker for PWA support
if ('serviceWorker' in navigator && import.meta.env.PROD) {
  window.addEventListener('load', () => {
    navigator.serviceWorker
      .register('/service-worker.js')
      .then((registration) => {
        console.log('[PWA] Service worker registered successfully:', registration)

        // Check for updates every hour
        setInterval(() => {
          registration.update()
        }, 60 * 60 * 1000)
      })
      .catch((error) => {
        console.error('[PWA] Service worker registration failed:', error)
      })

    // Guard against double-reload (controllerchange can fire multiple times during SW swap)
    let reloadPending = false
    navigator.serviceWorker.addEventListener('controllerchange', () => {
      if (reloadPending) return
      reloadPending = true
      console.log('[PWA] Nouvelle version disponible, rechargement dans 300ms...')
      // Small delay so the new SW finishes activating before the page reloads
      setTimeout(() => window.location.reload(), 300)
    })
  })
}