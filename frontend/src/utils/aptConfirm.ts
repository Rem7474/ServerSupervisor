import { useConfirmDialog } from '../composables/useConfirmDialog'

// `apt update` only refreshes the package index — non-destructive, unlike
// upgrade/dist-upgrade which stay confirmed. Centralized here so the
// confirmation copy (notably the dist-upgrade package-removal warning) can't
// silently drift between call sites — the host-detail tab's dialog used to
// be missing that warning for the exact same destructive action the
// fleet-wide /apt page already warned about.
export async function confirmAptCommand(command: string, targetLabel: string, count = 1): Promise<boolean> {
  if (command === 'update') return true
  const { confirm } = useConfirmDialog()
  const title = count > 1 ? `apt ${command} sur ${count} hôtes ?` : `apt ${command}`
  const message = command === 'dist-upgrade'
    ? `⚠️ apt dist-upgrade peut supprimer des paquets existants.\nExécuter sur : ${targetLabel} ?`
    : `Exécuter sur : ${targetLabel} ?`
  return confirm({
    title,
    message,
    variant: command === 'dist-upgrade' ? 'danger' : 'warning',
    destructive: command === 'dist-upgrade',
  })
}
