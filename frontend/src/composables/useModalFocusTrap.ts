import { onMounted, onUnmounted, ref, Ref } from 'vue'

export function useModalFocusTrap(modalRef: Ref<HTMLElement | null>) {
  const initialFocus = ref<HTMLElement | null>(null)

  const getFocusableElements = () => {
    if (!modalRef.value) return []
    return Array.from(
      modalRef.value.querySelectorAll(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      )
    ) as HTMLElement[]
  }

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key !== 'Tab') return

    const focusables = getFocusableElements()
    if (focusables.length === 0) return

    const currentIndex = focusables.indexOf(document.activeElement as HTMLElement)
    let nextIndex = currentIndex + (e.shiftKey ? -1 : 1)

    if (nextIndex < 0) nextIndex = focusables.length - 1 // Wrap to last
    if (nextIndex >= focusables.length) nextIndex = 0 // Wrap to first

    e.preventDefault()
    focusables[nextIndex]?.focus()
  }

  onMounted(() => {
    // Store initial focus
    initialFocus.value = document.activeElement as HTMLElement

    // Focus first input or focusable element
    const focusables = getFocusableElements()
    if (focusables.length > 0) {
      // Prefer focusing the first input
      const firstInput = focusables.find(el => el.tagName === 'INPUT' || el.tagName === 'SELECT' || el.tagName === 'TEXTAREA')
      if (firstInput) {
        firstInput.focus()
      } else {
        focusables[0]?.focus()
      }
    }

    // Add trap listener
    modalRef.value?.addEventListener('keydown', handleKeyDown)
  })

  onUnmounted(() => {
    // Restore initial focus
    modalRef.value?.removeEventListener('keydown', handleKeyDown)
    initialFocus.value?.focus()
  })

  return { getFocusableElements }
}
