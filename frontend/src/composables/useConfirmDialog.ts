import { ref, Ref } from 'vue'

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
const okLabel: Ref<string> = ref('Confirmer')
const cancelLabel: Ref<string> = ref('Annuler')
let resolvePromise: ((value: boolean) => void) | null = null

export function useConfirmDialog(): ConfirmDialogApi {
  function confirm(options: ConfirmOptions): Promise<boolean> {
    title.value = options.title || 'Confirmation'
    message.value = options.message || ''
    variant.value = options.variant || 'warning'
    requiredText.value = options.requiredText || ''
    destructive.value = options.destructive || false
    okLabel.value = options.okLabel || 'Confirmer'
    cancelLabel.value = options.cancelLabel || 'Annuler'
    isOpen.value = true
    return new Promise((resolve) => {
      resolvePromise = resolve
    })
  }

  function onConfirm(): void {
    isOpen.value = false
    requiredText.value = ''
    destructive.value = false
    okLabel.value = 'Confirmer'
    cancelLabel.value = 'Annuler'
    resolvePromise?.(true)
    resolvePromise = null
  }

  function onCancel(): void {
    isOpen.value = false
    requiredText.value = ''
    destructive.value = false
    okLabel.value = 'Confirmer'
    cancelLabel.value = 'Annuler'
    resolvePromise?.(false)
    resolvePromise = null
  }

  return { isOpen, title, message, variant, requiredText, destructive, okLabel, cancelLabel, confirm, onConfirm, onCancel }
}
