// Some agent modules report a machine-readable JSON payload as their command
// output instead of a human log (module=processes action=list, for example
// — see agent/internal/dispatcher/handler_processes.go). CommandLogPanel.vue
// is generic across every module/action combination used in the app (10
// consumer views), so the "which module/action produces which shape" mapping
// lives here instead of growing as a pile of ad hoc computeds inside that
// component — adding a new structured kind means one branch here, not a new
// computed + a new v-if wired by hand into a component that shouldn't need
// to know the details of any one module.
import type { HostProcess } from '../composables/useHostProcesses'
import type { SystemdService } from '../components/host/SystemdTable.vue'

export type StructuredOutput =
  | { kind: 'processes'; data: HostProcess[] }
  | { kind: 'systemd'; data: SystemdService[] }

function parseJSONArray<T>(raw: string): T[] | null {
  try {
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return null
    return parsed as T[]
  } catch {
    return null
  }
}

// Returns null while the command is still streaming (output isn't valid
// JSON yet), on failure, or for any module/action with no known structured
// shape — the caller falls back to the raw <pre> view in all those cases.
export function resolveStructuredOutput(
  module: string | undefined,
  action: string | undefined,
  raw: string | undefined
): StructuredOutput | null {
  if (!raw) return null

  if (module === 'processes' && action === 'list') {
    const data = parseJSONArray<HostProcess>(raw)
    return data ? { kind: 'processes', data } : null
  }

  if (module === 'systemd' && action === 'list') {
    const data = parseJSONArray<SystemdService>(raw)
    return data ? { kind: 'systemd', data } : null
  }

  return null
}
