/** Retourne le libellé localisé du statut d'un hôte. */
export function formatHostStatus(status: string): string {
  switch (status) {
    case 'online':  return 'En ligne'
    case 'warning': return 'Warning'
    case 'offline': return 'Hors ligne'
    default:        return 'Inconnu'
  }
}

/** Retourne la classe CSS Tabler correspondant au statut d'un hôte. */
export function hostStatusClass(status: string): string {
  switch (status) {
    case 'online':  return 'status status-lime'
    case 'warning': return 'status status-yellow'
    case 'offline': return 'status status-red'
    default:        return 'status status-secondary'
  }
}
