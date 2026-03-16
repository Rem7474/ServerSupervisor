export const MODULE_META = {
  apt:       { label: 'APT',       cls: 'badge bg-azure-lt text-azure' },
  docker:    { label: 'Docker',    cls: 'badge bg-blue-lt text-blue' },
  systemd:   { label: 'Systemd',   cls: 'badge bg-green-lt text-green' },
  journal:   { label: 'Journal',   cls: 'badge bg-purple-lt text-purple' },
  processes: { label: 'Processus', cls: 'badge bg-orange-lt text-orange' },
  custom:    { label: 'Custom',    cls: 'badge bg-teal-lt text-teal' },
  proxmox:   { label: 'Proxmox',  cls: 'badge bg-yellow-lt text-yellow' },
}

export function moduleLabel(module) {
  return MODULE_META[module]?.label ?? module
}

export function moduleClass(module) {
  return MODULE_META[module]?.cls ?? 'badge bg-secondary-lt text-secondary'
}
