/**
 * Dictionnaire de traduction des messages d'erreur backend → français.
 * Les clés sont des sous-chaînes (insensibles à la casse) du message d'origine.
 */
const ERROR_MESSAGES: Record<string, string> = {
  'host not found':        'Hôte introuvable',
  'unauthorized':          'Accès non autorisé',
  'forbidden':             'Action non autorisée',
  'invalid credentials':   'Identifiants incorrects',
  'connection refused':    'Connexion refusée',
  'timeout':               'Délai d\'attente dépassé',
  'network error':         'Erreur réseau',
  'internal server error': 'Erreur serveur interne',
  'not found':             'Ressource introuvable',
  'bad request':           'Requête invalide',
  'service unavailable':   'Service indisponible',
  'already exists':        'Ressource déjà existante',
  'invalid token':         'Jeton invalide ou expiré',
  'permission denied':     'Permission refusée',
}

interface ErrorLike {
  response?: { data?: { error?: unknown; message?: unknown } }
  message?: unknown
}

/** Traduit un objet erreur (Axios ou native Error) en message français lisible. */
export function translateError(error: unknown): string {
  if (!error) return 'Une erreur est survenue'

  const e = (typeof error === 'object' && error !== null ? error : {}) as ErrorLike
  const raw = String(
    e.response?.data?.error ||
    e.response?.data?.message ||
    e.message ||
    error
  )

  const lower = raw.toLowerCase()

  for (const [key, translation] of Object.entries(ERROR_MESSAGES)) {
    if (lower.includes(key)) return translation
  }

  // Aucune traduction : retourner le message original avec majuscule initiale
  return raw.charAt(0).toUpperCase() + raw.slice(1)
}
