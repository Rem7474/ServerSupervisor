import { i18n } from '../i18n'

/** Returns the localized label for a host status. */
export function formatHostStatus(status: string): string {
  switch (status) {
    case 'online':  return i18n.global.t('common.statusOnline')
    case 'warning': return 'Warning'
    case 'offline': return i18n.global.t('common.statusOffline')
    default:        return i18n.global.t('common.statusUnknown')
  }
}

/** Returns the Tabler CSS class matching a host status. */
export function hostStatusClass(status: string): string {
  switch (status) {
    case 'online':  return 'status status-success'
    case 'warning': return 'status status-warning'
    case 'offline': return 'status status-danger'
    default:        return 'status status-secondary'
  }
}
