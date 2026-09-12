// Thin, stable-named wrapper around statusClasses.ts's getExecutionStateLabel
// for remote_commands / tracker-execution style statuses. Kept as its own
// export (rather than switching every call site to getExecutionStateLabel
// directly) because it predates statusClasses.ts and several call sites
// already import it by this name — but it must not carry its own copy of the
// state→text map: an earlier version did, with a narrower alias set
// (pending/running/completed/failed/cancelled/skipped only, missing
// ok/warnings/error/success/succeeded), which is exactly the kind of drift a
// second implementation of the same vocabulary invites.
import { getExecutionStateLabel } from './statusClasses'

export function commandStatusLabel(status: string | undefined | null): string {
  return getExecutionStateLabel(status)
}
