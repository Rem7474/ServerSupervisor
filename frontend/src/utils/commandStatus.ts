// Shared translated labels for remote_commands / tracker-execution style
// statuses (pending/running/completed/failed/cancelled/skipped) — used
// anywhere a raw status string would otherwise leak into the UI untranslated.
import { i18n } from '../i18n'

const { t } = i18n.global

function statusLabels(): Record<string, string> {
  return {
    pending: t('common.statePending'),
    running: t('common.stateRunning'),
    completed: t('common.stateCompleted'),
    failed: t('common.stateFailed'),
    cancelled: t('common.stateCancelled'),
    skipped: t('common.stateSkipped'),
  }
}

export function commandStatusLabel(status: string | undefined | null): string {
  if (!status) return ''
  return statusLabels()[status] ?? status
}
