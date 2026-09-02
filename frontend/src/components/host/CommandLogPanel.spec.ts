import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import CommandLogPanel from './CommandLogPanel.vue'

beforeEach(() => {
  setLocale('fr')
})

const baseCommand = {
  host_name: 'web-01',
  module: 'processes',
  action: 'list',
  status: 'completed',
  created_at: new Date().toISOString(),
}

describe('CommandLogPanel', () => {
  it('renders the processes table for a completed module=processes/action=list command', () => {
    const wrapper = mount(CommandLogPanel, {
      props: {
        show: true,
        command: {
          ...baseCommand,
          output: JSON.stringify([
            { pid: 1, name: 'systemd', user: 'root', cpu_pct: 0.1, mem_pct: 0.2, mem_rss_kb: 1024, state: 'S' },
          ]),
        },
      },
    })
    expect(wrapper.find('table').exists()).toBe(true)
    expect(wrapper.text()).toContain('systemd')
    expect(wrapper.find('pre.console-output').exists()).toBe(false)
  })

  it('renders the systemd table (read-only, no action buttons) for a completed module=systemd/action=list command', () => {
    const wrapper = mount(CommandLogPanel, {
      props: {
        show: true,
        command: {
          ...baseCommand,
          module: 'systemd',
          action: 'list',
          output: JSON.stringify([
            { name: 'nginx.service', active_state: 'active', sub_state: 'running', description: 'web server' },
          ]),
        },
      },
    })
    expect(wrapper.find('table').exists()).toBe(true)
    expect(wrapper.text()).toContain('nginx.service')
    expect(wrapper.text()).not.toContain('Actions')
    expect(wrapper.find('pre.console-output').exists()).toBe(false)
  })

  it('renders a summary card for a completed module=restic/action=run_backup command', () => {
    const wrapper = mount(CommandLogPanel, {
      props: {
        show: true,
        command: {
          ...baseCommand,
          module: 'restic',
          action: 'run_backup',
          output: JSON.stringify({ status: 'ok', profile: 'files', duration_sec: 42, snapshot_id: 'abc123' }),
        },
      },
    })
    expect(wrapper.text()).toContain('abc123')
    const badges = wrapper.findAll('.badge')
    expect(badges[badges.length - 1].text()).toBe('ok')
    expect(wrapper.find('table').exists()).toBe(false)
    expect(wrapper.find('pre.console-output').exists()).toBe(false)
  })

  it('falls back to raw output for a non-processes module', () => {
    const wrapper = mount(CommandLogPanel, {
      props: {
        show: true,
        command: { ...baseCommand, module: 'apt', action: 'upgrade', output: 'Reading package lists...' },
      },
    })
    expect(wrapper.find('table').exists()).toBe(false)
    expect(wrapper.find('pre.console-output').text()).toContain('Reading package lists')
  })

  it('falls back to raw output while a processes command is still streaming (not valid JSON yet)', () => {
    const wrapper = mount(CommandLogPanel, {
      props: {
        show: true,
        command: { ...baseCommand, status: 'running', output: '[{"pid":1,"name":"sys' },
      },
    })
    expect(wrapper.find('table').exists()).toBe(false)
    expect(wrapper.find('pre.console-output').exists()).toBe(true)
  })

  // mode="custom" is what ProxmoxConsole.vue reuses to render a live xterm.js
  // terminal inside the same card/header/FAB shell every other console panel
  // in the app uses — the log-viewer body and its copy/download/clear
  // buttons (meaningless for a live PTY) are irrelevant in this mode.
  describe('mode="custom"', () => {
    it('renders the default slot instead of the log viewer, and hides copy/download/clear', () => {
      const wrapper = mount(CommandLogPanel, {
        props: { show: true, mode: 'custom', title: 'Console' },
        slots: { default: '<div class="my-terminal">terminal here</div>' },
      })
      expect(wrapper.find('.my-terminal').exists()).toBe(true)
      expect(wrapper.find('pre.console-output').exists()).toBe(false)
      expect(wrapper.text()).not.toContain('Aucun log sélectionné')
      expect(wrapper.find('button[title="Copier la sortie"]').exists()).toBe(false)
      expect(wrapper.find('button[title="Télécharger (.txt)"]').exists()).toBe(false)
    })

    it('still renders the close button and title-suffix/header-actions slots', async () => {
      const wrapper = mount(CommandLogPanel, {
        props: { show: true, mode: 'custom', title: 'Console' },
        slots: {
          default: '<div />',
          'title-suffix': '<span class="my-badge">connecté</span>',
          'header-actions': '<button class="my-action">Rouvrir</button>',
        },
      })
      expect(wrapper.find('.my-badge').exists()).toBe(true)
      expect(wrapper.find('.my-action').exists()).toBe(true)

      await wrapper.find('button[title="Fermer"]').trigger('click')
      expect(wrapper.emitted('close')).toBeTruthy()
    })

    it('shows the FAB reopen button when hidden (v-show, not unmounted), same as mode="log"', () => {
      const wrapper = mount(CommandLogPanel, {
        props: { show: false, mode: 'custom', title: 'Console' },
        slots: { default: '<div class="my-terminal" />' },
      })
      expect(wrapper.find('.console-fab').isVisible()).toBe(true)
      // The slotted content stays mounted (v-show), not torn down — this is
      // the whole point of ProxmoxConsole reusing this shell instead of a
      // v-if modal: hiding must not lose the live session.
      expect(wrapper.find('.my-terminal').exists()).toBe(true)
    })
  })
})
