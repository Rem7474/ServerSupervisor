import { ref, watch, Ref } from 'vue'

/**
 * A reactive ref that is persisted to localStorage.
 * @param key - localStorage key
 * @param defaultValue - value to use if the key does not exist
 * @returns A reactive ref bound to localStorage
 */
export function useLocalStorage<T = unknown>(key: string, defaultValue: T): Ref<T> {
  const stored = localStorage.getItem(key)
  let initial: T = defaultValue
  if (stored !== null) {
    try {
      initial = JSON.parse(stored) as T
    } catch {
      // Legacy value stored without JSON.stringify — use as-is and re-persist in correct format
      initial = stored as unknown as T
    }
  }

  const value = ref(initial) as Ref<T>

  watch(value, (newVal: T) => {
    if (newVal === null || newVal === undefined) {
      localStorage.removeItem(key)
    } else {
      localStorage.setItem(key, JSON.stringify(newVal))
    }
  })

  return value
}
