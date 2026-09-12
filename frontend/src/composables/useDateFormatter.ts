import { i18n } from '../i18n'
import dayjs from '../utils/dayjs'

type DateInput = string | Date | null | undefined

interface UseDateFormatterApi {
  dayjs: typeof dayjs
  formatRelativeDate: (date: DateInput, emptyValue?: string) => string
  formatExactDate: (date: DateInput, emptyValue?: string) => string
  formatLocaleDateTime: (date: DateInput, emptyValue?: string) => string
  formatRelativeTime: (date: DateInput, emptyValue?: string, shortSeconds?: boolean) => string
}

function defaultNever(): string {
  return i18n.global.t('common.never')
}

/**
 * Format a date in relative text (active i18n locale) with an optional short-seconds mode.
 */
export function formatRelativeTime(
  date: DateInput,
  emptyValue?: string,
  shortSeconds: boolean = false
): string {
  const fallback = emptyValue ?? defaultNever()
  if (!date || date === '0001-01-01T00:00:00Z') return fallback

  const dateObj = dayjs.utc(date).local()
  if (!shortSeconds) return dateObj.fromNow()

  const now = dayjs()
  const diffSeconds = now.diff(dateObj, 'second')

  if (diffSeconds < 10) return i18n.global.t('common.justNow')
  if (diffSeconds < 60) return i18n.global.t('common.secondsAgo', { n: diffSeconds })
  return dateObj.fromNow()
}

export function useDateFormatter(): UseDateFormatterApi {
  function formatRelativeDate(date: DateInput, emptyValue?: string): string {
    return formatRelativeTime(date, emptyValue, false)
  }

  function formatExactDate(date: DateInput, emptyValue: string = '-'): string {
    if (!date || date === '0001-01-01T00:00:00Z') return emptyValue
    return dayjs.utc(date).local().format('DD/MM/YYYY HH:mm')
  }

  function formatLocaleDateTime(date: DateInput, emptyValue: string = ''): string {
    if (!date) return emptyValue
    return new Date(date).toLocaleString(i18n.global.locale.value)
  }

  return {
    dayjs,
    formatRelativeDate,
    formatExactDate,
    formatLocaleDateTime,
    formatRelativeTime,
  }
}
