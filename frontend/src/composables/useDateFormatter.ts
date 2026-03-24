import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import utc from 'dayjs/plugin/utc'
import 'dayjs/locale/fr'

dayjs.extend(relativeTime)
dayjs.extend(utc)
dayjs.locale('fr')

type DateInput = string | Date | null | undefined

interface UseDateFormatterApi {
  dayjs: typeof dayjs
  formatRelativeDate: (date: DateInput, emptyValue?: string) => string
  formatExactDate: (date: DateInput, emptyValue?: string) => string
  formatLocaleDateTime: (date: DateInput, emptyValue?: string) => string
  formatRelativeTime: (date: DateInput, emptyValue?: string, shortSeconds?: boolean) => string
}

/**
 * Format a date in relative french text with an optional short-seconds mode.
 */
export function formatRelativeTime(
  date: DateInput,
  emptyValue: string = 'Jamais',
  shortSeconds: boolean = false
): string {
  if (!date || date === '0001-01-01T00:00:00Z') return emptyValue

  const dateObj = dayjs.utc(date).local()
  if (!shortSeconds) return dateObj.fromNow()

  const now = dayjs()
  const diffSeconds = now.diff(dateObj, 'second')

  if (diffSeconds < 10) return 'il y a quelques secondes'
  if (diffSeconds < 60) return `il y a ${diffSeconds}s`
  return dateObj.fromNow()
}

export function useDateFormatter(): UseDateFormatterApi {
  function formatRelativeDate(date: DateInput, emptyValue: string = 'Jamais'): string {
    return formatRelativeTime(date, emptyValue, false)
  }

  function formatExactDate(date: DateInput, emptyValue: string = '-'): string {
    if (!date || date === '0001-01-01T00:00:00Z') return emptyValue
    return dayjs.utc(date).local().format('DD/MM/YYYY HH:mm')
  }

  function formatLocaleDateTime(date: DateInput, emptyValue: string = ''): string {
    if (!date) return emptyValue
    return new Date(date).toLocaleString('fr-FR')
  }

  return {
    dayjs,
    formatRelativeDate,
    formatExactDate,
    formatLocaleDateTime,
    formatRelativeTime,
  }
}
