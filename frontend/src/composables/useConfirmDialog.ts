import { ref, Ref } from 'vue'
import { i18n } from '../i18n'

type ConfirmVariant = 'warning' | 'danger'

interface ConfirmOptions {
  title?: string
  message?: string
  variant?: ConfirmVariant
  requiredText?: string
  destructive?: boolean
  okLabel?: string
  cancelLabel?: string
}

interface ConfirmDialogApi {
  isOpen: Ref<boolean>
  title: Ref<string>
  message: Ref<string>
  variant: Ref<ConfirmVariant>
  requiredText: Ref<string>
  destructive: Ref<boolean>
  okLabel: Ref<string>
  cancelLabel: Ref<string>
  confirm: (options: ConfirmOptions) => Promise<boolean>
  onConfirm: () => void
  onCancel: () => void
}

const isOpen: Ref<boolean> = ref(false)
const message: Ref<string> = ref('')
const title: Ref<string> = ref('')
const variant: Ref<ConfirmVariant> = ref('warning')
const requiredText: Ref<string> = ref('')
const destructive: Ref<boolean> = ref(false)
const okLabel: Ref<string> = ref('')
const cancelLabel: Ref<string> = ref('')
let resolvePromise: ((value: boolean) => void) | null = null

export function useConfirmDialog(): ConfirmDialogApi {
  function confirm(options: ConfirmOptions): Promise<boolean> {
    title.value = options.title || i18n.global.t('common.confirmationTitle')
    message.value = options.message || ''
    variant.value = options.variant || 'warning'
    requiredText.value = options.requiredText || ''
    destructive.value = options.destructive || false
    okLabel.value = options.okLabel || i18n.global.t('common.confirm')
    cancelLabel.value = options.cancelLabel || i18n.global.t('common.cancel')
    isOpen.value = true
    return new Promise((resolve) => {
      resolvePromise = resolve
    })
  }

  function onConfirm(): void {
    isOpen.value = false
    requiredText.value = ''
    destructive.value = false
    okLabel.value = ''
    cancelLabel.value = ''
    resolvePromise?.(true)
    resolvePromise = null
  }

  function onCancel(): void {
    isOpen.value = false
    requiredText.value = ''
    destructive.value = false
    okLabel.value = ''
    cancelLabel.value = ''
    resolvePromise?.(false)
    resolvePromise = null
  }

  return { isOpen, title, message, variant, requiredText, destructive, okLabel, cancelLabel, confirm, onConfirm, onCancel }
}
