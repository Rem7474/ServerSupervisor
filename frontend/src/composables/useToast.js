import { onUnmounted, ref } from 'vue'

/**
 * @template T
 * @param {T} [initialValue=null]
 * @returns {{
 *  value: import('vue').Ref<T>,
 *  showToast: (nextValue: T, durationMs?: number) => void,
 *  clearToast: () => void,
 * }}
 */
export function useToast(initialValue = null) {
  const value = ref(initialValue)
  let timeoutId = null

  function clearToast() {
    if (timeoutId) {
      clearTimeout(timeoutId)
      timeoutId = null
    }
    value.value = initialValue
  }

  function showToast(nextValue, durationMs = 5000) {
    if (timeoutId) clearTimeout(timeoutId)
    value.value = nextValue
    if (durationMs > 0) {
      timeoutId = setTimeout(() => {
        value.value = initialValue
        timeoutId = null
      }, durationMs)
    }
  }

  onUnmounted(() => {
    if (timeoutId) clearTimeout(timeoutId)
  })

  return {
    value,
    showToast,
    clearToast,
  }
}