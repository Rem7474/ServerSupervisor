import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import AptHostCard from './AptHostCard.vue'
import type { Host } from '../../types/host'
import type { UUConfig } from '../../types/generated'

beforeEach(() => {
  setLocale('fr')
})

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

  it('shows the online/offline status text', () => {
    expect(mountCard({ host: { ...HOST, status: 'online' } as Host }).text()).toContain('En ligne')
    expect(mountCard({ host: { ...HOST, status: 'offline' } as Host }).text()).toContain('Hors ligne')
  })

  it('shows the no-data message with the embedded "apt update" hint when aptStatus is missing', () => {
    const wrapper = mountCard({ aptStatus: undefined })
    expect(wrapper.text()).toContain('Données APT non disponibles')
    expect(wrapper.text()).toContain('apt update')
    expect(wrapper.text()).toContain('pour initialiser.')
  })

  it('shows KPI counts and "Jamais" for missing update/upgrade dates', () => {
    const wrapper = mountCard({ aptStatus: { pending_packages: 5, security_updates: 2 } })
    expect(wrapper.text()).toContain('5')
    expect(wrapper.text()).toContain('en attente')
    expect(wrapper.text()).toContain('2')
    expect(wrapper.text()).toContain('sécurité')
    expect(wrapper.text()).toContain('Jamais')
  })

  it('shows the active-command badge with a translated status label', () => {
    const wrapper = mountCard({
      history: [{ id: 'cmd1', action: 'upgrade', status: 'running', created_at: new Date().toISOString() }],
    })
    expect(wrapper.text()).toContain('apt upgrade')
    expect(wrapper.text()).toContain('En cours')
  })

  it('shows the enriching badge and its tooltip after a command completes', () => {
    const wrapper = mountCard({ enriching: true })
    expect(wrapper.text()).toContain('Actualisation des données…')
    const badge = wrapper.findAll('.badge').find((b) => b.text().includes('Actualisation'))
    expect(badge?.attributes('title')).toContain("l'agent recalcule")
  })

  it('emits watch-command when clicking the live-logs button for an active command', async () => {
    const wrapper = mountCard({
      history: [{ id: 'cmd1', action: 'upgrade', status: 'running', created_at: new Date().toISOString() }],
    })
    await wrapper.find('[title="Voir les logs en direct"]').trigger('click')
    expect(wrapper.emitted('watch-command')).toBeTruthy()
  })

  it('shows the last two commands in history, with a link to the full history and a per-row logs button', async () => {
    const wrapper = mountCard({
      expanded: true,
      history: [
        { id: 'c1', action: 'update', status: 'completed', created_at: new Date().toISOString(), triggered_by: 'admin' },
        { id: 'c2', action: 'upgrade', status: 'failed', created_at: new Date().toISOString() },
      ],
    })
    expect(wrapper.text()).toContain('Dernières commandes')
    expect(wrapper.text()).toContain('Historique complet')
    expect(wrapper.text()).toContain('Terminé')
    expect(wrapper.text()).toContain('Échoué')
    expect(wrapper.text()).toContain('admin')

    await wrapper.findAll('[title="Voir les logs"]')[0].trigger('click')
    expect(wrapper.emitted('watch-command')?.[0][0]).toMatchObject({ id: 'c1' })
  })

  it('shows read-only mode and hides the run/schedule controls when canRunApt is false', () => {
    const wrapper = mountCard({ canRunApt: false })
    expect(wrapper.text()).toContain('Mode lecture seule')
    expect(wrapper.find('button').exists()).toBe(true) // expand toggle still renders
    expect(wrapper.text()).not.toContain('Planifier une commande APT')
  })

  it('emits run-cmd for update/upgrade/dist-upgrade and schedule for the calendar button', async () => {
    const wrapper = mountCard({ canRunApt: true })
    const buttons = wrapper.findAll('.btn-group.btn-group-sm button')
    await buttons[0].trigger('click')
    await buttons[1].trigger('click')
    await buttons[2].trigger('click')
    expect(wrapper.emitted('run-cmd')).toEqual([['update'], ['upgrade'], ['dist-upgrade']])

    await wrapper.find('[title="Planifier une commande APT"]').trigger('click')
    expect(wrapper.emitted('schedule')).toBeTruthy()
  })

  it('toggles expanded state and its tooltip', async () => {
    const wrapper = mountCard({ expanded: false })
    expect(wrapper.find('[title="Développer"]').exists()).toBe(true)
    await wrapper.find('[title="Développer"]').trigger('click')
    expect(wrapper.emitted('update:expanded')?.[0]).toEqual([true])
  })
})
