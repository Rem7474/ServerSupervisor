import { useConfirmDialog } from '../composables/useConfirmDialog'

export async function confirmBulkAction(
  action: string,
  count: number,
  warningMessage?: string
): Promise<boolean> {
  const { confirm } = useConfirmDialog()

  return await confirm({
    title: `${action} sur ${count} élément${count > 1 ? 's' : ''} ?`,
    message: warningMessage || 'Cette action ne peut pas être annulée',
    destructive: true,
    variant: 'danger',
    okLabel: `Oui, ${action}`,
    cancelLabel: 'Annuler',
  })
}

export async function confirmDestructiveAction(
  title: string,
  description: string
): Promise<boolean> {
  const { confirm } = useConfirmDialog()

  return await confirm({
    title,
    message: description,
    destructive: true,
    variant: 'danger',
  })
}