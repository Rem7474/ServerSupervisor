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
export function formatDateTime(dt: DateInput, empty = '-'): string {
  if (!dt) return empty
  return new Date(dt).toLocaleString(localeTag(), { dateStyle: 'short', timeStyle: 'short' })
}

/**
 * Short date + time with seconds, for event timestamps (requests, syslog
 * lines, executions). An unparseable string is shown as-is rather than
 * "Invalid Date".
 */
export function formatDateTimeSeconds(dt: DateInput, empty = '-'): string {
  if (!dt && dt !== 0) return empty
  const d = new Date(dt)
  if (Number.isNaN(d.getTime())) return typeof dt === 'string' ? dt : empty
  return d.toLocaleString(localeTag(), { dateStyle: 'short', timeStyle: 'medium' })
}

/** Time of day only (hours and minutes, optionally seconds). */
export function formatTime(dt: DateInput, withSeconds = false): string {
  if (!dt && dt !== 0) return '-'
  return new Date(dt).toLocaleTimeString(localeTag(), {
    hour: '2-digit',
    minute: '2-digit',
    ...(withSeconds ? { second: '2-digit' } : {}),
  })
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

/**
 * Format an uptime in seconds: "3j 4h" (fr) / "3d 4h" (en), "4h 10m", or
 * "10m" under an hour. A missing or zero uptime (never reported) is `empty`.
 */
export function formatUptime(seconds: NumberInput, empty = 'N/A'): string {
  if (!seconds) return empty
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const mins = Math.floor((seconds % 3600) / 60)
  if (days > 0) return `${days}${i18n.global.t('common.dayUnitShort')} ${hours}h`
  if (hours > 0) return `${hours}h ${mins}m`
  return `${mins}m`
}

const BYTE_UNIT_KEYS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'].map((u) => `common.byteUnit${u}`)

/**
 * Format a byte count (base 1024) with translated units ("1,5 Go" in French,
 * "1.5 GB" in English): whole bytes, one decimal from kilobytes up. A missing
 * value is `empty`; callers that treat "unknown" as zero pass `bytes || 0`.
 */
export function formatBytes(bytes: NumberInput, empty = '-'): string {
  if (bytes == null || Number.isNaN(bytes)) return empty
  let value = Math.max(0, bytes)
  let i = 0
  while (value >= 1024 && i < BYTE_UNIT_KEYS.length - 1) {
    value /= 1024
    i++
  }
  const digits = i === 0 ? 0 : 1
  const n = new Intl.NumberFormat(localeTag(), { minimumFractionDigits: digits, maximumFractionDigits: digits, useGrouping: false }).format(value)
  return `${n} ${i18n.global.t(BYTE_UNIT_KEYS[i])}`
}

/** Format an integer with the active locale's grouping separator. */
export function formatNumber(value: NumberInput): string {
  return new Intl.NumberFormat(localeTag()).format(Number(value) || 0)
}

/** Compare two strings using the active locale's collation. */
export function compareStrings(a: string, b: string, options?: Intl.CollatorOptions): number {
  return a.localeCompare(b, localeTag(), options)
}
