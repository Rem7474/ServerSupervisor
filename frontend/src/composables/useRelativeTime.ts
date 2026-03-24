import { ref, onMounted, onUnmounted, Ref } from 'vue'
import { formatRelativeTime } from './useDateFormatter'

type RelativeDateInput =
  | string
  | Date
  | null
  | undefined
  | { value: string | Date | null | undefined }
  | (() => string | Date | null | undefined)

/**
 * Composable to display a timestamp in relative time with automatic updates.
 */
export function useRelativeTime(dateInput: RelativeDateInput, updateInterval: number = 1000): Ref<string> {
  const relativeText: Ref<string> = ref('')
  let intervalId: ReturnType<typeof setInterval> | null = null

  const resolveDate = (): string | Date | null | undefined => {
    if (typeof dateInput === 'function') return dateInput()
    return dateInput && typeof dateInput === 'object' && 'value' in dateInput
      ? (dateInput as any).value
      : dateInput
  }

  function updateRelativeTime(): void {
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
 * Utility function to format a date without reactivity.
 */
export function formatRelativeTimeStatic(date: string | Date | null | undefined): string {
  return formatRelativeTime(date, 'Jamais', true)
}
