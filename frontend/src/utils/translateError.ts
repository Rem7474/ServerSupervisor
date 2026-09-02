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
// mostly-English technical substrings and renders a French phrase. Only
// consulted when the response carries no i18nKey; shrinks and eventually
// disappears as the backend error-code migration (see the i18n rollout plan)
// covers more call sites. Its French-only bias is a known, temporary
// limitation of this fallback path, not the primary (keyed) one below.
const LEGACY_FALLBACK_MESSAGES: Record<string, string> = {
  'host not found': 'Hôte introuvable',
  'unauthorized': 'Accès non autorisé',
  'forbidden': 'Action non autorisée',
  'invalid credentials': 'Identifiants incorrects',
  'connection refused': 'Connexion refusée',
  'timeout': 'Délai d\'attente dépassé',
  'network error': 'Erreur réseau',
  'internal server error': 'Erreur serveur interne',
  'not found': 'Ressource introuvable',
  'bad request': 'Requête invalide',
  'service unavailable': 'Service indisponible',
  'already exists': 'Ressource déjà existante',
  'invalid token': 'Jeton invalide ou expiré',
  'permission denied': 'Permission refusée',
}

/**
 * Translates an Axios/native error into a message in the active UI language.
 * Prefers the response's `i18nKey` (resolved against locales/{fr,en}/errors.json
 * in whatever language the UI is currently showing — not the server's own
 * Accept-Language-resolved text, which follows the browser's language and can
 * disagree with a UI language the user switched manually) and falls back to
 * substring-matching the raw message for endpoints not yet migrated.
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

  for (const [substring, translation] of Object.entries(LEGACY_FALLBACK_MESSAGES)) {
    if (lower.includes(substring)) return translation
  }

  return raw.charAt(0).toUpperCase() + raw.slice(1)
}
