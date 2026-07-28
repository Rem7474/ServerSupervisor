// Shared French labels for remote_commands / tracker-execution style statuses
// (pending/running/completed/failed/cancelled/skipped) — used anywhere a raw
// status string would otherwise leak into an all-French UI untranslated.
const STATUS_LABELS: Record<string, string> = {
  pending: 'En attente',
  running: 'En cours',
  completed: 'Terminé',
  failed: 'Échoué',
  cancelled: 'Annulé',
  skipped: 'Ignoré',
}

export function commandStatusLabel(status: string | undefined | null): string {
  if (!status) return ''
  return STATUS_LABELS[status] ?? status
}
