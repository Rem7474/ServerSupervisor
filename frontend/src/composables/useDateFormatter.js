import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import utc from 'dayjs/plugin/utc'
import 'dayjs/locale/fr'

dayjs.extend(relativeTime)
dayjs.extend(utc)
dayjs.locale('fr')

/**
 * @typedef {string | Date | null | undefined} DateInput
 */

/**
 * Format a date in relative french text with an optional short-seconds mode.
 * @param {DateInput} date
 * @param {string} [emptyValue='Jamais']
 * @param {boolean} [shortSeconds=false]
 * @returns {string}
 */
export function formatRelativeTime(date, emptyValue = 'Jamais', shortSeconds = false) {
  if (!date || date === '0001-01-01T00:00:00Z') return emptyValue

  const dateObj = dayjs.utc(date).local()
  if (!shortSeconds) return dateObj.fromNow()

  const now = dayjs()
  const diffSeconds = now.diff(dateObj, 'second')

  if (diffSeconds < 10) return 'il y a quelques secondes'
  if (diffSeconds < 60) return `il y a ${diffSeconds}s`
  return dateObj.fromNow()
}

/**
 * @returns {{
 *  dayjs: typeof dayjs,
 *  formatRelativeDate: (date: DateInput, emptyValue?: string) => string,
 *  formatExactDate: (date: DateInput, emptyValue?: string) => string,
 *  formatLocaleDateTime: (date: DateInput, emptyValue?: string) => string,
 *  formatRelativeTime: (date: DateInput, emptyValue?: string, shortSeconds?: boolean) => string,
 * }}
 */
export function useDateFormatter() {
  /**
   * @param {DateInput} date
   * @param {string} [emptyValue='Jamais']
   * @returns {string}
   */
  function formatRelativeDate(date, emptyValue = 'Jamais') {
    return formatRelativeTime(date, emptyValue, false)
  }

  /**
   * @param {DateInput} date
   * @param {string} [emptyValue='-']
   * @returns {string}
   */
  function formatExactDate(date, emptyValue = '-') {
    if (!date || date === '0001-01-01T00:00:00Z') return emptyValue
    return dayjs.utc(date).local().format('DD/MM/YYYY HH:mm')
  }

  /**
   * @param {DateInput} date
   * @param {string} [emptyValue='']
   * @returns {string}
   */
  function formatLocaleDateTime(date, emptyValue = '') {
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
