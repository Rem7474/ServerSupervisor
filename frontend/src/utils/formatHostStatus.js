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
    case 'online':  return 'badge bg-green-lt text-green status-badge status-dot-animated'
    case 'warning': return 'badge bg-yellow-lt text-yellow status-badge status-dot-animated'
    case 'offline': return 'badge bg-red-lt text-red status-badge status-dot-animated'
    default:        return 'badge bg-secondary-lt text-secondary status-badge'
  }
}
