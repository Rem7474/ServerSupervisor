import { ref } from 'vue'

const isOpen = ref(false)
const message = ref('')
const title = ref('')
const variant = ref('warning')  // 'warning' | 'danger'
let resolvePromise = null

export function useConfirmDialog() {
  function confirm(options) {
    title.value = options.title || 'Confirmation'
    message.value = options.message || ''
    variant.value = options.variant || 'warning'
    isOpen.value = true
    return new Promise((resolve) => {
      resolvePromise = resolve
    })
  }

  function onConfirm() {
    isOpen.value = false
    resolvePromise?.(true)
  }

  function onCancel() {
    isOpen.value = false
    resolvePromise?.(false)
  }

  return { isOpen, title, message, variant, confirm, onConfirm, onCancel }
}
