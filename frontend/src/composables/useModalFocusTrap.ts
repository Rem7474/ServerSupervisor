import { onUnmounted, ref, watch, Ref } from 'vue'

// isOpen is a getter, not a Ref, so callers can pass `() => props.visible`
// or `() => dialog.isOpen.value` uniformly. Watching it (flush: 'post',
// so it runs after the v-if this modal is gated behind has patched the
// DOM) instead of binding in onMounted is the fix for a real bug: every
// consumer of this composable keeps its component mounted for the whole
// page lifetime and toggles an *inner* v-if to show/hide the modal, so
// modalRef.value was still null when onMounted fired and the trap never
// actually engaged.
export function useModalFocusTrap(modalRef: Ref<HTMLElement | null>, isOpen: () => boolean) {
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

  watch(
    isOpen,
    (open) => {
      if (open) {
        initialFocus.value = document.activeElement as HTMLElement

        const focusables = getFocusableElements()
        if (focusables.length > 0) {
          const firstInput = focusables.find(el => el.tagName === 'INPUT' || el.tagName === 'SELECT' || el.tagName === 'TEXTAREA')
          ;(firstInput || focusables[0])?.focus()
        }

        modalRef.value?.addEventListener('keydown', handleKeyDown)
      } else {
        modalRef.value?.removeEventListener('keydown', handleKeyDown)
        initialFocus.value?.focus()
      }
    },
    { flush: 'post' }
  )

  onUnmounted(() => {
    modalRef.value?.removeEventListener('keydown', handleKeyDown)
  })

  return { getFocusableElements }
}
