interface ModuleMeta {
  label: string
  cls: string
}

export const MODULE_META: Record<string, ModuleMeta> = {
  apt:       { label: 'APT',       cls: 'badge bg-azure-lt text-azure' },
  docker:    { label: 'Docker',    cls: 'badge bg-blue-lt text-blue' },
  compose:   { label: 'Compose',   cls: 'badge bg-cyan-lt text-cyan' },
  systemd:   { label: 'Systemd',   cls: 'badge bg-green-lt text-green' },
  journal:   { label: 'Journal',   cls: 'badge bg-purple-lt text-purple' },
  processes: { label: 'Processus', cls: 'badge bg-orange-lt text-orange' },
  custom:    { label: 'Custom',    cls: 'badge bg-teal-lt text-teal' },
  crowdsec:  { label: 'CrowdSec',  cls: 'badge bg-pink-lt text-pink' },
  restic:    { label: 'Restic',    cls: 'badge bg-lime-lt text-lime' },
  agent:     { label: 'Agent',     cls: 'badge bg-indigo-lt text-indigo' },
  proxmox:   { label: 'Proxmox',  cls: 'badge bg-yellow-lt text-yellow' },
}

export function moduleLabel(module: string): string {
  return MODULE_META[module]?.label ?? module
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

export const REMOTE_COMMAND_MODULE_OPTIONS: ModuleOption[] = REMOTE_COMMAND_MODULES.map((value) => ({
  value,
  label: moduleLabel(value),
}))
