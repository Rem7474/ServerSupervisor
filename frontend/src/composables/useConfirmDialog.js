import { ref } from 'vue'

const isOpen = ref(false)
const message = ref('')
const title = ref('')
const variant = ref('warning')  // 'warning' | 'danger'
const requiredText = ref('')    // if set, user must type this exact string to confirm
let resolvePromise = null

export function useConfirmDialog() {
  function confirm(options) {
    title.value = options.title || 'Confirmation'
    message.value = options.message || ''
    variant.value = options.variant || 'warning'
    requiredText.value = options.requiredText || ''
    isOpen.value = true
    return new Promise((resolve) => {
      resolvePromise = resolve
    })
  }

  function onConfirm() {
    isOpen.value = false
    requiredText.value = ''
    resolvePromise?.(true)
  }

  function onCancel() {
    isOpen.value = false
    requiredText.value = ''
    resolvePromise?.(false)
  }

  return { isOpen, title, message, variant, requiredText, confirm, onConfirm, onCancel }
}
