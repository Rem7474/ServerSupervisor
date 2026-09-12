import { dispatchModules } from './dispatchStep'
import type { DispatchOption } from './dispatchStep'
import { i18n } from '../i18n'

const { t } = i18n.global

// Scheduled tasks are the only DispatchStepEditor caller whose backend scope
// exceeds dispatchModules() (validModules in scheduledtask.Service) — restic
// isn't allowed for runbook steps or alert-rule command triggers, so it's
// added only here rather than to the shared list (see DispatchStepEditor's
// `modules` prop doc). Shared by both scheduled-task editors (the global
// "Tâches planifiées" view and the per-host tab) so they stay in sync.
//
// A function (not a static array) for the same locale-reactivity reason as
// dispatchModules() itself.
export function scheduledTaskModules(): DispatchOption[] {
  return [
    ...dispatchModules(),
    { value: 'restic', label: t('common.dispatchModuleResticLabel') },
  ]
}

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

function targetLabels(): Record<string, string> {
  return {
    docker: t('common.dispatchContainerTargetLabel'),
    systemd: t('common.dispatchModuleSystemdLabel'),
    custom: t('common.dispatchCustomTaskIdTargetLabel'),
    apt: t('common.dispatchAptTargetLabel'),
    restic: t('common.dispatchResticTargetLabel'),
  }
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
  return scheduledTaskModules().filter((m) => {
    const key = MODULE_CAPABILITY_KEY[m.value]
    return !key || !!collectors?.[key]
  })
}

export function scheduledTaskActions(module: string): DispatchOption[] {
  return (MODULE_ACTIONS[module] || []).map((a) => ({ value: a, label: a }))
}

export function scheduledTaskTargetConfig(module: string): { label: string; placeholder?: string } | null {
  const label = targetLabels()[module]
  return label ? { label, placeholder: TARGET_PLACEHOLDERS[module] } : null
}
