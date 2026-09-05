import { describe, it, expect, beforeEach } from 'vitest'
import { confirmAptCommand } from './aptConfirm'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import { setLocale } from '../i18n'

beforeEach(() => {
  setLocale('fr')
})

describe('confirmAptCommand', () => {
  it('resolves true immediately for update, without opening a dialog', async () => {
    const dialog = useConfirmDialog()
    const result = await confirmAptCommand('update', 'web-01')
    expect(result).toBe(true)
    expect(dialog.isOpen.value).toBe(false)
  })

  it('opens a warning dialog for upgrade and resolves per the user choice', async () => {
    const dialog = useConfirmDialog()
    const promise = confirmAptCommand('upgrade', 'web-01')
    expect(dialog.isOpen.value).toBe(true)
    expect(dialog.title.value).toBe('apt upgrade')
    expect(dialog.variant.value).toBe('warning')
    expect(dialog.destructive.value).toBe(false)
    expect(dialog.message.value).toBe('Exécuter sur : web-01 ?')
    dialog.onConfirm()
    expect(await promise).toBe(true)
  })

  it('opens a danger dialog with the package-removal warning for dist-upgrade', async () => {
    const dialog = useConfirmDialog()
    const promise = confirmAptCommand('dist-upgrade', 'web-01')
    expect(dialog.variant.value).toBe('danger')
    expect(dialog.destructive.value).toBe(true)
    expect(dialog.message.value).toContain('⚠️ apt dist-upgrade peut supprimer des paquets existants.')
    dialog.onCancel()
    expect(await promise).toBe(false)
  })

  it('includes the host count in the dialog title for a bulk target', async () => {
    const dialog = useConfirmDialog()
    const promise = confirmAptCommand('upgrade', 'web-01, web-02', 2)
    expect(dialog.title.value).toBe('apt upgrade sur 2 hôtes ?')
    dialog.onConfirm()
    await promise
  })
})
