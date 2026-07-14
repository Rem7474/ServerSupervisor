export interface DispatchOption {
  value: string
  label: string
}

// The module set every dispatch-step backend validator shares (runbooks,
// alert rule command_trigger, scheduled tasks — see root CLAUDE.md). The
// *actions* allowed per module differ by caller (some validators enforce a
// strict per-module whitelist, others accept anything), so those aren't
// included here — see DispatchStepEditor.vue's actionsForModule/targetConfig
// props.
export const DISPATCH_MODULES: DispatchOption[] = [
  { value: 'docker', label: 'Docker' },
  { value: 'apt', label: 'APT' },
  { value: 'systemd', label: 'Service systemd' },
  { value: 'journal', label: 'Journal systemd' },
  { value: 'processes', label: 'Processus' },
  { value: 'custom', label: 'Tâche personnalisée' },
]
