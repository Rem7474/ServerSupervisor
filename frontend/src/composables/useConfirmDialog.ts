import { ref, Ref } from 'vue'

type ConfirmVariant = 'warning' | 'danger'

interface ConfirmOptions {
  title?: string
  message?: string
  variant?: ConfirmVariant
  requiredText?: string
}

interface ConfirmDialogApi {
  isOpen: Ref<boolean>
  title: Ref<string>
  message: Ref<string>
  variant: Ref<ConfirmVariant>
  requiredText: Ref<string>
  confirm: (options: ConfirmOptions) => Promise<boolean>
  onConfirm: () => void
  onCancel: () => void
}

const isOpen: Ref<boolean> = ref(false)
const message: Ref<string> = ref('')
const title: Ref<string> = ref('')
const variant: Ref<ConfirmVariant> = ref('warning')
const requiredText: Ref<string> = ref('')
let resolvePromise: ((value: boolean) => void) | null = null

export function useConfirmDialog(): ConfirmDialogApi {
  function confirm(options: ConfirmOptions): Promise<boolean> {
    title.value = options.title || 'Confirmation'
    message.value = options.message || ''
    variant.value = options.variant || 'warning'
    requiredText.value = options.requiredText || ''
    isOpen.value = true
    return new Promise((resolve) => {
      resolvePromise = resolve
    })
  }

  function onConfirm(): void {
    isOpen.value = false
    requiredText.value = ''
    resolvePromise?.(true)
  }

  function onCancel(): void {
    isOpen.value = false
    requiredText.value = ''
    resolvePromise?.(false)
  }

  return { isOpen, title, message, variant, requiredText, confirm, onConfirm, onCancel }
}
