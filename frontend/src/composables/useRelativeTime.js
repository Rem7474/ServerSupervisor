import { ref, onMounted, onUnmounted } from 'vue'
import { formatRelativeTime } from './useDateFormatter'

/**
 * @template T
 * @typedef {{ value: T }} MaybeRef
 */

/**
 * @typedef {string | Date | null | undefined | MaybeRef<string | Date | null | undefined> | (() => (string | Date | null | undefined))} RelativeDateInput
 */

/**
 * Composable pour afficher un timestamp en temps relatif avec mise a jour automatique.
 * @param {RelativeDateInput} dateInput
 * @param {number} [updateInterval=1000]
 * @returns {import('vue').Ref<string>}
 */
export function useRelativeTime(dateInput, updateInterval = 1000) {
  const relativeText = ref('')
  let intervalId = null

  const resolveDate = () => {
    if (typeof dateInput === 'function') return dateInput()
    return dateInput && typeof dateInput === 'object' && 'value' in dateInput ? dateInput.value : dateInput
  }

  function updateRelativeTime() {
    relativeText.value = formatRelativeTime(resolveDate(), 'Jamais', true)
  }

  onMounted(() => {
    updateRelativeTime()
    intervalId = setInterval(updateRelativeTime, updateInterval)
  })

  onUnmounted(() => {
    if (intervalId) clearInterval(intervalId)
  })

  return relativeText
}

/**
 * Fonction utilitaire pour formater une date sans reactivite.
 * @param {string | Date | null | undefined} date
 * @returns {string}
 */
export function formatRelativeTimeStatic(date) {
  return formatRelativeTime(date, 'Jamais', true)
}
