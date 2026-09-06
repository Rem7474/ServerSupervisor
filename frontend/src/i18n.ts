import { createI18n } from 'vue-i18n'
import { en, fr } from './locales'
import { setDayjsLocale } from './utils/dayjs'

export const SUPPORTED_LOCALES = ['fr', 'en'] as const
export type SupportedLocale = (typeof SUPPORTED_LOCALES)[number]

const STORAGE_KEY = 'locale'

function isSupportedLocale(value: string | null): value is SupportedLocale {
  return value !== null && (SUPPORTED_LOCALES as readonly string[]).includes(value)
}

function detectLocale(): SupportedLocale {
  const stored = localStorage.getItem(STORAGE_KEY)
  if (isSupportedLocale(stored)) return stored

  const browserLang = (navigator.language || '').slice(0, 2).toLowerCase()
  return isSupportedLocale(browserLang) ? browserLang : 'fr'
}

/**
 * BCP-47 tag per supported locale, for `Intl` / `toLocale*` formatting.
 * `SUPPORTED_LOCALES` are bare language subtags; date, number and collation
 * formatting need a region to pick a convention (`fr-FR` → `06/09/2026`,
 * `en-US` → `9/6/2026`).
 */
const LOCALE_TAGS: Record<SupportedLocale, string> = {
  fr: 'fr-FR',
  en: 'en-US',
}

export const i18n = createI18n({
  legacy: false,
  locale: detectLocale(),
  fallbackLocale: 'fr',
  messages: { fr, en },
  pluralRules: {
    // CLDR's French rule is `one` for i = 0 or 1; vue-i18n's built-in default is
    // the English one (`one` for exactly 1), which renders "0 hôtes" instead of
    // "0 hôte" on every pluralized French message.
    fr: (choice: number) => (Math.abs(choice) <= 1 ? 0 : 1),
  },
  missing: (locale, key) => {
    // Never let a raw key path reach the UI unnoticed.
    if (import.meta.env.DEV) console.warn(`[i18n] missing key "${key}" for locale "${locale}"`)
  },
})

/** The active locale, narrowed back to `SupportedLocale`. */
export function currentLocale(): SupportedLocale {
  const l = i18n.global.locale.value
  return isSupportedLocale(l) ? l : 'fr'
}

/**
 * BCP-47 tag for the active locale. Reads the reactive locale ref, so a
 * component formatting a date in its template re-renders on a language switch.
 */
export function localeTag(): string {
  return LOCALE_TAGS[currentLocale()]
}

/** Switches the active locale, persists the choice, and keeps dayjs / <html lang> in sync. */
export function setLocale(locale: SupportedLocale): void {
  i18n.global.locale.value = locale
  localStorage.setItem(STORAGE_KEY, locale)
  setDayjsLocale(locale)
  document.documentElement.lang = locale
}

setDayjsLocale(i18n.global.locale.value)
document.documentElement.lang = i18n.global.locale.value
