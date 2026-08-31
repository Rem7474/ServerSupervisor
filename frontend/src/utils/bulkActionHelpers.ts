import { useConfirmDialog } from '../composables/useConfirmDialog'
import { i18n } from '../i18n'

export async function confirmBulkAction(
  action: string,
  count: number,
  warningMessage?: string
): Promise<boolean> {
  const { confirm } = useConfirmDialog()
  const { t } = i18n.global

  return await confirm({
    title: t('common.bulkConfirmTitle', { action, count }, count),
    message: warningMessage || t('common.cannotUndo'),
    destructive: true,
    variant: 'danger',
    okLabel: t('common.yesAction', { action }),
    cancelLabel: t('common.cancel'),
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