import { DISPATCH_MODULES } from './dispatchStep'
import type { DispatchOption } from './dispatchStep'

// Scheduled tasks are the only DispatchStepEditor caller whose backend scope
// exceeds DISPATCH_MODULES (validModules in scheduledtask.Service) — restic
// isn't allowed for runbook steps or alert-rule command triggers, so it's
// added only here rather than to the shared list (see DispatchStepEditor's
// `modules` prop doc). Shared by both scheduled-task editors (the global
// "Tâches planifiées" view and the per-host tab) so they stay in sync.
export const SCHEDULED_TASK_MODULES: DispatchOption[] = [
  ...DISPATCH_MODULES,
  { value: 'restic', label: 'Restic (backup)' },
]

// Advisory only — the scheduled-task backend validates the module but not
// the action string (see root CLAUDE.md), so these lists just drive the
// picker UX. A module with no entry here falls back to a free-text action
// input (see DispatchStepEditor's actionsForModule prop doc).
const MODULE_ACTIONS: Record<string, string[]> = {
  apt: ['update', 'upgrade', 'install', 'remove'],
  docker: ['start', 'stop', 'restart', 'pull', 'prune'],
  systemd: ['restart', 'start', 'stop', 'enable', 'disable'],
  journal: ['tail'],
  processes: ['list'],
  restic: ['run_backup'],
}

const TARGET_LABELS: Record<string, string> = {
  docker: 'Conteneur (nom ou ID)',
  systemd: 'Service systemd',
  custom: 'ID de tâche custom',
  apt: 'Paquet (optionnel pour install/remove)',
  restic: 'Profil resticprofile (optionnel)',
}

const TARGET_PLACEHOLDERS: Record<string, string> = {
  docker: 'nginx',
  systemd: 'nginx.service',
  custom: 'my-deploy-task',
  apt: 'nginx',
  restic: 'files',
}

// Only docker/apt/restic are gated by an agent collector flag — systemd,
// journal, processes and custom have no capability toggle (the agent always
// supports them, mirroring its own "always on" Capabilities.Systemd/Journal
// — see agent/internal/reporter/reporter.go), so they're never filtered out.
const MODULE_CAPABILITY_KEY: Partial<Record<string, string>> = {
  docker: 'docker',
  apt: 'apt',
  restic: 'restic',
}

// Filters the module picker down to what the selected host's agent actually
// reports as active (the `collectors` flags from its periodic report) — a
// host with collect_docker/collect_apt/collect_restic off shouldn't offer
// that module, since dispatching it would just fail on the agent.
export function availableScheduledTaskModules(collectors: Record<string, boolean> | null | undefined): DispatchOption[] {
  return SCHEDULED_TASK_MODULES.filter((m) => {
    const key = MODULE_CAPABILITY_KEY[m.value]
    return !key || !!collectors?.[key]
  })
}

export function scheduledTaskActions(module: string): DispatchOption[] {
  return (MODULE_ACTIONS[module] || []).map((a) => ({ value: a, label: a }))
}

export function scheduledTaskTargetConfig(module: string): { label: string; placeholder?: string } | null {
  const label = TARGET_LABELS[module]
  return label ? { label, placeholder: TARGET_PLACEHOLDERS[module] } : null
}
