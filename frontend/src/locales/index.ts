import fr from './fr'

/**
 * The fallback locale is bundled eagerly: `i18n.ts` needs messages the moment
 * the module evaluates (main.ts renders its boot placeholder before mount),
 * and `fallbackLocale: 'fr'` means every other locale degrades to it rather
 * than to a raw key while its own chunk is still in flight.
 */
export { fr }

/**
 * One dynamic import per non-fallback locale — each becomes its own chunk.
 * A dynamic `import(`./${locale}`)` would defeat that by making Rollup emit a
 * chunk for every file the pattern could match, so the map is explicit.
 */
const LOADERS: Record<string, () => Promise<{ default: Record<string, unknown> }>> = {
  en: () => import('./en'),
}

/** Loads one locale's messages on demand. */
export async function loadLocaleMessages(locale: string): Promise<Record<string, unknown>> {
  if (locale === 'fr') return fr
  const load = LOADERS[locale]
  if (!load) throw new Error(`[i18n] no messages bundled for locale "${locale}"`)
  return (await load()).default
}
