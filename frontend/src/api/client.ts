import axios, { AxiosInstance, AxiosError, InternalAxiosRequestConfig } from 'axios'
import { useAuthStore } from '../stores/auth'
import { emitHttpError } from '../utils/httpErrorBus'

export type JsonObject = Record<string, unknown>

type ApiErrorLike = {
  response?: {
    data?: {
      error?: unknown
      message?: unknown
    }
  }
  message?: unknown
  name?: unknown
}

function asApiErrorLike(error: unknown): ApiErrorLike {
  return typeof error === 'object' && error !== null ? (error as ApiErrorLike) : {}
}

/**
 * Shared axios instance. The session is carried by an httpOnly cookie
 * (ss_access) set by the server on /api/auth/login and rotated on
 * /api/auth/refresh. Every per-domain module imports this instance.
 */
export const api: AxiosInstance = axios.create({
  baseURL: '/api',
  timeout: 30000,
  withCredentials: true,
})

const CSRF_COOKIE = 'ss_csrf'

function readCSRFFromCookie(): string {
  const prefix = `${CSRF_COOKIE}=`
  const parts = document.cookie ? document.cookie.split('; ') : []
  for (const part of parts) {
    if (part.startsWith(prefix)) {
      try {
        return decodeURIComponent(part.slice(prefix.length))
      } catch {
        return part.slice(prefix.length)
      }
    }
  }
  return ''
}

function isStateChangingMethod(method?: string): boolean {
  if (!method) return false
  const m = method.toUpperCase()
  return m === 'POST' || m === 'PUT' || m === 'PATCH' || m === 'DELETE'
}

let redirectingToLogin = false

function hardRedirectToLogin(): void {
  if (redirectingToLogin) return
  redirectingToLogin = true

  const now = Date.now()
  const target = `/login?reauth=${now}`

  if (window.location.pathname === '/login') {
    window.location.replace(target)
    setTimeout(() => window.location.reload(), 50)
    return
  }

  window.location.replace(target)
}

/**
 * Normalize API/Network error objects into a user-facing message.
 */
export function getApiErrorMessage(
  error: unknown,
  fallback: string = 'Une erreur est survenue'
): string {
  const parsed = asApiErrorLike(error)
  const message = parsed.response?.data?.error || parsed.response?.data?.message || parsed.message

  return message ? String(message) : fallback
}

/**
 * The CSRF token is mirrored in a readable cookie (ss_csrf) and must be echoed
 * in the X-CSRF-Token header on every state-changing request (double-submit
 * pattern). X-Requested-With remains as defense-in-depth.
 */
api.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const headers = config.headers as Record<string, string>
  if (isStateChangingMethod(config.method)) {
    const csrf = readCSRFFromCookie()
    if (csrf) headers['X-CSRF-Token'] = csrf
  }
  headers['X-Requested-With'] = 'XMLHttpRequest'
  return config
})

/**
 * Handle 401 (unauthorized) by logging out and redirecting to login.
 * Silently ignore aborted requests (AbortController / component unmount).
 */
api.interceptors.response.use(
  (response) => response,
  (error: AxiosError) => {
    if (axios.isCancel(error)) {
      return Promise.reject(error)
    }
    const status = error.response?.status ?? null
    if (status === 401) {
      const auth = useAuthStore()
      auth.logout()
      hardRedirectToLogin()
    } else if (status === 403) {
      emitHttpError(403, "Vous n'avez pas les droits nécessaires pour cette action")
    } else if (status && status >= 500) {
      emitHttpError(status, 'Le serveur a rencontré une erreur. Réessayez dans quelques instants.')
    } else if (status === null) {
      emitHttpError(null, 'Erreur réseau: impossible de joindre le serveur')
    }
    return Promise.reject(error)
  }
)

/**
 * Check if an axios error was caused by intentional cancellation.
 */
export function isApiAbort(error: unknown): boolean {
  const parsed = asApiErrorLike(error)
  return (
    axios.isCancel(error) ||
    parsed.name === 'CanceledError' ||
    parsed.name === 'AbortError'
  )
}
