import { ref, watch } from 'vue'

/**
 * A reactive ref that is persisted to localStorage.
 * @param {string} key - localStorage key
 * @template T
 * @param {T} defaultValue - value to use if the key does not exist
 * @returns {import('vue').Ref<T>}
 */
export function useLocalStorage(key, defaultValue) {
  const stored = localStorage.getItem(key)
  let initial = defaultValue
  if (stored !== null) {
    try {
      initial = JSON.parse(stored)
    } catch {
      // Legacy value stored without JSON.stringify — use as-is and re-persist in correct format
      initial = stored
    }
  }

  const value = ref(initial)

  watch(value, (newVal) => {
    if (newVal === null || newVal === undefined) {
      localStorage.removeItem(key)
    } else {
      localStorage.setItem(key, JSON.stringify(newVal))
    }
  })

  return value
}
