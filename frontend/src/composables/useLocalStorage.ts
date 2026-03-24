import { ref, watch, Ref } from 'vue'

/**
 * A reactive ref that is persisted to localStorage.
 * @param key - localStorage key
 * @param defaultValue - value to use if the key does not exist
 * @returns A reactive ref bound to localStorage
 */
export function useLocalStorage<T = any>(key: string, defaultValue: T): Ref<T> {
  const stored = localStorage.getItem(key)
  let initial: any = defaultValue
  if (stored !== null) {
    try {
      initial = JSON.parse(stored)
    } catch {
      // Legacy value stored without JSON.stringify — use as-is and re-persist in correct format
      initial = stored
    }
  }

  const value: Ref<T> = ref(initial)

  watch(value, (newVal: T) => {
    if (newVal === null || newVal === undefined) {
      localStorage.removeItem(key)
    } else {
      localStorage.setItem(key, JSON.stringify(newVal))
    }
  })

  return value
}
