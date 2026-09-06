<template>
  <div
    class="btn-group btn-group-sm"
    role="group"
    :aria-label="t('common.language')"
  >
    <button
      v-for="option in options"
      :key="option.locale"
      type="button"
      class="btn btn-icon"
      :class="locale === option.locale ? 'btn-primary' : 'btn-outline-secondary'"
      :title="t(option.labelKey)"
      :aria-label="t(option.labelKey)"
      :aria-pressed="locale === option.locale"
      @click="switchLocale(option.locale)"
    >
      <span :class="['flag', 'flag-xs', option.flagClass]" />
    </button>
  </div>
</template>

<script setup lang="ts">
/**
 * The language picker. Lives in the user dropdown once signed in, and on the
 * login page — where it is the only way in, since `detectLocale()` falls back
 * to French for any browser language that isn't fr/en and the app shell (and
 * with it the dropdown) is gated behind `auth.isAuthenticated`.
 */
import { useI18n } from 'vue-i18n'
import { setLocale, SUPPORTED_LOCALES, type SupportedLocale } from '../i18n'

const { t, locale } = useI18n()

const FLAGS: Record<SupportedLocale, { labelKey: string; flagClass: string }> = {
  fr: { labelKey: 'common.languageFrench', flagClass: 'flag-country-fr' },
  en: { labelKey: 'common.languageEnglish', flagClass: 'flag-country-gb' },
}

const options = SUPPORTED_LOCALES.map((l) => ({ locale: l, ...FLAGS[l] }))

function switchLocale(l: SupportedLocale): void {
  setLocale(l)
}
</script>
