import { onUnmounted, ref, Ref } from 'vue'

interface UseToastApi<T = any> {
  value: Ref<T | null>
  showToast: (nextValue: T, durationMs?: number) => void
  clearToast: () => void
}

export function useToast<T = any>(initialValue: T | null = null): UseToastApi<T> {
  const value: Ref<T | null> = ref(initialValue) as Ref<T | null>
  let timeoutId: ReturnType<typeof setTimeout> | null = null

  function clearToast(): void {
    if (timeoutId) {
      clearTimeout(timeoutId)
      timeoutId = null
    }
    value.value = initialValue
  }

  function showToast(nextValue: T, durationMs: number = 5000): void {
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
