import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import AptHostCard from './AptHostCard.vue'
import type { Host } from '../../types/host'
import type { UUConfig } from '../../types/generated'

const HOST: Host = { id: 'h1', name: 'web-01', hostname: 'web-01', status: 'online' } as Host
const UU_CONFIG: UUConfig = { security_only: true, auto_reboot: false, auto_reboot_time: '02:00', remove_unused: false }

function mountCard(props: Partial<InstanceType<typeof AptHostCard>['$props']> = {}) {
  return mount(AptHostCard, {
    props: {
      host: HOST,
      aptStatus: { pending_packages: 2, security_updates: 0 },
      history: [],
      expanded: false,
      selected: false,
      canRunApt: true,
      cmdLoading: null,
      ...props,
    },
  })
}

describe('AptHostCard', () => {
  it('shows a reboot-required badge when unattended-upgrades reports one', () => {
    const wrapper = mountCard({ uuStatus: { installed: true, enabled: true, reboot_required: true, last_run_packages: 3, config: UU_CONFIG } })
    expect(wrapper.text()).toContain('Redémarrage requis')
  })

  it('does not show the reboot-required badge when it is false', () => {
    const wrapper = mountCard({ uuStatus: { installed: true, enabled: true, reboot_required: false, last_run_packages: 3, config: UU_CONFIG } })
    expect(wrapper.text()).not.toContain('Redémarrage requis')
  })

  it('shows an outdated-agent badge when flagged, and not otherwise', () => {
    const outdated = mountCard({ agentOutdated: true, latestAgentVersion: '2.0.0' })
    expect(outdated.text()).toContain('Agent obsolète')

    const upToDate = mountCard({ agentOutdated: false })
    expect(upToDate.text()).not.toContain('Agent obsolète')
  })

  it('shows the unattended-upgrades on/off badge in the always-visible header, even collapsed', () => {
    const enabled = mountCard({
      expanded: false,
      uuStatus: { installed: true, enabled: true, reboot_required: false, last_run_packages: 3, config: UU_CONFIG },
    })
    expect(enabled.text()).toContain('MAJ auto activées')

    const disabled = mountCard({
      expanded: false,
      uuStatus: { installed: true, enabled: false, reboot_required: false, last_run_packages: 3, config: UU_CONFIG },
    })
    expect(disabled.text()).toContain('MAJ auto désactivées')
  })

  it('does not show the on/off badge when unattended-upgrades is not installed', () => {
    const wrapper = mountCard({
      expanded: false,
      uuStatus: { installed: false, enabled: false, reboot_required: false, last_run_packages: 0, config: UU_CONFIG },
    })
    expect(wrapper.text()).not.toContain('MAJ auto')
  })

  it('shows a condensed unattended-upgrades summary when expanded', () => {
    const wrapper = mountCard({
      expanded: true,
      uuStatus: { installed: true, enabled: true, reboot_required: false, last_run_packages: 3, config: UU_CONFIG },
    })
    expect(wrapper.text()).toContain('Mises à jour automatiques')
    expect(wrapper.text()).toContain('Activé')
  })

  it('shows "Non installé" when unattended-upgrades is not installed', () => {
    const wrapper = mountCard({
      expanded: true,
      uuStatus: { installed: false, enabled: false, reboot_required: false, last_run_packages: 0, config: UU_CONFIG },
    })
    expect(wrapper.text()).toContain('Non installé')
  })
})
