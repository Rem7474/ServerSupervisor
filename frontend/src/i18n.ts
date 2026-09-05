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

export const i18n = createI18n({
  legacy: false,
  locale: detectLocale(),
  fallbackLocale: 'fr',
  messages: { fr, en },
})

/** Switches the active locale, persists the choice, and keeps dayjs / <html lang> in sync. */
export function setLocale(locale: SupportedLocale): void {
  i18n.global.locale.value = locale
  localStorage.setItem(STORAGE_KEY, locale)
  setDayjsLocale(locale)
  document.documentElement.lang = locale
}

setDayjsLocale(i18n.global.locale.value)
document.documentElement.lang = i18n.global.locale.value
