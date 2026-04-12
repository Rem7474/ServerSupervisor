/**
 * Retourne le libellé localisé du statut d'un hôte.
 * @param {string} status - 'online' | 'offline' | 'warning' | autre
 * @returns {string}
 */
export function formatHostStatus(status) {
  switch (status) {
    case 'online':  return 'En ligne'
    case 'warning': return 'Warning'
    case 'offline': return 'Hors ligne'
    default:        return 'Inconnu'
  }
}

/**
 * Retourne la classe CSS Tabler correspondant au statut d'un hôte.
 * @param {string} status - 'online' | 'offline' | 'warning' | autre
 * @returns {string}
 */
export function hostStatusClass(status) {
  switch (status) {
    case 'online':  return 'status status-lime'
    case 'warning': return 'status status-yellow'
    case 'offline': return 'status status-red'
    default:        return 'status status-secondary'
  }
}
