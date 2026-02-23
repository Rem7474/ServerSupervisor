/**
 * Dictionnaire de traduction des messages d'erreur backend → français.
 * Les clés sont des sous-chaînes (insensibles à la casse) du message d'origine.
 */
const ERROR_MESSAGES = {
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

/**
 * Traduit un objet erreur (Axios ou native Error) en message français lisible.
 * @param {Error|unknown} error
 * @returns {string}
 */
export function translateError(error) {
  if (!error) return 'Une erreur est survenue'

  const raw = (
    error?.response?.data?.error ||
    error?.response?.data?.message ||
    error?.message ||
    String(error)
  )

  const lower = raw.toLowerCase()

  for (const [key, translation] of Object.entries(ERROR_MESSAGES)) {
    if (lower.includes(key)) return translation
  }

  // Aucune traduction : retourner le message original avec majuscule initiale
  return raw.charAt(0).toUpperCase() + raw.slice(1)
}
