import { createI18n } from 'vue-i18n'
import { fr, loadLocaleMessages } from './locales'
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
  // Only the fallback ships in the entry chunk; the rest arrive via
  // ensureLocaleMessages() before they are ever displayed. The cast tells
  // vue-i18n every supported locale is legal to switch to — which it is, once
  // its chunk has been registered.
  messages: { fr } as Record<SupportedLocale, typeof fr>,
  pluralRules: {
    // CLDR's French rule is `one` for i = 0 or 1; vue-i18n's built-in default is
    // the English one (`one` for exactly 1), which renders "0 hôtes" instead of
    // "0 hôte" on every pluralized French message.
    fr: (choice: number) => (Math.abs(choice) <= 1 ? 0 : 1),
  },
  missing: (locale, key) => {
    // Always logged, not just in dev: a key can only go missing here through a
    // *dynamically built* path (`errors.${code}`, `alerts.metricLabels.${m}`,
    // …) since locales.spec.ts fails the build on a static key present in one
    // language only — so this fires exactly when the server grew a code the
    // SPA doesn't know yet, which is worth seeing in production.
    console.warn(`[i18n] missing key "${key}" for locale "${locale}"`)

    // Dev keeps vue-i18n's default (render the key path) — loud and greppable.
    if (import.meta.env.DEV) return undefined

    // Production never shows a dotted developer path. The leaf of a dynamic key
    // is the server's own identifier (INVALID_TOKEN, cpu_temperature, …), which
    // humanizes into something a user can act on.
    return humanizeKeyLeaf(key)
  },
})

/** `errors.INVALID_TOKEN` → `Invalid token`; `…metricLabels.cpu_temp` → `Cpu temp`. */
export function humanizeKeyLeaf(key: string): string {
  const leaf = key.split('.').pop() ?? key
  const words = leaf
    .replace(/[_-]+/g, ' ')
    .replace(/([a-z\d])([A-Z])/g, '$1 $2')
    .trim()
    .toLowerCase()
  return words.charAt(0).toUpperCase() + words.slice(1)
}

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

/**
 * Fetches a locale's chunk unless it is already registered. Callers that can
 * change the locale (the switcher, the boot sequence) await this first, which
 * keeps setLocale() synchronous for everyone else.
 */
export async function ensureLocaleMessages(locale: SupportedLocale): Promise<void> {
  if (Object.keys(i18n.global.getLocaleMessage(locale) ?? {}).length > 0) return
  i18n.global.setLocaleMessage(locale, await loadLocaleMessages(locale) as never)
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
