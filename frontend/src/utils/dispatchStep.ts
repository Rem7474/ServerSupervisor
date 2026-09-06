import { i18n } from '../i18n'

const { t } = i18n.global

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
//
// A function (not a static array) so callers re-resolve the labels on every
// call — a locale switch would otherwise leave these frozen in whichever
// language was active when this module first loaded, same reasoning as
// moduleMeta.ts's remoteCommandModuleOptions().
export function dispatchModules(): DispatchOption[] {
  return [
    { value: 'docker', label: t('common.moduleDocker') },
    { value: 'apt', label: t('common.moduleApt') },
    { value: 'systemd', label: t('common.dispatchModuleSystemdLabel') },
    { value: 'journal', label: t('common.dispatchModuleJournalLabel') },
    { value: 'processes', label: t('common.moduleProcesses') },
    { value: 'custom', label: t('common.dispatchModuleCustomLabel') },
  ]
}
