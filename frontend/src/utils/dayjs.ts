import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import utc from 'dayjs/plugin/utc'
import 'dayjs/locale/fr'
import 'dayjs/locale/en'

dayjs.extend(relativeTime)
dayjs.extend(utc)
dayjs.locale('fr')

/** Switches dayjs's active locale (relative-time wording, month/day names, …). */
export function setDayjsLocale(locale: string): void {
  dayjs.locale(locale)
}

export default dayjs
