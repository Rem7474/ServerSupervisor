/**
 * Shared date/time/number formatters used across multiple views.
 *
 * Everything locale-sensitive resolves the tag through `localeTag()` rather than
 * hardcoding one: `Intl` reads the reactive vue-i18n locale, so a component
 * formatting a date in its template re-renders on a language switch.
 */
import { i18n, localeTag } from '../i18n'

type DateInput = string | number | Date | null | undefined
type NumberInput = number | null | undefined

/** Format an ISO date string as a short numeric date (locale order). */
export function formatDate(dt: DateInput): string {
  if (!dt) return '-'
  return new Date(dt).toLocaleDateString(localeTag(), { year: 'numeric', month: '2-digit', day: '2-digit' })
}

/** Format an ISO date string as a short date + time (locale order). */
export function formatDateTime(dt: DateInput): string {
  if (!dt) return '-'
  return new Date(dt).toLocaleString(localeTag(), { dateStyle: 'short', timeStyle: 'short' })
}

/** Format an ISO date string as a long date (e.g. "25 février 2026" / "February 25, 2026"). */
export function formatDateLong(dt: DateInput): string {
  if (!dt) return '-'
  return new Date(dt).toLocaleDateString(localeTag(), { year: 'numeric', month: 'long', day: 'numeric' })
}

/**
 * Format a duration in seconds to a human-readable string.
 * e.g. 65 → "1min 5s", 3661 → "1h 1min", 30 → "30s"
 *
 * The h/min/s abbreviations are identical in both supported locales, so this
 * one stays literal.
 */
export function formatDurationSecs(seconds: NumberInput): string {
  if (!seconds && seconds !== 0) return '-'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = seconds % 60
  if (h > 0) return m > 0 ? `${h}h ${m}min` : `${h}h`
  if (m > 0) return s > 0 ? `${m}min ${s}s` : `${m}min`
  return `${s}s`
}

/** Format an uptime in seconds, e.g. "3j 4h" (fr) / "3d 4h" (en), or "4h 10m". */
export function formatUptime(seconds: NumberInput): string {
  if (seconds == null) return 'N/A'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  if (days > 0) return `${days}${i18n.global.t('common.dayUnitShort')} ${hours}h`
  const mins = Math.floor((seconds % 3600) / 60)
  return `${hours}h ${mins}m`
}

/** Format bytes to a human-readable string (KB, MB, GB). */
export function formatBytes(bytes: NumberInput): string {
  if (!bytes && bytes !== 0) return '-'
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}

/** Format an integer with the active locale's grouping separator. */
export function formatNumber(value: NumberInput): string {
  return new Intl.NumberFormat(localeTag()).format(Number(value) || 0)
}

/** Compare two strings using the active locale's collation. */
export function compareStrings(a: string, b: string, options?: Intl.CollatorOptions): number {
  return a.localeCompare(b, localeTag(), options)
}
