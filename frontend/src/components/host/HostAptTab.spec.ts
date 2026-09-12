import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import HostAptTab from './HostAptTab.vue'

beforeEach(() => {
  setLocale('fr')
})

// Regression test: the host-detail APT tab used to only show the pending
// package *count*, unlike the /apt page's AptHostCard which lists the actual
// packages — the two surfaces read the same apt_status row, so there was no
// reason for one to hide data the other shows. Both now share
// AptPendingPackagesList.vue.
describe('HostAptTab', () => {
  it('shows read-only mode instead of action buttons when canRunApt is false', () => {
    const wrapper = mount(HostAptTab, {
      props: { aptStatus: { pending_packages: 0, security_updates: 0 }, canRunApt: false },
    })
    expect(wrapper.text()).toContain('Mode lecture seule')
    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('disables the action buttons while a command is loading', () => {
    const wrapper = mount(HostAptTab, {
      props: { aptStatus: { pending_packages: 1, security_updates: 0 }, canRunApt: true, aptCmdLoading: 'upgrade' },
    })
    const buttons = wrapper.findAll('.btn-group button')
    expect(buttons.map((b) => b.text().trim())).toEqual(['apt update', 'apt upgrade', 'apt dist-upgrade'])
    expect(buttons.every((b) => b.attributes('disabled') !== undefined)).toBe(true)
  })

  it('emits run-apt-command with the clicked action', async () => {
    const wrapper = mount(HostAptTab, {
      props: { aptStatus: { pending_packages: 1, security_updates: 0 }, canRunApt: true },
    })
    await wrapper.findAll('.btn-group button')[1].trigger('click')
    expect(wrapper.emitted('run-apt-command')?.[0]).toEqual(['upgrade'])
  })

  it('shows the no-data card with an apt update button when there is no apt status', () => {
    const wrapper = mount(HostAptTab, { props: { aptStatus: null, canRunApt: true } })
    expect(wrapper.text()).toContain('Aucune donnée APT disponible')
    expect(wrapper.text()).toContain('apt update')
  })

  describe('unattended-upgrades card', () => {
    it('shows the not-installed message and install button, emitting uu-install', async () => {
      const wrapper = mount(HostAptTab, {
        props: {
          aptStatus: { pending_packages: 0, security_updates: 0 },
          canRunApt: true,
          uuStatus: { installed: false },
        },
      })
      expect(wrapper.text()).toContain("unattended-upgrades n'est pas installé")
      expect(wrapper.text()).toContain('Non installé')

      const installButton = wrapper.findAll('button').find((b) => b.text() === 'Installer')
      await installButton?.trigger('click')
      expect(wrapper.emitted('uu-install')).toBeTruthy()
    })

    it('shows enabled/reboot-required badges and the last-run summary', () => {
      const wrapper = mount(HostAptTab, {
        props: {
          aptStatus: { pending_packages: 0, security_updates: 0 },
          uuStatus: { installed: true, enabled: true, reboot_required: true, last_run_at: new Date().toISOString(), last_run_packages: 3 },
        },
      })
      expect(wrapper.text()).toContain('Activé')
      expect(wrapper.text()).toContain('Redémarrage requis')
      expect(wrapper.text()).toContain('3 paquet(s) installé(s)')
    })

    it('shows the disabled badge and no-run-recorded fallback', () => {
      const wrapper = mount(HostAptTab, {
        props: {
          aptStatus: { pending_packages: 0, security_updates: 0 },
          uuStatus: { installed: true, enabled: false },
        },
      })
      expect(wrapper.text()).toContain('Désactivé')
      expect(wrapper.text()).toContain('Aucune exécution enregistrée.')
    })

    it('submits the config form and emits uu-configure / uu-run-now', async () => {
      const wrapper = mount(HostAptTab, {
        props: {
          aptStatus: { pending_packages: 0, security_updates: 0 },
          canRunApt: true,
          uuStatus: { installed: true, enabled: true },
          uuForm: { enabled: true, config: { security_only: false, remove_unused: false, auto_reboot: true, auto_reboot_time: '03:00' } },
        },
      })
      expect(wrapper.find('input[type="time"]').exists()).toBe(true)

      const saveButton = wrapper.findAll('button').find((b) => b.text().includes('Enregistrer'))
      await saveButton?.trigger('click')
      expect(wrapper.emitted('uu-configure')?.[0]?.[0]).toMatchObject({ enabled: true })

      const runNowButton = wrapper.findAll('button').find((b) => b.text().includes('Lancer maintenant'))
      await runNowButton?.trigger('click')
      expect(wrapper.emitted('uu-run-now')).toBeTruthy()
    })

    it('renders the run history with truncated packages and an error badge, and emits uu-log', async () => {
      const wrapper = mount(HostAptTab, {
        props: {
          aptStatus: { pending_packages: 0, security_updates: 0 },
          uuStatus: { installed: true, enabled: true },
          uuRuns: [
            { run_at: new Date().toISOString(), packages: ['a', 'b', 'c', 'd'], had_error: true, log_snippet: 'boom' },
            { run_at: new Date().toISOString(), packages: [], had_error: false },
          ],
        },
      })
      expect(wrapper.text()).toContain('a, b, c')
      expect(wrapper.text()).toContain('(+1)')
      expect(wrapper.text()).toContain('Aucun')
      expect(wrapper.text()).toContain('Erreur')

      const logButton = wrapper.findAll('button[title="Voir les logs"]')[0]
      expect(logButton.attributes('disabled')).toBeUndefined()
      await logButton.trigger('click')
      expect(wrapper.emitted('uu-log')?.[0]?.[0]).toMatchObject({ had_error: true })
    })

    it('shows the no-automatic-upgrades fallback when the run list is empty', () => {
      const wrapper = mount(HostAptTab, {
        props: {
          aptStatus: { pending_packages: 0, security_updates: 0 },
          uuStatus: { installed: true, enabled: true },
          uuRuns: [],
        },
      })
      expect(wrapper.text()).toContain('Aucun upgrade automatique enregistré.')
    })

    it('shows the waiting-for-agent-data state when uuStatus is not yet known', () => {
      const wrapper = mount(HostAptTab, {
        props: { aptStatus: { pending_packages: 0, security_updates: 0 }, canRunApt: true, uuStatus: null },
      })
      expect(wrapper.text()).toContain("En attente des données de l'agent")
    })
  })
})

describe('HostAptTab — pending packages list parity with the /apt page', () => {
  it('lists the pending packages by name, not just the count', () => {
    const wrapper = mount(HostAptTab, {
      props: {
        aptStatus: {
          pending_packages: 2,
          security_updates: 0,
          package_list: JSON.stringify(['curl', 'openssl']),
        },
      },
    })
    expect(wrapper.text()).toContain('Paquets en attente')
    expect(wrapper.text()).toContain('curl')
    expect(wrapper.text()).toContain('openssl')
  })

  it('renders no package grid when package_list is absent (only the KPI count tile)', () => {
    const wrapper = mount(HostAptTab, {
      props: { aptStatus: { pending_packages: 0, security_updates: 0 } },
    })
    expect(wrapper.find('.apt-package-item').exists()).toBe(false)
  })
})
