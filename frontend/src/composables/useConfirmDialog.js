import { ref } from 'vue'

/**
 * @typedef {'warning' | 'danger'} ConfirmVariant
 */

/**
 * @typedef {Object} ConfirmOptions
 * @property {string} [title]
 * @property {string} [message]
 * @property {ConfirmVariant} [variant]
 * @property {string} [requiredText]
 */

/**
 * @typedef {Object} ConfirmDialogApi
 * @property {import('vue').Ref<boolean>} isOpen
 * @property {import('vue').Ref<string>} title
 * @property {import('vue').Ref<string>} message
 * @property {import('vue').Ref<ConfirmVariant>} variant
 * @property {import('vue').Ref<string>} requiredText
 * @property {(options: ConfirmOptions) => Promise<boolean>} confirm
 * @property {() => void} onConfirm
 * @property {() => void} onCancel
 */

const isOpen = ref(false)
const message = ref('')
const title = ref('')
/** @type {import('vue').Ref<ConfirmVariant>} */
const variant = ref('warning')
const requiredText = ref('')    // if set, user must type this exact string to confirm
/** @type {((value: boolean) => void) | null} */
let resolvePromise = null

/** @returns {ConfirmDialogApi} */
export function useConfirmDialog() {
  /** @param {ConfirmOptions} options */
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
