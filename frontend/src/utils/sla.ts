// Pure helpers shared by the SLA view and the public status page.
import { localeTag } from '../i18n'

/** Percentage with up to `digits` decimals in the active locale ("99,95"). */
export function formatPercent(value: number | null | undefined, digits = 2): string {
  if (value == null || Number.isNaN(value)) return ''
  return new Intl.NumberFormat(localeTag(), { maximumFractionDigits: digits }).format(value)
}

/** Badge class for an SLA window status (met / at_risk / breached / no_data). */
export function slaStatusClass(status: string): string {
  switch (status) {
    case 'met': return 'bg-success-lt text-success'
    case 'at_risk': return 'bg-warning-lt text-warning'
    case 'breached': return 'bg-danger-lt text-danger'
    default: return 'bg-secondary-lt text-secondary'
  }
}

/** i18n key suffix of an SLA window status. */
export function slaStatusKey(status: string): string {
  switch (status) {
    case 'met': return 'statusMet'
    case 'at_risk': return 'statusAtRisk'
    case 'breached': return 'statusBreached'
    default: return 'statusNoData'
  }
}

/** Badge class for a public service / page status. */
export function publicStatusClass(status: string): string {
  switch (status) {
    case 'operational': return 'bg-success text-white'
    case 'partial_outage': return 'bg-warning text-white'
    case 'major_outage': return 'bg-danger text-white'
    default: return 'bg-secondary text-white'
  }
}

/**
 * Colour class of one day of the 90-day uptime bar: no data, flawless,
 * minor blips, or a real outage.
 */
export function uptimeDayClass(pct: number | null | undefined): string {
  if (pct == null) return 'uptime-day-none'
  if (pct >= 99.9) return 'uptime-day-ok'
  if (pct >= 95) return 'uptime-day-warn'
  return 'uptime-day-down'
}

/** Whole minutes between two instants (never negative). */
export function minutesBetween(start: string, end: string | null | undefined, now: Date = new Date()): number {
  const endMs = end ? new Date(end).getTime() : now.getTime()
  return Math.max(0, Math.round((endMs - new Date(start).getTime()) / 60000))
}
