import { i18n } from '../i18n'

interface ErrorLike {
  response?: {
    data?: {
      error?: unknown
      message?: unknown
      i18nKey?: unknown
      params?: Record<string, string>
    }
  }
  message?: unknown
}

// Fallback for endpoints whose backend error site hasn't set an I18nKey yet
// (see server/internal/apperr/catalog.go's Error.I18n) — matches raw,
// mostly-English technical substrings and resolves them through the same
// errors.json catalog as the keyed path, so this branch renders in the active
// UI language too. Only consulted when the response carries no i18nKey;
// shrinks and eventually disappears as the backend error-code migration covers
// more call sites.
const LEGACY_FALLBACK_KEYS: Record<string, string> = {
  'host not found': 'legacy.hostNotFound',
  'unauthorized': 'legacy.unauthorized',
  'forbidden': 'legacy.forbidden',
  'invalid credentials': 'legacy.invalidCredentials',
  'connection refused': 'legacy.connectionRefused',
  'timeout': 'legacy.timeout',
  'network error': 'legacy.networkError',
  'internal server error': 'legacy.internalServerError',
  'not found': 'legacy.notFound',
  'bad request': 'legacy.badRequest',
  'service unavailable': 'legacy.serviceUnavailable',
  'already exists': 'legacy.alreadyExists',
  'invalid token': 'legacy.invalidToken',
  'permission denied': 'legacy.permissionDenied',
}

/**
 * Translates an Axios/native error into a message in the active UI language.
 * Prefers the response's `i18nKey` (resolved against locales/{fr,en}/errors.json
 * in whatever language the UI is currently showing — not the server's own
 * Accept-Language-resolved text, which follows the browser's language and can
 * disagree with a UI language the user switched manually — and which the
 * frontend cannot steer, since Accept-Language is a forbidden header name that
 * JS may not set) and falls back to substring-matching the raw message for
 * endpoints not yet migrated.
 */
export function translateError(error: unknown): string {
  if (!error) return i18n.global.t('errors.unknown')

  const e = (typeof error === 'object' && error !== null ? error : {}) as ErrorLike
  const data = e.response?.data

  const key = data?.i18nKey
  if (typeof key === 'string' && i18n.global.te(`errors.${key}`)) {
    const params = data?.params && typeof data.params === 'object' ? data.params : undefined
    return i18n.global.t(`errors.${key}`, params ?? {})
  }

  const raw = String(data?.error || data?.message || e.message || error)
  const lower = raw.toLowerCase()

  for (const [substring, fallbackKey] of Object.entries(LEGACY_FALLBACK_KEYS)) {
    if (lower.includes(substring)) return i18n.global.t(`errors.${fallbackKey}`)
  }

  return raw.charAt(0).toUpperCase() + raw.slice(1)
}
