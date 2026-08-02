import { computed, onUnmounted, ref, watch, toValue, ComputedRef, Ref, MaybeRefOrGetter } from 'vue'

interface ModalChromeOptions {
  /** Called on ESC (when closeOnEsc is true and this modal is topmost, and not persistent). */
  onClose?: () => void
  /** When true, ESC is ignored — for modals with no dismiss affordance (e.g. a one-time secret display). */
  persistent?: MaybeRefOrGetter<boolean>
  closeOnEsc?: boolean
  lockScroll?: boolean
  trapFocus?: boolean
  autoFocus?: 'first-input' | 'first' | 'none'
}

// Module-level stack of open modals, each identified by a unique symbol.
// ESC and scroll-lock are both driven off this single stack so nested
// modals behave correctly — e.g. ConfirmDialog opened from inside
// IPTimelineModal: ESC closes only the topmost (the confirm), and the
// page scroll stays locked until the *last* modal on the stack closes,
// not just whichever one happens to unmount first.
const openStack: symbol[] = []
const closeHandlers = new Map<symbol, { onClose?: () => void; isPersistent: () => boolean }>()
let escListenerAttached = false
let scrollLockCount = 0
let savedBodyOverflow = ''
let savedBodyPaddingRight = ''

function handleGlobalKeydown(e: KeyboardEvent): void {
  if (e.key !== 'Escape') return
  const topId = openStack[openStack.length - 1]
  if (!topId) return
  const entry = closeHandlers.get(topId)
  if (!entry || entry.isPersistent()) return
  entry.onClose?.()
}

function attachEscListener(): void {
  if (escListenerAttached) return
  document.addEventListener('keydown', handleGlobalKeydown)
  escListenerAttached = true
}

function detachEscListenerIfIdle(): void {
  if (openStack.length === 0 && escListenerAttached) {
    document.removeEventListener('keydown', handleGlobalKeydown)
    escListenerAttached = false
  }
}

// Bootstrap-style scroll lock: hide overflow and compensate the vanished
// scrollbar with padding so the page doesn't visibly shift width. Tabler
// 1.x ships no .modal-open behaviour of its own (Bootstrap applies it via
// JS, not CSS), so this has to be done by hand.
function lockScroll(): void {
  if (scrollLockCount === 0) {
    savedBodyOverflow = document.body.style.overflow
    savedBodyPaddingRight = document.body.style.paddingRight
    const scrollbarWidth = window.innerWidth - document.documentElement.clientWidth
    document.body.style.overflow = 'hidden'
    if (scrollbarWidth > 0) {
      document.body.style.paddingRight = `${scrollbarWidth}px`
    }
  }
  scrollLockCount++
}

function unlockScroll(): void {
  scrollLockCount = Math.max(0, scrollLockCount - 1)
  if (scrollLockCount === 0) {
    document.body.style.overflow = savedBodyOverflow
    document.body.style.paddingRight = savedBodyPaddingRight
  }
}

/**
 * Wires ESC-to-close, a refcounted body scroll lock, and a focus trap onto
 * an existing modal's own markup — no structural change, no wrapper
 * component. Drives everything off `watch(isOpen, ..., { flush: 'post' })`
 * rather than onMounted: every modal in this app stays mounted for the
 * page's whole lifetime and toggles an *inner* v-if to show/hide itself,
 * so modalRef.value is still null at onMounted time.
 */
export function useModalChrome(
  modalRef: Ref<HTMLElement | null>,
  isOpen: MaybeRefOrGetter<boolean>,
  options: ModalChromeOptions = {}
): { isTopmost: ComputedRef<boolean> } {
  const {
    onClose,
    persistent = false,
    closeOnEsc = true,
    lockScroll: shouldLockScroll = true,
    trapFocus = true,
    autoFocus = 'first-input',
  } = options

  const id = Symbol('modal')
  const initialFocus = ref<HTMLElement | null>(null)
  const isTopmost = computed(() => openStack[openStack.length - 1] === id)

  const getFocusableElements = (): HTMLElement[] => {
    if (!modalRef.value) return []
    return Array.from(
      modalRef.value.querySelectorAll(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      )
    ) as HTMLElement[]
  }

  const handleTabTrap = (e: KeyboardEvent): void => {
    if (e.key !== 'Tab') return
    const focusables = getFocusableElements()
    if (focusables.length === 0) return
    const currentIndex = focusables.indexOf(document.activeElement as HTMLElement)
    let nextIndex = currentIndex + (e.shiftKey ? -1 : 1)
    if (nextIndex < 0) nextIndex = focusables.length - 1
    if (nextIndex >= focusables.length) nextIndex = 0
    e.preventDefault()
    focusables[nextIndex]?.focus()
  }

  function openChrome(): void {
    openStack.push(id)
    if (closeOnEsc) {
      closeHandlers.set(id, { onClose, isPersistent: () => toValue(persistent) })
      attachEscListener()
    }
    if (shouldLockScroll) lockScroll()

    if (trapFocus) {
      initialFocus.value = document.activeElement as HTMLElement
      if (autoFocus !== 'none') {
        const focusables = getFocusableElements()
        if (focusables.length > 0) {
          const target = autoFocus === 'first-input'
            ? focusables.find(el => el.tagName === 'INPUT' || el.tagName === 'SELECT' || el.tagName === 'TEXTAREA') || focusables[0]
            : focusables[0]
          target?.focus()
        }
      }
      modalRef.value?.addEventListener('keydown', handleTabTrap)
    }
  }

  function closeChrome(): void {
    const idx = openStack.indexOf(id)
    if (idx !== -1) openStack.splice(idx, 1)
    closeHandlers.delete(id)
    detachEscListenerIfIdle()
    if (shouldLockScroll) unlockScroll()
    if (trapFocus) {
      modalRef.value?.removeEventListener('keydown', handleTabTrap)
      initialFocus.value?.focus()
    }
  }

  watch(
    () => toValue(isOpen),
    (open) => {
      if (open) openChrome()
      else closeChrome()
    },
    { immediate: true, flush: 'post' }
  )

  onUnmounted(() => {
    if (openStack.includes(id)) closeChrome()
  })

  return { isTopmost }
}
