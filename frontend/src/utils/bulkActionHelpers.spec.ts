import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { setLocale } from '../i18n'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import { confirmBulkAction } from './bulkActionHelpers'

beforeEach(() => {
  setActivePinia(createPinia())
  setLocale('fr')
})

describe('confirmBulkAction', () => {
  it('builds a singular title/okLabel for a single item', () => {
    void confirmBulkAction('apt upgrade', 1)
    const dialog = useConfirmDialog()
    expect(dialog.title.value).toBe('apt upgrade sur 1 élément ?')
    expect(dialog.okLabel.value).toBe('Oui, apt upgrade')
    expect(dialog.cancelLabel.value).toBe('Annuler')
    expect(dialog.message.value).toBe('Cette action ne peut pas être annulée')
    dialog.onCancel()
  })

  it('builds a plural title for multiple items, in the active locale', () => {
    setLocale('en')
    void confirmBulkAction('restart', 3)
    const dialog = useConfirmDialog()
    expect(dialog.title.value).toBe('restart on 3 items?')
    expect(dialog.okLabel.value).toBe('Yes, restart')
    dialog.onCancel()
  })

  it('uses a custom warning message instead of the default when provided', () => {
    void confirmBulkAction('delete', 2, 'This will remove the containers permanently.')
    const dialog = useConfirmDialog()
    expect(dialog.message.value).toBe('This will remove the containers permanently.')
    dialog.onCancel()
  })

  it('resolves true/false based on which dialog action fires', async () => {
    const confirmed = confirmBulkAction('apt upgrade', 1)
    useConfirmDialog().onConfirm()
    expect(await confirmed).toBe(true)

    const cancelled = confirmBulkAction('apt upgrade', 1)
    useConfirmDialog().onCancel()
    expect(await cancelled).toBe(false)
  })
})
