import { i18n } from '../i18n'

const { t } = i18n.global

interface ModuleMeta {
  labelKey: string
  cls: string
}

// labelKey (not a plain label) + moduleLabel() re-resolving t() on every call
// (not a static Record built once) so the badge text updates immediately on
// a locale switch — same pattern as utils/statusClasses.ts.
export const MODULE_META: Record<string, ModuleMeta> = {
  apt:       { labelKey: 'common.moduleApt',       cls: 'badge bg-azure-lt text-azure' },
  docker:    { labelKey: 'common.moduleDocker',    cls: 'badge bg-blue-lt text-blue' },
  compose:   { labelKey: 'common.moduleCompose',   cls: 'badge bg-cyan-lt text-cyan' },
  systemd:   { labelKey: 'common.moduleSystemd',   cls: 'badge bg-green-lt text-green' },
  journal:   { labelKey: 'common.moduleJournal',   cls: 'badge bg-purple-lt text-purple' },
  processes: { labelKey: 'common.moduleProcesses', cls: 'badge bg-orange-lt text-orange' },
  custom:    { labelKey: 'common.moduleCustom',    cls: 'badge bg-teal-lt text-teal' },
  crowdsec:  { labelKey: 'common.moduleCrowdsec',  cls: 'badge bg-pink-lt text-pink' },
  restic:    { labelKey: 'common.moduleRestic',    cls: 'badge bg-lime-lt text-lime' },
  agent:     { labelKey: 'common.moduleAgent',     cls: 'badge bg-indigo-lt text-indigo' },
  proxmox:   { labelKey: 'common.moduleProxmox',   cls: 'badge bg-yellow-lt text-yellow' },
}

export function moduleLabel(module: string): string {
  const key = MODULE_META[module]?.labelKey
  return key ? t(key) : module
}

export function moduleClass(module: string): string {
  return MODULE_META[module]?.cls ?? 'badge bg-secondary-lt text-secondary'
}

export interface ModuleOption {
  value: string
  label: string
}

// The exact module set a remote command can target — agent/internal/dispatcher/
// registry.go's module → handler map. Kept as an explicit whitelist (not
// derived from all of MODULE_META's keys) because MODULE_META also carries
// entries used outside remote-command dispatch (e.g. `proxmox`, used by
// AccountView's personal audit trail) that would be meaningless as a
// remote_commands filter. This is the single source the audit module filter
// derives its options from, so adding a new agent module here is the one
// place to update — see the restic module filter going stale as the
// motivating example (it existed in the dispatcher for a while before this).
const REMOTE_COMMAND_MODULES = [
  'docker', 'compose', 'apt', 'journal', 'agent', 'systemd', 'processes', 'custom', 'crowdsec', 'restic',
] as const

// A function (not a static array) so callers re-resolve the labels on every
// call — a locale switch would otherwise leave these frozen in whichever
// language was active when this module first loaded.
export function remoteCommandModuleOptions(): ModuleOption[] {
  return REMOTE_COMMAND_MODULES.map((value) => ({
    value,
    label: moduleLabel(value),
  }))
}
